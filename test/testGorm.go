package main

import (
	"fmt"
	"ginchat/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := "root:123456@tcp(127.0.0.1:3306)/ginchat?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}

	if err := db.AutoMigrate(&models.UserBasic{}); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	// Create 创建
	user := models.UserBasic{
		Name:     "test",
		PassWord: "123456",
		Phone:    "12345678901",
		Email:    "test@example.com",
	}
	db.Create(&user)

	// Read 查询
	db.First(&user, user.ID)
	fmt.Printf("查询结果: %+v\n", user)

	// Update 更新
	db.Model(&user).Update("name", "test1")

	fmt.Println("MySQL 连接成功")
}
