package model

import "time"

const (
	ContentText = "text/plain"
	ContentLink = "link"
	ContentFile = "file"
)

type Clip struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	Code          string    `gorm:"type:varchar(10);not null;uniqueIndex:idx_cb_clips_code"`
	ContentType   string    `gorm:"type:varchar(20);not null;index:idx_cb_clips_content_type"`
	Content       string    `gorm:"type:longtext"`
	ContentHash   string    `gorm:"type:char(40);index:idx_cb_clips_content_hash"`
	FileID        *int64    `gorm:"index:idx_cb_clips_file_id"`
	File          *File     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	ClientIP      string    `gorm:"type:varchar(45)"`
	AccessCount   int       `gorm:"not null;default:1000"`
	MaxCount      int       `gorm:"not null;default:1000"`
	ExpireSeconds int       `gorm:"not null;default:86400"`
	CreatedAt     time.Time `gorm:"index:idx_cb_clips_created_at"`
	UpdatedAt     time.Time
}

func (Clip) TableName() string { return "cb_clips" }

// File stores metadata for one logical file upload. Multiple records may point
// at the same content-addressed path when instant upload reuses the bytes.
type File struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Filename  string `gorm:"type:varchar(255);not null;index:idx_cb_files_filename_sha1,priority:1"`
	Path      string `gorm:"type:varchar(500);not null;index:idx_cb_files_path"`
	SHA1      string `gorm:"type:char(40);not null;index:idx_cb_files_sha1,priority:2;index:idx_cb_files_filename_sha1,priority:2"`
	Size      int64  `gorm:"not null"`
	MIMEType  string `gorm:"type:varchar(100);not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (File) TableName() string { return "cb_files" }

func (clip Clip) Expired(now time.Time) bool {
	return clip.CreatedAt.IsZero() || now.After(clip.CreatedAt.Add(time.Duration(clip.ExpireSeconds)*time.Second))
}
