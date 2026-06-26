package models

import (
	"fmt"

	"gorm.io/gorm"
)

// Contact 人员关系
type Contact struct {
	gorm.Model
	OwnerId  uint   //谁的联系人
	TargetId uint   // 对应的谁
	Type     int    // 1 好友 2 群主
	Desc     string // 描述
}

func (table *Contact) TableName() string {
	return "contacts"
}

// SearchFriend 搜索好友
func SearchFriend(userId uint) []UserBasic {
	contacts := make([]Contact, 0)
	users := make([]UserBasic, 0)
	DB.Where("owner_id = ? AND type = ?", userId, 1).Find(&contacts)
	for _, v := range contacts {
		user := UserBasic{}
		DB.Where("id = ?", v.TargetId).First(&user)
		users = append(users, user)
	}
	return users
}

// AddFriend 添加好友（双向插入：两条 Contact 记录）
func AddFriend(userId uint, targetId uint) error {
	// 检查是否已是好友
	var count int64
	DB.Model(&Contact{}).Where("owner_id = ? AND target_id = ? AND type = 1", userId, targetId).Count(&count)
	if count > 0 {
		return fmt.Errorf("已是好友关系")
	}

	// 事务：双向插入
	return DB.Transaction(func(tx *gorm.DB) error {
		contact1 := Contact{OwnerId: userId, TargetId: targetId, Type: 1}
		if err := tx.Create(&contact1).Error; err != nil {
			return err
		}
		contact2 := Contact{OwnerId: targetId, TargetId: userId, Type: 1}
		if err := tx.Create(&contact2).Error; err != nil {
			return err
		}
		return nil
	})
}
