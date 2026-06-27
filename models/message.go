package models

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type Message struct {
	gorm.Model
	FormId    int64  //发送者id
	TargetId  int64  //接收者id
	Type      int    //消息类型 1私聊  2群聊 3广播
	Media     string //消息媒体类型 1 文本 2表情包 3 图片 4 音频
	Content   string //消息内容
	Pic       string //图片
	Url       string //链接
	Desc      string //描述
	IsOffline bool   //是否离线消息
	Amount    int    //其他数字统计
}

func (table *Message) TableName() string {
	return "messages"
}

type Node struct {
	Conn      *websocket.Conn
	DataQueue chan []byte
	GroupSets map[string]interface{}
}

// 映射关系
var clientMap map[int64]*Node = make(map[int64]*Node)

// 读写锁
var rwLocker sync.RWMutex

// SaveMessage 消息持久化 保存在数据库
func SaveMessage(message Message) error {
	return DB.Create(message).Error
}
func Chat(writer http.ResponseWriter, request *http.Request) {
	//校验合法性
	query := request.URL.Query()
	Id := query.Get("userId")
	userId, _ := strconv.ParseInt(Id, 10, 64)
	//token := query.Get("token")
	//targetId := query.Get("targetId")
	//context := query.Get("context")
	//msgType := query.Get("type")
	isvalida := true //todo 校验token
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			//token校验
			return isvalida
		},
	}).Upgrade(writer, request, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	//2.获取conn
	node := &Node{
		Conn:      conn,
		DataQueue: make(chan []byte, 50),
		GroupSets: make(map[string]interface{}),
	}
	//3.用户关系
	//4.userid 与 node绑定并加锁
	rwLocker.Lock()
	clientMap[userId] = node
	rwLocker.Unlock()
	//5.完成发送逻辑
	go sendProc(node)
	//6.完成接收逻辑
	go recvProc(node)
	sendMsg(userId, []byte("欢迎来到聊天室"))
	//7.发送离线消息
	go sendOfflineMessages(userId)
}

// 发送消息
func sendProc(node *Node) {
	for {
		select {
		case data := <-node.DataQueue:
			fmt.Printf("sendProc>>>>   [ws] 发送消息成功: %s\n", string(data))
			err := node.Conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				fmt.Println("发送消息错误", err)
				return
			}
		}
	}
}

// 发送离线消息
func sendOfflineMessages(userId int64) {
	messages, err := getOfflineMessages(userId, 1)
	if err != nil {
		fmt.Println("获取离线消息错误", err)
		return
	}
	if len(messages) == 0 {
		return
	}
	fmt.Printf("发送离线消息一共%d给用户%d", len(messages), userId)
	for _, message := range messages {
		msgData := map[string]interface{}{
			"userId":    message.FormId,
			"targetId":  message.TargetId,
			"type":      message.Type,
			"userName":  "用户",
			"content":   message.Content,
			"pic":       message.Pic,
			"url":       message.Url,
			"messageId": fmt.Sprintf("%d", message.ID),
			"timestamp": message.CreatedAt.Format("2006-01-02 15:04:05"),
			"isOffline": true,
		}
		jsonData, err := json.Marshal(msgData)
		if err != nil {
			fmt.Println("json解析错误", err)
			fmt.Println("json解析错误:", err, "原始数据:", jsonData)
			continue
		}
		sendMsg(userId, jsonData)
	}
	fmt.Printf("已发送 %d 条离线消息给用户 %d\n", len(messages), userId)
}

// 接收消息
func recvProc(node *Node) {
	for {
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			fmt.Println("接收消息错误", err)
			return
		}
		broadMsg(data) // ← 直接分发消息，让目标用户接收
		Dispatch(data)
		fmt.Printf("recvProc<<<  [ws] 接收消息成功 %s\n", string(data))
	}
}

var udpsendChan = make(chan []byte, 1024)

func broadMsg(data []byte) {
	udpsendChan <- data
}
func init() {
	go udpSendProc()
	go udpRecvProc()
}

// 完成udp数据发送协程
func udpSendProc() {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 8081,
	})
	if err != nil {
		fmt.Println("udp连接错误", err)
		return
	}
	defer func(conn *net.UDPConn) {
		err := conn.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
	}(conn)
	for {
		select {
		case data := <-udpsendChan:
			_, err := conn.Write(data)
			if err != nil {
				fmt.Println("udp发送错误", err)
				return
			}
		}
	}
}

// 完成udp数据发送协程
func udpRecvProc() {
	con, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 8081,
	})
	if err != nil {
		fmt.Println("udp监听错误", err)
		return
	}
	defer func(con *net.UDPConn) {
		err := con.Close()
		if err != nil {
		}
	}(con)
	for {
		var buf [1024]byte
		n, err := con.Read(buf[0:])
		if err != nil {
			fmt.Println("udp接收错误", err)
			return
		}
		fmt.Println("[udp] 接收数据成功", string(buf[:n]))
	}
}

// Dispatch 后端调度逻辑处理
func Dispatch(data []byte) {
	msg := Message{}
	err := json.Unmarshal(data, &msg)
	if err != nil {
		fmt.Println("Dispatch  json解析错误", err, "原始数据:", msg) //todo 待修改
		return
	}
	//保存消息到数据库
	if msg.Type == 1 || msg.Type == 3 || msg.Type == 2 {
		if err := SaveMessage(msg); err != nil {
			fmt.Println("json解析错误:", err, "原始数据:", string(data))
			return
		}
		fmt.Printf("保存消息成功: %s", msg)
	}
	switch msg.Type {
	case 1:
		//单聊
		sendMsg(msg.TargetId, data)
		//case 2:
		//	//群聊
		//	sendGroupMSg()
		//case 3:
		//	//广播
		//	sendAllMsg()

	}
}

// 发送消息
func sendMsg(userID int64, msg []byte) {
	rwLocker.RLock()
	node, ok := clientMap[userID]
	rwLocker.RUnlock()
	if !ok {
		fmt.Println("用户不存在")
		return
	}
	node.DataQueue <- msg
	fmt.Println("[ws] >>>进入聊天系统")
}

// 获取用户的离线消息
func getOfflineMessages(userId int64, msgType int) (messages []Message, err error) {
	err = DB.Where("target_id = ? AND type = ? AND created_at > ?", userId, msgType, time.Now().Add(-24*7*time.Hour)).
		Order("created_at ASC").
		Find(&messages).Error
	return
}

// GetChatHistory 获取与某个用户的聊天记录
func GetChatHistory(userId uint, targetId int, page int, pageSize int) ([]Message, error) {
	var messages []Message
	err := DB.Where("form_id = ? AND target_id = ?", userId, targetId).
		Order("created_at DESC").
		Offset(pageSize * (page - 1)).
		Limit(pageSize).
		Find(&messages).Error
	//反转数组,显示顺序为正序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, err
}
