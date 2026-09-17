package share

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MeTerminator/ClipBox/internal/model"
)

func TestClipboardSourceOnlyAcceptsText(t *testing.T) {
	service := NewService(nil, 1024, time.Now)
	_, err := service.SendMessage(context.Background(), &model.ShareRoom{}, &model.RoomMember{}, MessageInput{
		Kind:     model.RoomMessageFile,
		Source:   model.MessageSourceClipboard,
		FileCode: "12345",
	})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("clipboard file error = %v, want ErrInvalidMessage", err)
	}
}
