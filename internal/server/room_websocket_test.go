package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MeTerminator/ClipBox/internal/config"
	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/MeTerminator/ClipBox/internal/store"
	"github.com/MeTerminator/ClipBox/internal/upload"
	"golang.org/x/net/websocket"
)

type roomSessionPayload struct {
	Room struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		CurrentMemberID int64  `json:"current_member_id"`
		Members         []struct {
			Nickname string `json:"nickname"`
		} `json:"members"`
	} `json:"room"`
	Token string `json:"token"`
}

func TestRoomWebSocketBroadcastsAndPersistsMessages(t *testing.T) {
	dataDir := t.TempDir()
	database, err := store.Open(context.Background(), "sqlite", filepath.Join(dataDir, "clipbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	uploads, err := upload.NewManager(dataDir, 1024, 1024*1024, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: dataDir, WWWRoot: filepath.Join(dataDir, "missing"), RealIPHeader: "X-Real-IP", MaxTextSize: 1024, MaxLinkLength: 2048, MaxUploadFileSize: 1024 * 1024, UploadChunkSize: 1024, UploadSessionTTL: time.Minute, UploadWorkers: 1}
	application := New(cfg, database, uploads)
	httpServer := httptest.NewServer(application.Handler())
	defer httpServer.Close()

	owner := postRoomJSON(t, httpServer.URL+"/api/rooms", `{"name":"","nickname":"","device":"Desktop","os":"macOS","browser":"Safari"}`)
	if !regexp.MustCompile(`^\d{8}$`).MatchString(owner.Room.ID) {
		t.Fatalf("room ID = %q, want eight digits", owner.Room.ID)
	}
	if owner.Room.Name != "" {
		t.Fatalf("empty room name became %q", owner.Room.Name)
	}
	if len(owner.Room.Members) != 1 || !regexp.MustCompile(`^\d{4}$`).MatchString(owner.Room.Members[0].Nickname) {
		t.Fatalf("generated nickname = %#v, want four digits", owner.Room.Members)
	}
	member := postRoomJSON(t, httpServer.URL+"/api/rooms/"+owner.Room.ID+"/join", `{"nickname":"Phone","device":"Phone","os":"iOS","browser":"Safari"}`)
	renameRequest, err := http.NewRequest(http.MethodPatch, httpServer.URL+"/api/rooms/"+owner.Room.ID, bytes.NewBufferString(`{"name":"Renamed"}`))
	if err != nil {
		t.Fatal(err)
	}
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.Header.Set("Authorization", "Bearer "+owner.Token)
	renameResponse, err := http.DefaultClient.Do(renameRequest)
	if err != nil {
		t.Fatal(err)
	}
	renameResponse.Body.Close()
	if renameResponse.StatusCode != http.StatusOK {
		t.Fatalf("rename status = %d", renameResponse.StatusCode)
	}

	ownerWS := dialRoomWebSocket(t, httpServer.URL, owner.Room.ID, owner.Token)
	defer ownerWS.Close()
	phoneWS := dialRoomWebSocket(t, httpServer.URL, owner.Room.ID, member.Token)
	defer phoneWS.Close()

	command := map[string]any{"type": "send", "message": map[string]any{"kind": "text", "source": "clipboard", "text": "shared text"}}
	if err := websocket.JSON.Send(ownerWS, command); err != nil {
		t.Fatal(err)
	}
	var received map[string]any
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = phoneWS.SetReadDeadline(deadline)
		if err := websocket.JSON.Receive(phoneWS, &received); err != nil {
			t.Fatal(err)
		}
		if received["type"] == "message" {
			break
		}
	}
	message, ok := received["message"].(map[string]any)
	if !ok || message["text"] != "shared text" || message["source"] != model.MessageSourceClipboard {
		t.Fatalf("WebSocket message = %#v", received)
	}
	room, err := database.FindRoom(context.Background(), owner.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if room.Name != "Renamed" {
		t.Fatalf("persisted room name = %q", room.Name)
	}
	persisted, err := database.ListRoomMessages(context.Background(), room.ID, 0, 10)
	if err != nil || len(persisted) != 1 || persisted[0].Text != "shared text" {
		t.Fatalf("persisted messages = %#v, %v", persisted, err)
	}
	_ = phoneWS.Close()
	_ = ownerWS.Close()
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, err = database.FindRoom(context.Background(), owner.Room.ID)
		if errors.Is(err, store.ErrNotFound) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("room %s still exists after its last client disconnected", owner.Room.ID)
}

func postRoomJSON(t *testing.T, url, payload string) roomSessionPayload {
	t.Helper()
	response, err := http.Post(url, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("POST %s status = %d", url, response.StatusCode)
	}
	var result roomSessionPayload
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func dialRoomWebSocket(t *testing.T, baseURL, roomID, token string) *websocket.Conn {
	t.Helper()
	connection, err := websocket.Dial("ws"+strings.TrimPrefix(baseURL, "http")+"/api/rooms/"+roomID+"/ws", "", baseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := websocket.JSON.Send(connection, map[string]string{"type": "auth", "token": token}); err != nil {
		t.Fatal(err)
	}
	var ready map[string]any
	if err := websocket.JSON.Receive(connection, &ready); err != nil {
		t.Fatal(err)
	}
	if ready["type"] != "ready" {
		t.Fatalf("first WebSocket event = %#v", ready)
	}
	return connection
}
