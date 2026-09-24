package server

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/MeTerminator/ClipBox/internal/share"
	"github.com/MeTerminator/ClipBox/internal/store"
	"github.com/gin-gonic/gin"
)

type roomIdentityRequest struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
	Device   string `json:"device"`
	OS       string `json:"os"`
	Browser  string `json:"browser"`
}

type createRoomRequest struct {
	Name string `json:"name"`
	roomIdentityRequest
}

type renameRoomRequest struct {
	Name string `json:"name"`
}

type updateNicknameRequest struct {
	Nickname string `json:"nickname"`
}

type roomPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) createRoom(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	var request createRoomRequest
	if c.ShouldBindJSON(&request) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid room data")
		return
	}
	request.Name = cleanRoomText(request.Name, 80)
	token, member, err := newRoomMember(request.roomIdentityRequest, c.GetHeader("User-Agent"), s.now())
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Could not create identity")
		return
	}
	for attempt := 0; attempt < 20; attempt++ {
		publicID, randomErr := randomRoomID()
		if randomErr != nil {
			err = randomErr
			break
		}
		room := &model.ShareRoom{PublicID: publicID, Name: request.Name}
		err = s.rooms.CreateRoom(c.Request.Context(), room, member)
		if errors.Is(err, store.ErrConflict) {
			continue
		}
		if err == nil {
			room.Members = append(room.Members, *member)
			c.JSON(http.StatusCreated, s.roomSessionResponse(room, member, token))
			return
		}
		break
	}
	jsonError(c, http.StatusInternalServerError, "Could not create room")
}

func (s *Server) joinRoom(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	var request roomIdentityRequest
	if c.ShouldBindJSON(&request) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid member data")
		return
	}
	token, member, err := newRoomMember(request, c.GetHeader("User-Agent"), s.now())
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Could not create identity")
		return
	}
	room, err := s.sharing.JoinRoom(c.Request.Context(), normalizedRoomID(c.Param("roomID")), request.Password, member)
	if err != nil {
		if errors.Is(err, share.ErrUnauthorized) {
			jsonError(c, http.StatusUnauthorized, "Incorrect room password")
		} else {
			jsonError(c, http.StatusNotFound, "Room not found")
		}
		return
	}
	room.Members = append(room.Members, *member)
	c.JSON(http.StatusCreated, s.roomSessionResponse(room, member, token))
}

func (s *Server) authenticatedRoom(c *gin.Context) (*model.ShareRoom, *model.RoomMember, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	room, member, err := s.sharing.Authenticate(c.Request.Context(), c.Param("roomID"), token)
	if err != nil {
		if errors.Is(err, share.ErrUnauthorized) {
			jsonError(c, http.StatusUnauthorized, "Join the room first")
		} else {
			jsonError(c, http.StatusNotFound, "Room not found")
		}
		return nil, nil, false
	}
	return room, member, true
}

func (s *Server) getRoom(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, member, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, s.roomResponse(room, member.ID, s.now()))
}

func (s *Server) renameRoom(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, _, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	var request renameRoomRequest
	if c.ShouldBindJSON(&request) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid room data")
		return
	}
	name := cleanRoomText(request.Name, 80)
	if err := s.sharing.RenameRoom(c.Request.Context(), room, name); err != nil {
		jsonError(c, http.StatusInternalServerError, "Could not rename room")
		return
	}
	room.Name = name
	s.hub.Broadcast(room.ID, gin.H{"type": "room_updated", "name": name})
	c.JSON(http.StatusOK, gin.H{"name": name})
}

func (s *Server) updateRoomNickname(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, member, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	var request updateNicknameRequest
	if c.ShouldBindJSON(&request) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid member data")
		return
	}
	nickname := cleanRoomText(request.Nickname, 40)
	if err := s.sharing.UpdateNickname(c.Request.Context(), member, nickname); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, share.ErrInvalidRoom) {
			status = http.StatusBadRequest
		}
		jsonError(c, status, "Could not update nickname")
		return
	}
	member.Nickname = nickname
	for i := range room.Members {
		if room.Members[i].ID == member.ID {
			room.Members[i].Nickname = nickname
		}
	}
	s.hub.Broadcast(room.ID, gin.H{"type": "member_updated", "member": memberResponse(*member, true)})
	c.JSON(http.StatusOK, memberResponse(*member, true))
}

func (s *Server) updateRoomPassword(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, _, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	var request roomPasswordRequest
	if c.ShouldBindJSON(&request) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid room password")
		return
	}
	if err := s.sharing.SetRoomPassword(c.Request.Context(), room, request.Password); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, share.ErrInvalidRoom) {
			status = http.StatusBadRequest
		}
		jsonError(c, status, "Room password must be less than 64 characters")
		return
	}
	hasPassword := request.Password != ""
	room.PasswordHash = share.TokenDigest(request.Password)
	s.hub.Broadcast(room.ID, gin.H{"type": "room_updated", "name": room.Name, "has_password": hasPassword})
	c.JSON(http.StatusOK, gin.H{"has_password": hasPassword})
}

