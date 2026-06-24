package main

import (
	_ "ginchat/docs"
	"ginchat/models"
	"ginchat/router"
	"ginchat/utils"
)

// @title           ginchat API
// @version         1.0
// @description     IM 即时通讯项目 API 文档
// @host            localhost:8080
// @BasePath        /
func main() {
	utils.InitConfig()
	utils.InitMySQL()
	utils.InitRedis()
	models.GetUserList()
	r := router.Router()
	r.Run(":8080") // listens on 0.0.0.0:8080 by default
}
