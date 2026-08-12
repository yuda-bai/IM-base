package models

import (
	"fmt"

	"gorm.io/gorm"
)

// Contact 人员关系
type Contact struct {
	gorm.Model
	OwnerId  uint   `gorm:"index:uk_contact_relation,unique,priority:1;not null" json:"ownerId"`  //谁的联系人
	TargetId uint   `gorm:"index:uk_contact_relation,unique,priority:2;not null" json:"targetId"` // 对应的谁
	Type     int    `gorm:"index:uk_contact_relation,unique,priority:3;not null" json:"type"`     // 1 好友 2 群主
	Desc     string // 描述
}

func (table *Contact) TableName() string {
	return "contacts"
}

// SearchFriend 搜索好友列表
func SearchFriend(userId uint) []UserBasic {
	users := make([]UserBasic, 0)
	DB.Table("user_basic").
		Select("user_basic.*").
		Joins("INNER JOIN contacts on user_basic.id = contacts.target_id").
		Where("contacts.owner_id = ? AND contacts.type = ?", userId, 1).
		Find(&users)
	return users
}

// SearchFriendById 是否是好友
func SearchFriendById(userId uint, targetId uint) bool {
	if DB == nil {
		return false
	}
	var count int64
	DB.Model(&Contact{}).
		Where("owner_id = ? AND target_id = ? AND type = ?", userId, targetId, 1).
		Count(&count)
	return count > 0
}

// AddFriend 已废弃。好友关系只能由好友申请通过事务创建。
func AddFriend(userId uint, targetId uint) error {
	return fmt.Errorf("direct friend creation is disabled; approve a friend request")
}

// CreateFriendshipTx 在调用方事务中幂等创建双向好友关系。
func CreateFriendshipTx(tx *gorm.DB, userID, targetID uint) error {
	if tx == nil {
		return fmt.Errorf("database is not initialized")
	}
	if userID == 0 || targetID == 0 || userID == targetID {
		return fmt.Errorf("invalid friendship users")
	}
	for _, relation := range []Contact{
		{OwnerId: userID, TargetId: targetID, Type: 1},
		{OwnerId: targetID, TargetId: userID, Type: 1},
	} {
		var existing Contact
		result := tx.Where("owner_id = ? AND target_id = ? AND type = ?", relation.OwnerId, relation.TargetId, relation.Type).First(&existing)
		if result.Error == nil {
			continue
		}
		if result.Error != gorm.ErrRecordNotFound {
			return result.Error
		}
		if err := tx.Create(&relation).Error; err != nil {
			// Another concurrent transaction may have inserted the same unique
			// relation between First and Create. Re-read and treat it as success.
			var concurrent Contact
			if lookupErr := tx.Where("owner_id = ? AND target_id = ? AND type = ?", relation.OwnerId, relation.TargetId, relation.Type).First(&concurrent).Error; lookupErr == nil {
				continue
			}
			return err
		}
	}
	return nil
}

// PushToUser 将业务事件推送给在线用户；失败或离线只返回，不影响数据库事务。
func PushToUser(userID uint, payload []byte) {
	if userID == 0 {
		return
	}
	sendMsg(int64(userID), payload)
}
