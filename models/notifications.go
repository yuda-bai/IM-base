package models

import (
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	gorm.Model
	UserID  uint       `gorm:"not null;index:idx_notification_user_read,priority:1" json:"userId"`
	Type    string     `gorm:"size:50;not null;index:idx_notification_biz,priority:1" json:"type"`
	BizID   uint       `gorm:"not null;index:idx_notification_biz,priority:2" json:"bizId"`
	Title   string     `gorm:"size:100;not null" json:"title"`
	Content string     `gorm:"size:255;not null;default:''" json:"content"`
	IsRead  bool       `gorm:"not null;default:false;index:idx_notification_user_read,priority:2" json:"isRead"`
	ReadAt  *time.Time `gorm:"index" json:"readAt"`
}

func (Notification) TableName() string { return "notifications" }

func CreateNotificationTx(tx *gorm.DB, userID, bizID uint, title, content string) error {
	return tx.Create(&Notification{UserID: userID, Type: "friend_request", BizID: bizID, Title: title, Content: content}).Error
}
