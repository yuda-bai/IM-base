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
	Phone         string
	Email         string
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

	return DB.Updates(&user).Update("name", user.Name).Update("password", user.PassWord).Update("phone", user.Phone).Update("email", user.Email)
}
