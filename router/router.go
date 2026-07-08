package router

import (
	"ginchat/models"
	"ginchat/service"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Router() *gin.Engine {
	r := gin.Default()
	r.GET("swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/index", service.GetIndex)

	// 用户相关
	r.GET("/user/CreateUser", service.CreateUser)
	r.GET("/user/GetUserList", service.GetUserList)
	r.GET("/user/DeleteUser", service.DeleteUser)
	r.POST("/user/UpdateUser", service.UpdateUser)
	r.POST("/user/FindUserByNameAndPassword", service.FindUserByNameAndPassword)
	r.GET("/user/SearchFriend", service.SearchFriend)
	r.POST("/user/AddFriend", service.AddFriend)
	r.GET("/user/GetChatRecord", models.GetChatRecord)
	r.POST("/user/MarkMessagesRead", service.MarkMessagesRead)
	//发送消息
	r.GET("/user/SendMsg", service.SendMsg)
	r.GET("/user/SendUserMsg", service.SendUserMsg)
	//上传图片
	r.Static("uploads", "./uploads")
	r.POST("/user/UploadImage", service.UploadImage)
	r.POST("/user/UploadAudio", service.UploadAudio)
	return r
}
