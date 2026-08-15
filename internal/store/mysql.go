package store

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MeTerminator/ClipBox/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMStore struct {
	db *gorm.DB
}

func OpenMySQL(ctx context.Context, dsn string) (*GORMStore, error) {
	return open(ctx, "mysql", dsn)
}

func Open(ctx context.Context, driver, dsn string) (*GORMStore, error) {
	return open(ctx, strings.ToLower(strings.TrimSpace(driver)), strings.TrimSpace(dsn))
}

func open(ctx context.Context, driver, dsn string) (*GORMStore, error) {
	var dialector gorm.Dialector
	maxOpen, maxIdle := 20, 10
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite", "sqlite3":
		if err := prepareSQLitePath(dsn); err != nil {
			return nil, err
		}
		dsn = sqlitePragmaDSN(dsn)
		dialector = sqlite.Open(dsn)
		maxOpen, maxIdle = 1, 1
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", driver, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get database connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect to %s: %w", driver, err)
	}

	store := &GORMStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return store, nil
}

func prepareSQLitePath(dsn string) error {
	path := strings.TrimPrefix(dsn, "file:")
	path, _, _ = strings.Cut(path, "?")
	if path == "" || path == ":memory:" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create SQLite directory: %w", err)
	}
	return nil
}

func sqlitePragmaDSN(dsn string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on"
}

func (s *GORMStore) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func (s *GORMStore) migrate(ctx context.Context) error {
	db := s.db.WithContext(ctx)
	if err := db.AutoMigrate(&model.File{}, &model.Clip{}); err != nil {
		return fmt.Errorf("migrate database schema: %w", err)
	}
	if err := s.backfillTextSHA1(ctx); err != nil {
		return fmt.Errorf("backfill text SHA1: %w", err)
	}

	// GORM deliberately keeps legacy columns on upgraded installations. Once
	// copied, application reads and writes use cb_files/file_id exclusively.
	if db.Migrator().HasColumn("cb_clips", "file_path") {
		if _, err := s.backfillLegacyFileSHA1(ctx); err != nil {
			return fmt.Errorf("backfill legacy file SHA1: %w", err)
		}
		if err := s.migrateLegacyFiles(ctx); err != nil {
			return fmt.Errorf("migrate legacy files: %w", err)
		}
	}
	return nil
}

func (s *GORMStore) backfillTextSHA1(ctx context.Context) error {
	var clips []model.Clip
	if err := s.db.WithContext(ctx).
		Select("id", "content").
		Where("content_type = ? AND content IS NOT NULL AND (content_hash IS NULL OR content_hash = '')", model.ContentText).
		Find(&clips).Error; err != nil {
		return err
	}
	for _, clip := range clips {
		hash := sha1.Sum([]byte(clip.Content))
		if err := s.db.WithContext(ctx).Model(&model.Clip{}).
			Where("id = ?", clip.ID).
			Update("content_hash", hex.EncodeToString(hash[:])).Error; err != nil {
			return err
		}
	}
	return nil
}

