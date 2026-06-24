package service

import (
	"fmt"
	"ginchat/models"
	"ginchat/utils"
	"math/rand"
	"strconv"

	"github.com/asaskevich/govalidator/v12"
	"github.com/gin-gonic/gin"
)

// CreateUser 新增用户
// @Summary      新增用户
// @Tags         用户管理
// @Param        name   query   string   true  "用户名"
// @Param        password   query   string   true  "密码"
// @Param        repassword   query   string   true  "确认密码"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/CreateUser [get]
func CreateUser(c *gin.Context) {
	user := models.UserBasic{}
	user.Name = c.Query("name")
	password := c.Query("password")
	repassword := c.Query("repassword")
	salt := fmt.Sprintf("%06d", rand.Int31())
	data := models.FindUserByName(user.Name)

	if password != repassword {
		c.JSON(-1, gin.H{
			"message": "密码不一致",
		})
		return
	}
	if data.Name != "" {
		c.JSON(-1, gin.H{
			"message": "用户已存在",
		})
		return
	}
	user.Salt = salt
	user.PassWord = utils.MakePassword(password, salt)
	models.CreateUser(user)
	c.JSON(200, gin.H{
		"message": "新增用户成功",
	})
}

// FindUserByNameAndPassword 通过用户名和密码查找用户
// @Summary      登入
// @Tags         用户管理
// @Param        name   query   string   true  "用户名"
// @Param        password   query   string   true  "密码"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/FindUserByNameAndPassword [post]
func FindUserByNameAndPassword(c *gin.Context) {
	data := models.UserBasic{}
	name := c.Query("name")
	password := c.Query("password")
	if password == "" {
		password = c.Query("pass_word")
	}
	user := models.FindUserByName(name)
	if user.Name == "" {
		c.JSON(-1, gin.H{
			"message": "用户不存在",
		})
		return
	}
	flag := utils.ValidatePassword(password, user.Salt, user.PassWord)
	if !flag {
		c.JSON(-1, gin.H{
			"message": "密码错误",
		})
		return
	}
	pwd := utils.MakePassword(password, user.Salt)
	data = models.FindUserByNameAndPassword(name, pwd)
	c.JSON(200, gin.H{
		"message": data,
	})
}

// GetUserList 获取用户列表
// @Summary      所有用户
// @Tags         用户管理
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/GetUserList [get]
func GetUserList(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": models.GetUserList(),
	})
}

// DeleteUser 删除用户
// @Summary      删除用户
// @Tags         用户管理
// @Param        id   query   string   false  "id"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/DeleteUser [get]
func DeleteUser(c *gin.Context) {
	user := models.UserBasic{}
	idStr := c.Query("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(-1, gin.H{
			"message": "无效的id",
		})
		return
	}
	user.ID = uint(id)
	models.DeleteUser(user)
	c.JSON(200, gin.H{
		"message": "删除成功",
	})
}

// UpdateUser 修改用户
// @Summary      修改 用户
// @Tags         用户管理
// @Param        id   formData   string   false  "id"
// @Param        name   formData   string   true  "用户名"
// @Param        password   formData   string   true  "密码"
// @Param        email   formData   string   true  "邮箱"
// @Param        phone   formData   string   true  "手机号"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/UpdateUser [post]
func UpdateUser(c *gin.Context) {
	user := models.UserBasic{}
	id, _ := strconv.Atoi(c.PostForm("id"))
	user.ID = uint(id)
	user.Name = c.PostForm("name")
	user.PassWord = c.PostForm("password")
	user.Email = c.PostForm("email")
	user.Phone = c.PostForm("phone")
	_, err := govalidator.ValidateStruct(user)
	if err != nil {
		c.JSON(-1, gin.H{
			"message": "数据验证错误",
		})
		return
	}
	models.UpdateUser(user)
	c.JSON(200, gin.H{
		"message": "修改成功",
	})
}
