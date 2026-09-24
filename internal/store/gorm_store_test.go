package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGORMStorePersistsFilesSeparately(t *testing.T) {
	store := newSQLiteStore(t)
	for _, legacyColumn := range []string{"filename", "file_path", "file_hash", "file_size", "mime_type"} {
		if store.db.Migrator().HasColumn(&model.Clip{}, legacyColumn) {
			t.Fatalf("new cb_clips unexpectedly contains file column %q", legacyColumn)
		}
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	path := filepath.Join(t.TempDir(), "shared-file")
	hash := hashBytes([]byte("shared"))

	first := fileClip("12345", "first.txt", path, hash, now.Add(-2*time.Hour), 3600)
	if err := store.Create(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if first.FileID == nil || first.File == nil || first.File.ID == 0 {
		t.Fatalf("file association was not populated: %#v", first)
	}

	second := fileClip("23456", "second.txt", path, hash, now, 3600)
	if err := store.Create(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	third := fileClip("34567", "second.txt", "/different/path", hash, now, 3600)
	if err := store.Create(context.Background(), third); err != nil {
		t.Fatal(err)
	}
	if second.FileID == nil || third.FileID == nil || *second.FileID != *third.FileID {
		t.Fatalf("same filename and SHA1 did not reuse file ID: %v/%v", second.FileID, third.FileID)
	}

	var clipCount, fileCount int64
	if err := store.db.Model(&model.Clip{}).Count(&clipCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.db.Model(&model.File{}).Count(&fileCount).Error; err != nil {
		t.Fatal(err)
	}
	if clipCount != 3 || fileCount != 2 {
		t.Fatalf("clip/file counts = %d/%d, want 3/2", clipCount, fileCount)
	}

	found, err := store.FindFile(context.Background(), hash, "second.txt", now)
	if err != nil || found.File == nil || found.File.Path != path {
		t.Fatalf("FindFile() = %#v, %v", found, err)
	}
	reusable, err := store.FindReusableFile(context.Background(), hash, int64(len("shared")), now)
	if err != nil || reusable.File == nil || reusable.File.SHA1 != hash {
		t.Fatalf("FindReusableFile() = %#v, %v", reusable, err)
	}

	paths, expired, err := store.CleanupExpired(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if expired != 1 || len(paths) != 1 || paths[0] != path {
		t.Fatalf("CleanupExpired() = %#v, %d", paths, expired)
	}
	references, err := store.CountFileReferences(context.Background(), path, now)
	if err != nil || references != 2 {
		t.Fatalf("CountFileReferences() = %d, %v", references, err)
	}
}

func TestOpenUsesSQLiteAndCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database", "clipbox.db")
	database, err := Open(context.Background(), "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("SQLite database was not created at %s: %v", path, err)
	}
}

func TestGORMStoreShareRoomLifecycle(t *testing.T) {
	database := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	room := &model.ShareRoom{PublicID: "ROOM1234", Name: "My devices"}
	owner := &model.RoomMember{TokenHash: strings.Repeat("a", 64), Nickname: "Laptop", Device: "Desktop", OS: "macOS", Browser: "Safari", LastSeenAt: now}
	if err := database.CreateRoom(ctx, room, owner); err != nil {
		t.Fatal(err)
	}
	if !owner.IsOwner || owner.RoomID != room.ID {
		t.Fatalf("owner was not associated with room: %#v", owner)
	}
	phone := &model.RoomMember{TokenHash: strings.Repeat("b", 64), Nickname: "Phone", Device: "iPhone", OS: "iOS", Browser: "Safari", LastSeenAt: now}
	joined, err := database.JoinRoom(ctx, room.PublicID, phone)
	if err != nil || joined.ID != room.ID {
		t.Fatalf("JoinRoom() = %#v, %v", joined, err)
	}
	message := &model.RoomMessage{RoomID: room.ID, MemberID: phone.ID, Kind: model.RoomMessageText, Source: model.MessageSourceClipboard, Text: "copied text", CreatedAt: now}
	if err := database.CreateRoomMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	messages, err := database.ListRoomMessages(ctx, room.ID, 0, 20)
	if err != nil || len(messages) != 1 || messages[0].Member.Nickname != "Phone" || messages[0].Source != model.MessageSourceClipboard {
		t.Fatalf("ListRoomMessages() = %#v, %v", messages, err)
	}
	if err := database.DeleteRoom(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.FindRoom(ctx, room.PublicID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindRoom() after delete error = %v, want ErrNotFound", err)
	}
}

func TestGORMStoreSharesFiveDigitCodeNamespace(t *testing.T) {
	database := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	room := &model.ShareRoom{PublicID: "54321", Name: ""}
	owner := &model.RoomMember{TokenHash: strings.Repeat("a", 64), Nickname: "Laptop", LastSeenAt: now}
	if err := database.CreateRoom(ctx, room, owner); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(ctx, fileClip("54321", "conflict.txt", "/tmp/conflict", hashBytes([]byte("conflict")), now, 3600)); !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() with room ID error = %v, want ErrConflict", err)
	}
	if err := database.DeleteRoom(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(ctx, fileClip("54321", "reused.txt", "/tmp/reused", hashBytes([]byte("reused")), now, 3600)); err != nil {
		t.Fatalf("Create() after room deletion: %v", err)
	}
}

func TestGORMStoreTransfersRoomOwnershipInJoinOrder(t *testing.T) {
	database := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	room := &model.ShareRoom{PublicID: "11111", Name: ""}
	owner := &model.RoomMember{TokenHash: strings.Repeat("a", 64), Nickname: "Owner", LastSeenAt: now}
	if err := database.CreateRoom(ctx, room, owner); err != nil {
		t.Fatal(err)
	}
	phone := &model.RoomMember{TokenHash: strings.Repeat("b", 64), Nickname: "Phone", LastSeenAt: now}
	if _, err := database.JoinRoom(ctx, room.PublicID, phone); err != nil {
		t.Fatal(err)
	}
	successor, err := database.TransferRoomOwnership(ctx, room.ID, owner.ID)
	if err != nil || successor != phone.ID {
		t.Fatalf("TransferRoomOwnership() = %d, %v; want %d", successor, err, phone.ID)
	}
	updated, err := database.FindRoom(ctx, room.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Members) != 2 || updated.Members[0].IsOwner || !updated.Members[1].IsOwner {
		t.Fatalf("owners after transfer = %#v", updated.Members)
	}
}

func TestGORMStoreRetainsRoomFilesForOneDay(t *testing.T) {
	database := newSQLiteStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "room-file")
	if err := os.WriteFile(path, []byte("room payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Filename: "room.txt", Path: path, SHA1: hashBytes([]byte("room payload")), Size: 12, MIMEType: "text/plain"}
	if err := database.db.Create(file).Error; err != nil {
		t.Fatal(err)
	}
	room := &model.ShareRoom{PublicID: "22222", Name: ""}
	owner := &model.RoomMember{TokenHash: strings.Repeat("a", 64), Nickname: "Owner", LastSeenAt: now}
	if err := database.CreateRoom(ctx, room, owner); err != nil {
		t.Fatal(err)
	}
	message := &model.RoomMessage{RoomID: room.ID, MemberID: owner.ID, Kind: model.RoomMessageFile, Source: model.MessageSourceUI, FileID: &file.ID, CreatedAt: now}
	if err := database.CreateRoomMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteRoom(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	var messageCount int64
	if err := database.db.Model(&model.RoomMessage{}).Count(&messageCount).Error; err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 {
		t.Fatalf("room messages after deletion = %d, want 0", messageCount)
	}
	if paths, files, err := database.CleanupRoomFileRetentions(ctx, now.Add(time.Hour)); err != nil || files != 0 || len(paths) != 0 {
		t.Fatalf("early CleanupRoomFileRetentions() = %#v, %d, %v", paths, files, err)
	}
	paths, files, err := database.CleanupRoomFileRetentions(ctx, now.Add(25*time.Hour))
	if err != nil || files != 1 || len(paths) != 1 || paths[0] != path {
		t.Fatalf("due CleanupRoomFileRetentions() = %#v, %d, %v", paths, files, err)
	}
}

func TestGORMStoreRollsBackFileOnCodeConflict(t *testing.T) {
	store := newSQLiteStore(t)
	now := time.Now().UTC()
	if err := store.Create(context.Background(), fileClip("12345", "first.txt", "/tmp/first", hashBytes([]byte("one")), now, 3600)); err != nil {
		t.Fatal(err)
	}
	conflicting := fileClip("12345", "second.txt", "/tmp/second", hashBytes([]byte("two")), now, 3600)
	err := store.Create(context.Background(), conflicting)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() error = %v, want ErrConflict", err)
	}
	if conflicting.File.ID != 0 || conflicting.FileID != nil {
		t.Fatalf("failed transaction retained file IDs: %#v", conflicting)
	}
	var fileCount int64
	if err := store.db.Model(&model.File{}).Count(&fileCount).Error; err != nil {
		t.Fatal(err)
	}
	if fileCount != 1 {
		t.Fatalf("file count = %d, want transaction rollback to retain 1", fileCount)
	}
	conflicting.Code = "23456"
	if err := store.Create(context.Background(), conflicting); err != nil {
		t.Fatalf("retry after allocating a new code: %v", err)
	}
}

func TestGORMStoreMigratesLegacyClipFiles(t *testing.T) {
	db := openSQLite(t)
	legacySchema := `CREATE TABLE cb_clips (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code VARCHAR(10) NOT NULL UNIQUE,
		content_type VARCHAR(20) NOT NULL,
		content TEXT,
		filename VARCHAR(255), file_path VARCHAR(500), file_hash VARCHAR(64), file_size BIGINT, mime_type VARCHAR(100),
		client_ip VARCHAR(45), access_count INTEGER NOT NULL, max_count INTEGER NOT NULL, expire_seconds INTEGER NOT NULL,
		created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
	)`
	if err := db.Exec(legacySchema).Error; err != nil {
		t.Fatal(err)
	}

	payload := []byte("legacy file")
	path := filepath.Join(t.TempDir(), "legacy.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := db.Exec(`INSERT INTO cb_clips
		(code, content_type, content, filename, file_path, file_hash, file_size, mime_type, access_count, max_count, expire_seconds, created_at, updated_at)
		VALUES (?, ?, NULL, ?, ?, ?, ?, ?, 10, 10, 3600, ?, ?)`,
		"12345", model.ContentFile, "legacy.bin", path, "legacy-sha256-placeholder", len(payload), "application/octet-stream", now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO cb_clips
		(code, content_type, content, access_count, max_count, expire_seconds, created_at, updated_at)
		VALUES (?, ?, ?, 10, 10, 3600, ?, ?)`, "23456", model.ContentText, "legacy text", now, now).Error; err != nil {
		t.Fatal(err)
	}

	store := &GORMStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	fileClip, err := store.ConsumeByCode(context.Background(), "12345", now)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := hashBytes(payload)
	if fileClip.File == nil || fileClip.File.Path != path || fileClip.File.SHA1 != wantHash || fileClip.File.Filename != "legacy.bin" {
		t.Fatalf("migrated file = %#v, want path/hash/name preserved", fileClip.File)
	}
	textClip, err := store.FindText(context.Background(), hashBytes([]byte("legacy text")), now)
	if err != nil || textClip.Content != "legacy text" {
		t.Fatalf("migrated text = %#v, %v", textClip, err)
	}

	// The data migration is idempotent even though legacy columns are retained.
	if err := store.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	var fileCount int64
	if err := db.Model(&model.File{}).Count(&fileCount).Error; err != nil {
		t.Fatal(err)
	}
	if fileCount != 1 {
		t.Fatalf("file count after second migration = %d, want 1", fileCount)
	}
}

func newSQLiteStore(t *testing.T) *GORMStore {
	t.Helper()
	store := &GORMStore{db: openSQLite(t)}
	if err := store.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store
}

func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func fileClip(code, filename, path, hash string, createdAt time.Time, expireSeconds int) *model.Clip {
	return &model.Clip{
		Code: code, ContentType: model.ContentFile, File: &model.File{
			Filename: filename, Path: path, SHA1: hash, Size: int64(len("shared")), MIMEType: "text/plain",
		},
		AccessCount: 10, MaxCount: 10, ExpireSeconds: expireSeconds, CreatedAt: createdAt,
	}
}

func hashBytes(payload []byte) string {
	hash := sha1.Sum(payload)
	return hex.EncodeToString(hash[:])
}
