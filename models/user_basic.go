package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

var DB *gorm.DB

type UserBasic struct {
	gorm.Model
	Name          string
	PassWord      string
	Phone         string `valid:"matches(^1[3-9][0-9]{9}$)"`
	Email         string `valid:"email"`
	ClientID      string
	Identity      string
	ClientPort    string
	LoginTime     *time.Time
	HeartbeatTime *time.Time
	LoginOutTime  *time.Time
	IsLogout      bool
	DeviceInfo    string
}

func (table *UserBasic) TableName() string {
	return "user_basic"
}
func GetUserList() []*UserBasic {
	date := make([]*UserBasic, 10)
	DB.Find(&date)
	for _, v := range date {
		fmt.Println(v)
	}
	return date
}
func CreateUser(user UserBasic) *gorm.DB {

	return DB.Create(&user)
}
func DeleteUser(user UserBasic) *gorm.DB {

	return DB.Delete(&user)
}
func UpdateUser(user UserBasic) *gorm.DB {

	return DB.Model(&user).Updates(map[string]interface{}{
		"name":      user.Name,
		"pass_word": user.PassWord,
		"phone":     user.Phone,
		"email":     user.Email,
	})
}
