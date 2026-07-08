package service

import (
	"context"
	"fmt"
	"ginchat/common"
	"ginchat/models"
	"ginchat/utils"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator/v12"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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

// WebSocket 升级器
var upGrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func SendMsg(c *gin.Context) {
	ws, err := upGrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(ws *websocket.Conn) {
		err = ws.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(ws)
	MsgHandler(ws, c)
}
func MsgHandler(ws *websocket.Conn, c *gin.Context) {
	// 订阅Redis频道
	sub := models.Redis.Subscribe(c, common.PublishKey)
	defer sub.Close()
	fmt.Println("订阅成功")

	// 获取Redis消息通道
	redisMsgCh := sub.Channel()

	// 发送欢迎消息到Redis，通过Pub/Sub广播给所有客户端
	welcomeMsg := `{"userId":0,"userName":"系统","content":"欢迎来到聊天室！","type":"system","messageId":"0"}`
	models.Redis.Publish(c, common.PublishKey, welcomeMsg).Err()

	// 启动goroutine读取WebSocket客户端消息
	wsMsgCh := make(chan []byte)
	go func() {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				wsMsgCh <- nil
				return
			}
			wsMsgCh <- msg
		}
	}()

	for {
		select {
		// 读取WebSocket客户端发来的消息
		case msg := <-wsMsgCh:
			if msg == nil {
				fmt.Println("客户端断开连接")
				return
			}
			//保存消息到数据库
			models.Dispatch(msg)
			// 将客户端消息发布到Redis
			err := models.Redis.Publish(c, common.PublishKey, string(msg)).Err()
			if err != nil {
				fmt.Println("发布消息失败:", err)
			}

		// 读取Redis订阅消息，推送给WebSocket客户端
		case redisMsg := <-redisMsgCh:
			tm := time.Now().Format("2006-01-02 15:04:05")
			m := fmt.Sprintf("[ws][%s]:%s", tm, redisMsg.Payload)
			fmt.Println("收到Redis消息:", m)
			err := ws.WriteMessage(1, []byte(m))
			if err != nil {
				fmt.Println("写入消息失败")
				return
			}
		}
	}
}
func SendUserMsg(c *gin.Context) {
	models.Chat(c.Writer, c.Request)
}

// SearchFriend 获取好友列表
// @Summary      获取好友列表
// @Tags         用户管理
// @Param        userid   query   uint   true  "用户ID"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/SearchFriend [get]
func SearchFriend(c *gin.Context) {
	idStr := c.Query("userid")
	userid, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.Fail(c, "无效的用户ID")
		return
	}
	friends := models.SearchFriend(uint(userid))
	common.Success(c, "获取好友列表成功", friends)
}

// AddFriend 添加好友
// @Summary      添加好友
// @Tags         用户管理
// @Param        userId   formData   uint   true  "当前用户ID"
// @Param        targetId   formData   uint   true  "目标好友ID"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/AddFriend [post]
func AddFriend(c *gin.Context) {
	userIdStr := c.PostForm("userId")
	targetIdStr := c.PostForm("targetId")

	userId, err := strconv.ParseUint(userIdStr, 10, 64)
	if err != nil {
		common.Fail(c, "无效的用户ID")
		return
	}
	targetId, err := strconv.ParseUint(targetIdStr, 10, 64)
	if err != nil {
		common.Fail(c, "无效的目标好友ID")
		return
	}

	if userId == targetId {
		common.Fail(c, "不能添加自己为好友")
		return
	}

	err = models.AddFriend(uint(userId), uint(targetId))
	if err != nil {
		common.Fail(c, err.Error())
		return
	}
	common.Success(c, "添加好友成功", nil)
}

// GetChatRecord 获取与某个用户的聊天记录
// @Summary      获取与某个用户的聊天记录
// @Tags         用户管理
// @Param        id   formData   uint   true  "当前用户ID"
// @Param        targetId   formData   uint   true  "目标用户ID"
// @Param        page   formData   uint   true  "页码"
// @Param        pageSize   formData   uint   true  "每页数量"
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /user/GetChatRecord [get]
func GetChatRecord(c *gin.Context) {
	user := models.UserBasic{}
	id, _ := strconv.Atoi(c.PostForm("userId"))
	user.ID = uint(id)
	fmt.Println(user.ID)
	targetId, _ := strconv.Atoi(c.PostForm("targetId"))
	page, _ := strconv.Atoi(c.PostForm("page"))
	pageSize, _ := strconv.Atoi(c.PostForm("pageSize"))
	messages, _, err := models.GetChatHistory(user.ID, targetId, page, pageSize)
	fmt.Println(messages)
	if err != nil {
		common.Fail(c, "获取失败")
		return
	}
	common.Success(c, "获取成功", messages)
}

// MarkMessagesRead 标记消息为已读（打开聊天窗口时调用）
// POST /user/MarkRead
func MarkMessagesRead(c *gin.Context) {
	userId := c.PostForm("userId")
	targetId := c.PostForm("targetId")

	// 更新数据库
	models.DB.Model(&models.Message{}).
		Where("target_id = ? AND form_id = ? AND is_offline = ?", userId, targetId, true).
		Update("is_offline", false)

	// 清理 Redis 离线缓存
	ctx := context.Background()
	uid, _ := strconv.ParseInt(userId, 10, 64)
	models.Redis.Del(ctx, models.OfflineMessageKey(uid))

	common.Success(c, "标记成功", nil)
}