type legacyFile struct {
	ClipID    int64     `gorm:"column:clip_id"`
	FileID    *int64    `gorm:"column:file_id"`
	Filename  string    `gorm:"column:filename"`
	FilePath  string    `gorm:"column:file_path"`
	FileHash  string    `gorm:"column:file_hash"`
	FileSize  int64     `gorm:"column:file_size"`
	MIMEType  string    `gorm:"column:mime_type"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (s *GORMStore) backfillLegacyFileSHA1(ctx context.Context) (int, error) {
	var rows []legacyFile
	if err := s.db.WithContext(ctx).Table("cb_clips").
		Select("id AS clip_id, file_id, file_path, COALESCE(file_hash, '') AS file_hash").
		Where("content_type = ? AND file_path IS NOT NULL AND file_path <> ''", model.ContentFile).
		Find(&rows).Error; err != nil {
		return 0, err
	}

	updated := 0
	for _, row := range rows {
		normalized := strings.ToLower(row.FileHash)
		if validSHA1(normalized) {
			if normalized == row.FileHash {
				continue
			}
		} else {
			var err error
			normalized, err = hashFile(row.FilePath)
			if err != nil {
				continue
			}
		}
		result := s.db.WithContext(ctx).Table("cb_clips").Where("id = ?", row.ClipID).Update("file_hash", normalized)
		if result.Error != nil {
			return updated, result.Error
		}
		if row.FileID != nil {
			if err := s.db.WithContext(ctx).Model(&model.File{}).Where("id = ?", *row.FileID).Update("sha1", normalized).Error; err != nil {
				return updated, err
			}
		}
		updated += int(result.RowsAffected)
	}
	return updated, nil
}

func (s *GORMStore) migrateLegacyFiles(ctx context.Context) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []legacyFile
		if err := tx.Table("cb_clips").
			Select(`id AS clip_id, COALESCE(filename, '') AS filename, COALESCE(file_path, '') AS file_path,
				COALESCE(file_hash, '') AS file_hash, COALESCE(file_size, 0) AS file_size,
				COALESCE(mime_type, '') AS mime_type, created_at, updated_at`).
			Where("content_type = ? AND file_id IS NULL", model.ContentFile).
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			file := model.File{
				Filename: row.Filename, Path: row.FilePath, SHA1: row.FileHash, Size: row.FileSize,
				MIMEType: row.MIMEType, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			}
			if file.Filename == "" {
				file.Filename = "file"
			}
			if file.MIMEType == "" {
				file.MIMEType = "application/octet-stream"
			}
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Clip{}).Where("id = ?", row.ClipID).Update("file_id", file.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GORMStore) Create(ctx context.Context, clip *model.Clip) error {
	var file *model.File
	if clip.ContentType == model.ContentFile {
		file = clip.File
		if file != nil {
			file.ID = 0
		}
		clip.FileID = nil
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if clip.ContentType == model.ContentFile {
			if file == nil {
				return fmt.Errorf("file clip has no file metadata")
			}
			if err := tx.Create(file).Error; err != nil {
				return err
			}
			clip.FileID = &file.ID
		}
		return tx.Omit("File").Create(clip).Error
	})
	if err != nil && file != nil {
		file.ID = 0
		clip.FileID = nil
	}
	if err != nil {
		clip.ID = 0
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

func (s *GORMStore) ConsumeByCode(ctx context.Context, code string, now time.Time) (*model.Clip, error) {
	var clip model.Clip
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("File").Where("code = ?", code).First(&clip).Error; err != nil {
			return err
		}
		if clip.Expired(now) || clip.AccessCount <= 0 {
			return ErrNotFound
		}
		if err := tx.Model(&model.Clip{}).Where("id = ?", clip.ID).Updates(map[string]any{
			"access_count": gorm.Expr("access_count - 1"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		clip.AccessCount--
		clip.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, normalizeNotFound(err)
	}
	return &clip, nil
}

func (s *GORMStore) FindByCode(ctx context.Context, code string) (*model.Clip, error) {
	var clip model.Clip
	if err := s.db.WithContext(ctx).Preload("File").Where("code = ?", code).First(&clip).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &clip, nil
}

func (s *GORMStore) FindText(ctx context.Context, hash string, now time.Time) (*model.Clip, error) {
	return s.findActive(s.db.WithContext(ctx).
		Where("content_type = ? AND content_hash = ?", model.ContentText, hash).
		Order("created_at DESC").Limit(20), now)
}

func (s *GORMStore) FindFile(ctx context.Context, hash, filename string, now time.Time) (*model.Clip, error) {
	return s.findActive(s.db.WithContext(ctx).
		Joins("JOIN cb_files ON cb_files.id = cb_clips.file_id").
		Where("cb_clips.content_type = ? AND cb_files.sha1 = ? AND cb_files.filename = ?", model.ContentFile, hash, filename).
		Order("cb_clips.created_at DESC").Limit(20), now)
}

func (s *GORMStore) FindReusableFile(ctx context.Context, hash string, size int64, now time.Time) (*model.Clip, error) {
	return s.findActive(s.db.WithContext(ctx).
		Joins("JOIN cb_files ON cb_files.id = cb_clips.file_id").
		Where("cb_clips.content_type = ? AND cb_files.sha1 = ? AND cb_files.size = ?", model.ContentFile, hash, size).
		Order("cb_clips.created_at DESC").Limit(20), now)
}

func (s *GORMStore) findActive(query *gorm.DB, now time.Time) (*model.Clip, error) {
	var clips []model.Clip
	if err := query.Preload("File").Find(&clips).Error; err != nil {
		return nil, err
	}
	for i := range clips {
		if !clips[i].Expired(now) {
			return &clips[i], nil
		}
	}
	return nil, ErrNotFound
}

func (s *GORMStore) CleanupExpired(ctx context.Context, now time.Time) ([]string, int64, error) {
	var paths []string
	var expiredCount int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []model.Clip
		if err := tx.Select("id", "file_id", "created_at", "expire_seconds").Find(&candidates).Error; err != nil {
			return err
		}

		expiredIDs := make([]int64, 0)
		fileIDs := make(map[int64]struct{})
		for _, clip := range candidates {
			if !clip.Expired(now) {
				continue
			}
			expiredIDs = append(expiredIDs, clip.ID)
			if clip.FileID != nil {
				fileIDs[*clip.FileID] = struct{}{}
			}
		}
		if len(expiredIDs) == 0 {
			return nil
		}

		result := tx.Where("id IN ?", expiredIDs).Delete(&model.Clip{})
		if result.Error != nil {
			return result.Error
		}
		expiredCount = result.RowsAffected

		for fileID := range fileIDs {
			var references int64
			if err := tx.Model(&model.Clip{}).Where("file_id = ?", fileID).Count(&references).Error; err != nil {
				return err
			}
			if references != 0 {
				continue
			}
			var file model.File
			if err := tx.First(&file, fileID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			paths = append(paths, file.Path)
			if err := tx.Delete(&file).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(paths)
	paths = compactStrings(paths)
	return paths, expiredCount, nil
}

func (s *GORMStore) CountFileReferences(ctx context.Context, path string, now time.Time) (int, error) {
	var clips []model.Clip
	if err := s.db.WithContext(ctx).
		Select("cb_clips.created_at", "cb_clips.expire_seconds").
		Joins("JOIN cb_files ON cb_files.id = cb_clips.file_id").
		Where("cb_files.path = ?", path).
		Find(&clips).Error; err != nil {
		return 0, err
	}
	count := 0
	for _, clip := range clips {
		if !clip.Expired(now) {
			count++
		}
	}
	return count, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validSHA1(value string) bool {
	if len(value) != sha1.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}
