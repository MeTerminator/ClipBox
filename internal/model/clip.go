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

// ShareRoom is a persistent, browser-to-browser conversation. The public ID
// is intentionally separate from the numeric primary key so it is safe to put
// in URLs and can be regenerated on collision.
type ShareRoom struct {
	ID        int64         `gorm:"primaryKey;autoIncrement"`
	PublicID  string        `gorm:"type:varchar(16);not null;uniqueIndex:idx_cb_share_rooms_public_id"`
	Name      string        `gorm:"type:varchar(80);not null"`
	Members   []RoomMember  `gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE"`
	Messages  []RoomMessage `gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ShareRoom) TableName() string { return "cb_share_rooms" }

// RoomMember represents one browser identity. Only a SHA-256 token digest is
// stored; the bearer token itself stays in that browser's localStorage.
type RoomMember struct {
	ID         int64  `gorm:"primaryKey;autoIncrement"`
	RoomID     int64  `gorm:"not null;uniqueIndex:idx_cb_room_member_token,priority:1;index:idx_cb_room_members_room"`
	TokenHash  string `gorm:"type:char(64);not null;uniqueIndex:idx_cb_room_member_token,priority:2"`
	Nickname   string `gorm:"type:varchar(40);not null"`
	UserAgent  string `gorm:"type:varchar(500);not null"`
	Device     string `gorm:"type:varchar(80);not null"`
	OS         string `gorm:"column:operating_system;type:varchar(80);not null"`
	Browser    string `gorm:"type:varchar(80);not null"`
	IsOwner    bool   `gorm:"not null;default:false"`
	LastSeenAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (RoomMember) TableName() string { return "cb_room_members" }

const (
	RoomMessageText        = "text"
	RoomMessageFile        = "file"
	MessageSourceUI        = "user"
	MessageSourceClipboard = "clipboard"
)

// RoomMessage deliberately separates kind (the payload shape) from source
// (why it was sent). Future clipboard sync can use the same endpoint with
// source=clipboard while retaining text/file payload semantics.
type RoomMessage struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	RoomID    int64      `gorm:"not null;index:idx_cb_room_messages_room_id"`
	MemberID  int64      `gorm:"not null;index:idx_cb_room_messages_member_id"`
	Member    RoomMember `gorm:"constraint:OnDelete:CASCADE"`
	Kind      string     `gorm:"type:varchar(20);not null"`
	Source    string     `gorm:"type:varchar(32);not null;default:user"`
	Text      string     `gorm:"type:longtext"`
	FileID    *int64     `gorm:"index:idx_cb_room_messages_file_id"`
	File      *File      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	CreatedAt time.Time  `gorm:"index:idx_cb_room_messages_created_at"`
}

func (RoomMessage) TableName() string { return "cb_room_messages" }

func (clip Clip) Expired(now time.Time) bool {
	return clip.CreatedAt.IsZero() || now.After(clip.CreatedAt.Add(time.Duration(clip.ExpireSeconds)*time.Second))
}
