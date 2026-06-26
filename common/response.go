package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一返回结构体
type Response struct {
	Code    int         `json:"code"`    // 0=成功, -1=失败
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 返回数据
}

// Success 成功响应
func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

// Fail 失败响应
func Fail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    -1,
		Message: message,
		Data:    nil,
	})
}
