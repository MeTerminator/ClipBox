package share

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/MeTerminator/ClipBox/internal/store"
)

var (
	ErrInvalidMessage = errors.New("invalid room message")
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

// RenameRoom changes a room name when member owns the room.
func (s *Service) RenameRoom(ctx context.Context, room *model.ShareRoom, member *model.RoomMember, name string) error {
	if !member.IsOwner {
		return ErrForbidden
	}
	return s.store.UpdateRoomName(ctx, room.ID, name)
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
	message := &model.RoomMessage{RoomID: room.ID, MemberID: member.ID, Kind: input.Kind, Source: input.Source, CreatedAt: s.now()}
	switch input.Kind {
	case model.RoomMessageText:
		message.Text = strings.TrimSpace(input.Text)
		if message.Text == "" || int64(len([]byte(message.Text))) > s.maxTextSize {
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
