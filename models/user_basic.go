package models

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	DB    *gorm.DB
	Redis *redis.Client
)

type UserBasic struct {
	gorm.Model
	Name          string
	PassWord      string
	Salt          string
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

// FindUserByName 通过用户名查找用户
func FindUserByName(name string) UserBasic {
	user := UserBasic{}
	DB.Where("name = ?", name).First(&user)
	return user
}

// FindUserByNameAndPassword 通过用户名和密码查找用户
func FindUserByNameAndPassword(name string, password string) UserBasic {
	user := UserBasic{}
	DB.Where("name = ? and pass_word=? ", name, password).First(&user)
	return user
}

// UpdateIdentity 更新用户身份标识(token)
func UpdateIdentity(userID uint, identity string) {
	DB.Model(&UserBasic{}).Where("id = ?", userID).Update("identity", identity)
}

// FindUserByPhone 通过电话号码查找用户
func FindUserByPhone(phone string) *gorm.DB {
	return DB.Where("phone = ?", phone).First(&UserBasic{})
}

// FindUserByEmail 通过email查找用户
func FindUserByEmail(email string) *gorm.DB {
	return DB.Where("email = ?", email).First(&UserBasic{})
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
