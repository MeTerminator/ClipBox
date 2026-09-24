package store

import (
	"context"
	"errors"
	"time"

	"github.com/MeTerminator/ClipBox/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *GORMStore) CreateRoom(ctx context.Context, room *model.ShareRoom, owner *model.RoomMember) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := reserveSharedCode(tx, room.PublicID, "room"); err != nil {
			return err
		}
		if err := tx.Omit("Members", "Messages").Create(room).Error; err != nil {
			return err
		}
		owner.RoomID, owner.IsOwner = room.ID, true
		return tx.Create(owner).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

func (s *GORMStore) JoinRoom(ctx context.Context, publicID string, member *model.RoomMember) (*model.ShareRoom, error) {
	room, err := s.FindRoom(ctx, publicID)
	if err != nil {
		return nil, err
	}
	member.RoomID = room.ID
	if err := s.db.WithContext(ctx).Create(member).Error; err != nil {
		return nil, err
	}
	return room, nil
}

func (s *GORMStore) FindRoom(ctx context.Context, publicID string) (*model.ShareRoom, error) {
	var room model.ShareRoom
	if err := s.db.WithContext(ctx).Preload("Members", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).Where("public_id = ?", publicID).First(&room).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &room, nil
}

func (s *GORMStore) ListRooms(ctx context.Context) ([]model.ShareRoom, error) {
	var rooms []model.ShareRoom
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&rooms).Error; err != nil {
		return nil, err
	}
	return rooms, nil
}

func (s *GORMStore) FindRoomMember(ctx context.Context, roomID int64, tokenHash string) (*model.RoomMember, error) {
	var member model.RoomMember
	if err := s.db.WithContext(ctx).Where("room_id = ? AND token_hash = ?", roomID, tokenHash).First(&member).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &member, nil
}

func (s *GORMStore) TouchRoomMember(ctx context.Context, id int64, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.RoomMember{}).Where("id = ?", id).Update("last_seen_at", now).Error
}

func (s *GORMStore) UpdateRoomName(ctx context.Context, roomID int64, name string) error {
	return s.db.WithContext(ctx).Model(&model.ShareRoom{}).Where("id = ?", roomID).Update("name", name).Error
}

func (s *GORMStore) UpdateRoomPassword(ctx context.Context, roomID int64, passwordHash string) error {
	return s.db.WithContext(ctx).Model(&model.ShareRoom{}).Where("id = ?", roomID).Update("password_hash", passwordHash).Error
}

func (s *GORMStore) UpdateRoomMemberNickname(ctx context.Context, memberID int64, nickname string) error {
	return s.db.WithContext(ctx).Model(&model.RoomMember{}).Where("id = ?", memberID).Update("nickname", nickname).Error
}

func (s *GORMStore) TransferRoomOwnership(ctx context.Context, roomID, fromMemberID int64) (int64, error) {
	var successorID int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RoomMember{}).Where("id = ?", fromMemberID).Update("is_owner", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.RoomMember{}).
			Where("room_id = ? AND id <> ? AND is_owner = false", roomID, fromMemberID).
			Order("created_at ASC, id ASC").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Limit(1).
			Pluck("id", &successorID).Error; err != nil {
			return err
		}
		if successorID == 0 {
			return nil
		}
		return tx.Model(&model.RoomMember{}).Where("id = ?", successorID).Update("is_owner", true).Error
	})
	return successorID, err
}

func (s *GORMStore) DeleteRoom(ctx context.Context, roomID int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var fileIDValues []*int64
		if err := tx.Model(&model.RoomMessage{}).Where("room_id = ? AND file_id IS NOT NULL", roomID).Distinct().Pluck("file_id", &fileIDValues).Error; err != nil {
			return err
		}
		fileIDs := make([]int64, 0, len(fileIDValues))
		for _, fileID := range fileIDValues {
			if fileID != nil {
				fileIDs = append(fileIDs, *fileID)
			}
		}
		var room model.ShareRoom
		if err := tx.Where("id = ?", roomID).First(&room).Error; err != nil {
			return normalizeNotFound(err)
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&model.RoomMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&model.RoomMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("code = ?", room.PublicID).Delete(&model.SharedCode{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.ShareRoom{}, roomID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		deleteAfter := time.Now().UTC().Add(24 * time.Hour)
		for _, fileID := range fileIDs {
			if fileID == 0 {
				continue
			}
			retention := model.RoomFileRetention{FileID: fileID, DeleteAfter: deleteAfter, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&retention)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				if err := tx.Model(&model.RoomFileRetention{}).Where("file_id = ? AND delete_after < ?", fileID, deleteAfter).Update("delete_after", deleteAfter).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *GORMStore) CreateRoomMessage(ctx context.Context, message *model.RoomMessage) error {
	return s.db.WithContext(ctx).Omit("Member", "File").Create(message).Error
}

func (s *GORMStore) ListRoomMessages(ctx context.Context, roomID, afterID int64, limit int) ([]model.RoomMessage, error) {
	var messages []model.RoomMessage
	err := s.db.WithContext(ctx).Preload("Member").Preload("File").Where("room_id = ? AND id > ?", roomID, afterID).Order("id ASC").Limit(limit).Find(&messages).Error
	return messages, err
}

func (s *GORMStore) FindRoomMessage(ctx context.Context, roomID, messageID int64) (*model.RoomMessage, error) {
	var message model.RoomMessage
	if err := s.db.WithContext(ctx).Preload("File").Where("room_id = ? AND id = ?", roomID, messageID).First(&message).Error; err != nil {
		return nil, normalizeNotFound(err)
	}
	return &message, nil
}

func (s *GORMStore) FindFileByClipCode(ctx context.Context, code string, now time.Time) (*model.File, error) {
	clip, err := s.FindByCode(ctx, code)
	if err != nil || clip.Expired(now) || clip.ContentType != model.ContentFile || clip.File == nil {
		return nil, ErrNotFound
	}
	return clip.File, nil
}
