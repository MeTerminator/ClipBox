package store

import (
	"context"
	"errors"
	"time"

	"github.com/MeTerminator/ClipBox/internal/model"
)

var (
	ErrNotFound = errors.New("clip not found")
	ErrConflict = errors.New("clip code already exists")
)

type Store interface {
	Create(context.Context, *model.Clip) error
	FindByCode(context.Context, string) (*model.Clip, error)
	ConsumeByCode(context.Context, string, time.Time) (*model.Clip, error)
	FindText(context.Context, string, time.Time) (*model.Clip, error)
	FindFile(context.Context, string, string, time.Time) (*model.Clip, error)
	FindReusableFile(context.Context, string, int64, time.Time) (*model.Clip, error)
	CleanupExpired(context.Context, time.Time) ([]string, int64, error)
	CountFileReferences(context.Context, string, time.Time) (int, error)
	Close() error
}

// RoomStore is kept separate so lightweight Store implementations used by
// clip-only embedders and tests remain source compatible.
type RoomStore interface {
	CreateRoom(context.Context, *model.ShareRoom, *model.RoomMember) error
	JoinRoom(context.Context, string, *model.RoomMember) (*model.ShareRoom, error)
	FindRoom(context.Context, string) (*model.ShareRoom, error)
	ListRooms(context.Context) ([]model.ShareRoom, error)
	FindRoomMember(context.Context, int64, string) (*model.RoomMember, error)
	UpdateRoomName(context.Context, int64, string) error
	TouchRoomMember(context.Context, int64, time.Time) error
	DeleteRoom(context.Context, int64) error
	CreateRoomMessage(context.Context, *model.RoomMessage) error
	ListRoomMessages(context.Context, int64, int64, int) ([]model.RoomMessage, error)
	FindRoomMessage(context.Context, int64, int64) (*model.RoomMessage, error)
	FindFileByClipCode(context.Context, string, time.Time) (*model.File, error)
}