func (s *Server) deleteRoom(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, member, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	if !member.IsOwner {
		jsonError(c, http.StatusForbidden, "Only the room owner can delete it")
		return
	}
	if err := s.rooms.DeleteRoom(c.Request.Context(), room.ID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Could not delete room")
		return
	}
	s.hub.Broadcast(room.ID, gin.H{"type": "room_deleted"})
	c.Status(http.StatusNoContent)
}

func (s *Server) listRoomMessages(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, _, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	after, _ := strconv.ParseInt(c.Query("after"), 10, 64)
	messages, err := s.rooms.ListRoomMessages(c.Request.Context(), room.ID, max(after, 0), 200)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Could not load messages")
		return
	}
	result := make([]gin.H, 0, len(messages))
	for i := range messages {
		result = append(result, roomMessageResponse(room.PublicID, &messages[i]))
	}
	c.JSON(http.StatusOK, gin.H{"messages": result})
}

func (s *Server) downloadRoomFile(c *gin.Context) {
	if s.rooms == nil {
		jsonError(c, http.StatusNotImplemented, "Room sharing is unavailable")
		return
	}
	room, _, ok := s.authenticatedRoom(c)
	if !ok {
		return
	}
	messageID, err := strconv.ParseInt(c.Param("messageID"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusNotFound, "File not found")
		return
	}
	message, err := s.rooms.FindRoomMessage(c.Request.Context(), room.ID, messageID)
	if err != nil || message.Kind != model.RoomMessageFile || message.File == nil {
		jsonError(c, http.StatusNotFound, "File not found")
		return
	}
	if info, err := os.Stat(message.File.Path); err != nil || !info.Mode().IsRegular() {
		jsonError(c, http.StatusNotFound, "File not found")
		return
	}
	contentType := message.File.MIMEType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(message.File.Filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": message.File.Filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(message.File.Path)
}

func newRoomMember(request roomIdentityRequest, userAgent string, now time.Time) (string, *model.RoomMember, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	nickname := cleanRoomText(request.Nickname, 40)
	if nickname == "" {
		generated, err := randomNickname()
		if err != nil {
			return "", nil, err
		}
		nickname = generated
	}
	member := &model.RoomMember{TokenHash: share.TokenDigest(token), Nickname: nickname, UserAgent: cleanRoomText(userAgent, 500), Device: cleanRoomText(request.Device, 80), OS: cleanRoomText(request.OS, 80), Browser: cleanRoomText(request.Browser, 80), LastSeenAt: now}
	if member.Device == "" {
		member.Device = "未知设备"
	}
	if member.OS == "" {
		member.OS = "未知系统"
	}
	if member.Browser == "" {
		member.Browser = "未知浏览器"
	}
	return token, member, nil
}

func normalizedRoomID(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }
func cleanRoomText(value string, limit int) string {
	value = strings.TrimSpace(value)
	for utf8.RuneCountInString(value) > limit {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}
func randomRoomID() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(90_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%05d", value.Int64()+10_000), nil
}
func randomNickname() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", value.Int64()+1000), nil
}

func (s *Server) roomSessionResponse(room *model.ShareRoom, member *model.RoomMember, token string) gin.H {
	return gin.H{"room": s.roomResponse(room, member.ID, s.now()), "token": token}
}
func (s *Server) roomResponse(room *model.ShareRoom, currentMemberID int64, now time.Time) gin.H {
	members := make([]gin.H, 0, len(room.Members)+1)
	found := false
	for _, m := range room.Members {
		if m.ID == currentMemberID {
			found = true
		}
		online := now.Sub(m.LastSeenAt) < 45*time.Second
		if s.hub != nil {
			online = s.hub.IsOnline(room.ID, m.ID)
		}
		members = append(members, memberResponse(m, online))
	}
	return gin.H{"id": room.PublicID, "name": room.Name, "has_password": room.PasswordHash != "", "members": members, "current_member_id": currentMemberID, "current_member_found": found, "created_at": room.CreatedAt}
}
func memberResponse(member model.RoomMember, online bool) gin.H {
	return gin.H{"id": member.ID, "nickname": member.Nickname, "device": member.Device, "os": member.OS, "browser": member.Browser, "user_agent": member.UserAgent, "is_owner": member.IsOwner, "online": online, "last_seen_at": member.LastSeenAt}
}
func roomMessageResponse(roomID string, message *model.RoomMessage) gin.H {
	result := gin.H{"id": message.ID, "kind": message.Kind, "source": message.Source, "text": message.Text, "created_at": message.CreatedAt, "sender": memberResponse(message.Member, true)}
	if message.File != nil {
		result["file"] = gin.H{"name": message.File.Filename, "size": message.File.Size, "mime_type": message.File.MIMEType, "download_url": "/api/rooms/" + roomID + "/messages/" + strconv.FormatInt(message.ID, 10) + "/file"}
	}
	return result
}
