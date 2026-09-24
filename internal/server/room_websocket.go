package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/MeTerminator/ClipBox/internal/share"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

type webSocketCommand struct {
	Type    string             `json:"type"`
	Token   string             `json:"token"`
	Message share.MessageInput `json:"message"`
}

func (s *Server) roomWebSocket(c *gin.Context) {
	if s.sharing == nil || s.hub == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	roomID := c.Param("roomID")
	websocket.Handler(func(connection *websocket.Conn) {
		s.serveRoomWebSocket(connection, roomID)
	}).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) serveRoomWebSocket(connection *websocket.Conn, publicID string) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	var auth webSocketCommand
	if err := websocket.JSON.Receive(connection, &auth); err != nil || auth.Type != "auth" {
		_ = websocket.JSON.Send(connection, gin.H{"type": "error", "code": "authentication_required"})
		return
	}
	ctx := context.Background()
	room, member, err := s.sharing.Authenticate(ctx, publicID, auth.Token)
	if err != nil {
		_ = websocket.JSON.Send(connection, gin.H{"type": "error", "code": "unauthorized"})
		return
	}
	messages, err := s.rooms.ListRoomMessages(ctx, room.ID, 0, 200)
	if err != nil {
		_ = websocket.JSON.Send(connection, gin.H{"type": "error", "code": "history_unavailable"})
		return
	}
	client := &share.Client{RoomID: room.ID, MemberID: member.ID, Send: make(chan []byte, 32)}
	s.hub.Register(client)
	defer func() {
		s.hub.Unregister(client)
		close(client.Send)
		if s.hub.OnlineCount(room.ID) == 0 {
			_ = s.rooms.DeleteRoom(ctx, room.ID)
			return
		}
		if member.IsOwner {
			if _, err := s.rooms.TransferRoomOwnership(ctx, room.ID, member.ID); err != nil {
				slog.Error("transfer room ownership", "room_id", room.PublicID, "error", err)
			}
		}
		s.broadcastPresence(ctx, room.PublicID, room.ID)
	}()

	initial := make([]gin.H, 0, len(messages))
	for i := range messages {
		initial = append(initial, roomMessageResponse(room.PublicID, &messages[i]))
	}
	if err := websocket.JSON.Send(connection, gin.H{"type": "ready", "room": s.roomResponse(room, member.ID, s.now()), "messages": initial}); err != nil {
		return
	}
	_ = connection.SetDeadline(time.Time{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for payload := range client.Send {
			if err := websocket.Message.Send(connection, string(payload)); err != nil {
				return
			}
		}
	}()
	s.broadcastPresence(ctx, room.PublicID, room.ID)

	for {
		var command webSocketCommand
		if err := websocket.JSON.Receive(connection, &command); err != nil {
			return
		}
		switch command.Type {
		case "send":
			message, sendErr := s.sharing.SendMessage(ctx, room, member, command.Message)
			if sendErr != nil {
				code := "send_failed"
				if errors.Is(sendErr, share.ErrInvalidMessage) {
					code = "invalid_message"
				}
				s.enqueueEvent(client, gin.H{"type": "error", "code": code})
				continue
			}
			s.hub.Broadcast(room.ID, gin.H{"type": "message", "message": roomMessageResponse(room.PublicID, message)})
		case "ping":
			_ = s.rooms.TouchRoomMember(ctx, member.ID, s.now())
			s.enqueueEvent(client, gin.H{"type": "pong"})
		default:
			s.enqueueEvent(client, gin.H{"type": "error", "code": "unknown_command"})
		}
		select {
		case <-done:
			return
		default:
		}
	}
}

func (s *Server) enqueueEvent(client *share.Client, event any) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	select {
	case client.Send <- payload:
	default:
	}
}

func (s *Server) broadcastPresence(ctx context.Context, publicID string, roomID int64) {
	room, err := s.rooms.FindRoom(ctx, publicID)
	if err != nil {
		return
	}
	response := s.roomResponse(room, 0, s.now())
	s.hub.Broadcast(roomID, gin.H{"type": "presence", "members": response["members"]})
}
