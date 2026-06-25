package main

import (
	"ginchat/models"
	"ginchat/utils"
)

func main() {
	utils.InitConfig()
	utils.InitMySQL()

	db := models.DB
	//db.AutoMigrate(&models.Message{})
	db.AutoMigrate(&models.Contact{})
	db.AutoMigrate(&models.Message{})
	db.AutoMigrate(&models.GroupBasic{})
	//// Create 创建
	//user := models.UserBasic{
	//	Name:     "test",
	//	PassWord: "123456",
	//	Phone:    "12345678901",
	//	Email:    "test@example.com",
	//}
	//result := db.Create(&user)
	//if result.Error != nil {
	//	fmt.Println("插入失败:", result.Error)
	//	return
	//}
	//fmt.Printf("插入成功，影响行数: %d，新用户 ID: %d\n", result.RowsAffected, user.ID)
	//
	//// Read 查询
	//var found models.UserBasic
	//result = db.First(&found, user.ID)
	//if result.Error != nil {
	//	fmt.Println("查询失败:", result.Error)
	//	return
	//}
	//fmt.Printf("查询结果: %+v\n", found)
	//
	//// Update 更新
	//result = db.Model(&found).Update("name", "test_updated")
	//if result.Error != nil {
	//	fmt.Println("更新失败:", result.Error)
	//	return
	//}
	//fmt.Printf("更新成功，影响行数: %d\n", result.RowsAffected)
	//
	//// 最终验证
	//var count int64
	//db.Model(&models.UserBasic{}).Count(&count)
	//fmt.Printf("数据库中共有 %d 条用户记录\n", count)
	//
	//fmt.Println("MySQL 连接成功")
}
