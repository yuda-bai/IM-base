package service

import "github.com/gin-gonic/gin"

// GetIndex 首页接口
// @Summary      首页
// @Description  返回欢迎信息
// @Tags         首页
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /index [get]
func GetIndex(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}
