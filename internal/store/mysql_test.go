package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	var clipCount, fileCount int64
	if err := store.db.Model(&model.Clip{}).Count(&clipCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.db.Model(&model.File{}).Count(&fileCount).Error; err != nil {
		t.Fatal(err)
	}
	if clipCount != 2 || fileCount != 2 {
		t.Fatalf("clip/file counts = %d/%d, want 2/2", clipCount, fileCount)
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
	if err != nil || references != 1 {
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
