package service

import (
	"fmt"
	"ginchat/common"
	"ginchat/models"
	"ginchat/utils"
	"math/rand"
	"strconv"
	"time"

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
		common.Fail(c, "密码不一致")
		return
	}
	if data.Name != "" {
		common.Fail(c, "用户已存在")
		return
	}
	user.Salt = salt
	user.PassWord = utils.MakePassword(password, salt)
	models.CreateUser(user)
	common.Success(c, "新增用户成功", nil)
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
	name := c.Query("name")
	password := c.Query("password")
	if password == "" {
		password = c.Query("pass_word")
	}
	user := models.FindUserByName(name)
	if user.Name == "" {
		common.Fail(c, "用户不存在")
		return
	}
	flag := utils.ValidatePassword(password, user.Salt, user.PassWord)
	if !flag {
		common.Fail(c, "密码错误")
		return
	}
	// 生成token并更新用户身份标识
	str := fmt.Sprintf("%d", time.Now().Unix())
	temp := utils.Md5Encode(str)
	models.UpdateIdentity(user.ID, temp)
	user.Identity = temp
	common.Success(c, "登录成功", user)
}

// GetUserList 获取用户列表
// @Summary      所有用户
// @Tags         用户管理
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/GetUserList [get]
func GetUserList(c *gin.Context) {
	common.Success(c, "获取成功", models.GetUserList())
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
		common.Fail(c, "无效的id")
		return
	}
	user.ID = uint(id)
	models.DeleteUser(user)
	common.Success(c, "删除成功", nil)
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
		common.Fail(c, "数据验证错误")
		return
	}
	models.UpdateUser(user)
	common.Success(c, "修改成功", nil)
}
