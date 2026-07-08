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

// PaginatedData 分页数据结构
type PaginatedData struct {
	List  interface{} `json:"list"`  // 数据列表
	Total int64       `json:"total"` // 总条数
	Page  int         `json:"page"`  // 当前页码
}

// SuccessWithPagination 带分页信息的成功响应
func SuccessWithPagination(c *gin.Context, message string, list interface{}, total int64, page int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Data: PaginatedData{
			List:  list,
			Total: total,
			Page:  page,
		},
	})
}
