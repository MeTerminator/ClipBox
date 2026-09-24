package share

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/MeTerminator/ClipBox/internal/store"
)

var (
	ErrInvalidMessage = errors.New("invalid room message")
	ErrInvalidRoom    = errors.New("invalid room data")
	ErrUnauthorized   = errors.New("room member authentication failed")
	ErrForbidden      = errors.New("room operation is forbidden")
)

// MessageInput is the transport-neutral command for creating a room message.
type MessageInput struct {
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Text     string `json:"text"`
	FileCode string `json:"file_code"`
}

// RenameRoom changes a room name. Every joined member may keep shared room
// metadata current; destructive room deletion remains owner-only.
func (s *Service) RenameRoom(ctx context.Context, room *model.ShareRoom, name string) error {
	return s.store.UpdateRoomName(ctx, room.ID, name)
}

// JoinRoom verifies the optional room password and registers a new member.
func (s *Service) JoinRoom(ctx context.Context, publicID, password string, member *model.RoomMember) (*model.ShareRoom, error) {
	room, err := s.store.FindRoom(ctx, strings.ToUpper(strings.TrimSpace(publicID)))
	if err != nil {
		return nil, err
	}
	if room.PasswordHash != "" {
		digest := TokenDigest(password)
		if subtle.ConstantTimeCompare([]byte(digest), []byte(room.PasswordHash)) != 1 {
			return nil, ErrUnauthorized
		}
	}
	return s.store.JoinRoom(ctx, room.PublicID, member)
}

// SetRoomPassword installs or clears a password. The limit counts Unicode
// characters so every character, including controls and emoji, is accepted.
func (s *Service) SetRoomPassword(ctx context.Context, room *model.ShareRoom, password string) error {
	if utf8.RuneCountInString(password) >= 64 {
		return ErrInvalidRoom
	}
	return s.store.UpdateRoomPassword(ctx, room.ID, TokenDigest(password))
}

// UpdateNickname changes only the authenticated member's display name.
func (s *Service) UpdateNickname(ctx context.Context, member *model.RoomMember, nickname string) error {
	if strings.TrimSpace(nickname) == "" {
		return ErrInvalidRoom
	}
	return s.store.UpdateRoomMemberNickname(ctx, member.ID, nickname)
}

// Service owns room authentication and message rules. HTTP and WebSocket
// transports call the same methods so validation and persistence cannot drift.
type Service struct {
	store       store.RoomStore
	now         func() time.Time
	maxTextSize int64
}

// NewService constructs the room application service.
func NewService(repository store.RoomStore, maxTextSize int64, now func() time.Time) *Service {
	return &Service{store: repository, maxTextSize: maxTextSize, now: now}
}

// Authenticate resolves a public room and verifies a plaintext member token
// against its persisted digest.
func (s *Service) Authenticate(ctx context.Context, publicID, token string) (*model.ShareRoom, *model.RoomMember, error) {
	room, err := s.store.FindRoom(ctx, strings.ToUpper(strings.TrimSpace(publicID)))
	if err != nil {
		return nil, nil, err
	}
	member, err := s.store.FindRoomMember(ctx, room.ID, TokenDigest(strings.TrimSpace(token)))
	if err != nil {
		return nil, nil, ErrUnauthorized
	}
	_ = s.store.TouchRoomMember(ctx, member.ID, s.now())
	return room, member, nil
}

// SendMessage validates and persists a room message before it is broadcast.
func (s *Service) SendMessage(ctx context.Context, room *model.ShareRoom, member *model.RoomMember, input MessageInput) (*model.RoomMessage, error) {
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.Source = strings.ToLower(strings.TrimSpace(input.Source))
	if input.Source == "" {
		input.Source = model.MessageSourceUI
	}
	if input.Source != model.MessageSourceUI && input.Source != model.MessageSourceClipboard {
		return nil, ErrInvalidMessage
	}
	if input.Source == model.MessageSourceClipboard && input.Kind != model.RoomMessageText {
		return nil, ErrInvalidMessage
	}
	message := &model.RoomMessage{RoomID: room.ID, MemberID: member.ID, Kind: input.Kind, Source: input.Source, CreatedAt: s.now()}
	switch input.Kind {
	case model.RoomMessageText:
		message.Text = input.Text
		if strings.TrimSpace(message.Text) == "" || int64(len([]byte(message.Text))) > s.maxTextSize {
			return nil, ErrInvalidMessage
		}
	case model.RoomMessageFile:
		file, err := s.store.FindFileByClipCode(ctx, strings.TrimSpace(input.FileCode), s.now())
		if err != nil {
			return nil, ErrInvalidMessage
		}
		message.FileID, message.File = &file.ID, file
	default:
		return nil, ErrInvalidMessage
	}
	if err := s.store.CreateRoomMessage(ctx, message); err != nil {
		return nil, err
	}
	message.Member = *member
	return message, nil
}

// TokenDigest returns the one-way representation persisted for member tokens.
func TokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
