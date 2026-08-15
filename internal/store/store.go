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
