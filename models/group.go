package models

import (
	"gorm.io/gorm"
)

// GroupBasic 群
type GroupBasic struct {
	gorm.Model
	Name    string `gorm:"not null"`
	OwnerId uint
	Icon    string
	Type    int // 1 公开 2 私有
	Desc    string
}

func (table *GroupBasic) TableName() string {
	return "group_basic"
}
