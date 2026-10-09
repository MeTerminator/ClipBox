package server

import (
	"testing"

	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/gin-gonic/gin"
)

func TestRoomFileMessagesExposeContentIdentityAcrossRepeatedUploads(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567"
	first := roomMessageResponse("12345", &model.RoomMessage{ID: 1, File: &model.File{Filename: "original.txt", SHA1: hash, Size: 4}})["file"].(gin.H)
	repeated := roomMessageResponse("54321", &model.RoomMessage{ID: 2, File: &model.File{Filename: "renamed.txt", SHA1: hash, Size: 4}})["file"].(gin.H)
	if first["sha1"] != hash || repeated["sha1"] != hash || first["size"] != repeated["size"] {
		t.Fatalf("repeated content must have the same identity: %v, %v", first, repeated)
	}
	if first["download_url"] == repeated["download_url"] {
		t.Fatal("distinct messages must retain their own authenticated download URLs")
	}
}
