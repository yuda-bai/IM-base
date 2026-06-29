package models

import (
	"context"
	"fmt"
	"ginchat/common"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
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
	UserId    int64 // 用于 recvProc/sendProc 退出时按值精确清理 clientMap
}

// 映射关系
var clientMap = make(map[int64]*Node)

// 读写锁
var rwLocker sync.RWMutex

// SaveMessage 消息持久化 保存在数据库,redis缓存
func SaveMessage(message *Message) error {
	//1.写入数据库
	if err := DB.Save(message).Error; err != nil {
		return err
	}
	//2.序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return err
	}
	ctx := context.Background()
	//3.写入聊天历史缓存
	key := chatHistoryKey(message.FormId, message.TargetId)
	Redis.ZAdd(ctx, key, redis.Z{
		Score:  float64(message.ID),
		Member: string(msgJSON),
	})
	Redis.Expire(ctx, key, common.HistoryTTL)
	//裁剪超过500条的最早消息
	count, _ := Redis.ZCard(ctx, key).Result()
	if count > common.MaxCacheMessages {
		Redis.ZRemRangeByRank(ctx, key, 0, count-common.MaxCacheMessages-1)
	}
	//4.写入离线消息缓存
	offlineKey := offlineMessageKey(message.TargetId)
	Redis.ZAdd(ctx, offlineKey, redis.Z{Score: float64(message.ID), Member: string(msgJSON)})
	Redis.Expire(ctx, offlineKey, common.HistoryTTL)
	return nil
}

// removeFromClientMap 安全地从 clientMap 中移除指定 userId 的连接（仅在锁外调用）
func removeFromClientMap(userId int64, reason string) {
	rwLocker.Lock()
	defer rwLocker.Unlock()
	if _, exists := clientMap[userId]; exists {
		delete(clientMap, userId)
		fmt.Printf("🗑 用户%d已从clientMap移除(原因:%s),当前在线:%d人\n", userId, reason, len(clientMap))
	}
}

// 生成聊天记录redis key,保证同一对用户生成唯一key
func chatHistoryKey(userId, targetId int64) string {
	ids := []int64{userId, targetId}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return fmt.Sprintf("chat:%d:%d", ids[0], ids[1])
}

// 生成离线消息redis key
func offlineMessageKey(userId int64) string {
	return fmt.Sprintf("offline:%d", userId)
}

func Chat(writer http.ResponseWriter, request *http.Request) {
	//校验合法性
	query := request.URL.Query()
	Id := query.Get("userId")
	userId, _ := strconv.ParseInt(Id, 10, 64)
	isvalida := true //todo 校验token
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
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
		UserId:    userId,
	}
	//3.用户关系
	//4.userid 与 node绑定并加锁
	rwLocker.Lock()
	if oldNode, exists := clientMap[userId]; exists {
		fmt.Printf("⚠️ 用户%d重新连接,关闭旧链接并替换为新连接\n", userId)
		oldNode.Conn.Close()
		close(oldNode.DataQueue)
	}
	clientMap[userId] = node
	rwLocker.Unlock()
	fmt.Printf("✅ 用户%d已注册到clientMap,当前在线:%d人\n", userId, len(clientMap))
	//5.完成发送逻辑
	go sendProc(node)
	//6.完成接收逻辑
	go recvProc(node)
	welcomeData, _ := json.Marshal(map[string]interface{}{
		"userId":    0,
		"userName":  "系统",
		"content":   "欢迎来到聊天室",
		"type":      "system",
		"messageId": fmt.Sprintf("welcome-%d", userId),
	})
	sendMsg(userId, welcomeData)
	//7.发送离线消息
	go sendOfflineMessages(userId)
}

// 发送消息到客户端
func sendProc(node *Node) {
	defer removeFromClientMap(node.UserId, "sendProc退出")
	for data := range node.DataQueue {
		err := node.Conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			fmt.Printf("发送消息错误(连接可能已关闭): %v\n", err)
			return
		}
		fmt.Printf("[ws] <<<发送消息成功 userId=%d: %s\n", node.UserId, string(data))
	}
	fmt.Printf("sendProc 退出:DataQueue已关闭 userId=%d\n", node.UserId)
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
	fmt.Printf("发送离线消息一共%d给用户%d\n", len(messages), userId)
	for _, message := range messages {
		msgData := map[string]interface{}{
			"userId":    message.FormId,
			"targetId":  message.TargetId,
			"type":      message.Type,
			"userName":  "用户",
			"content":   message.Content,
			"pic":       message.Pic,
			"url":       message.Url,
			"media":     message.Media,
			"messageId": fmt.Sprintf("%d", message.ID),
			"timestamp": message.CreatedAt.Format("2006-01-02 15:04:05"),
			"isOffline": true,
		}
		jsonData, err := json.Marshal(msgData)
		if err != nil {
			fmt.Println("离线消息json解析错误:", err)
			continue
		}
		sendMsg(userId, jsonData)
	}
	fmt.Printf("已发送 %d 条离线消息给用户 %d\n", len(messages), userId)
}

// 接收客户端消息
func recvProc(node *Node) {
	defer func() {
		// ★ 关键：连接断开时清理 clientMap，使用 UserId 而不是指针比较
		removeFromClientMap(node.UserId, "recvProc连接断开")
	}()
	for {
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			fmt.Printf("接收消息错误 userId=%d: %v\n", node.UserId, err)
			return // defer 会自动清理
		}
		broadMsg(data)
		Dispatch(data)
		fmt.Printf("recvProc<<< [ws] userId=%d 接收消息成功: %s\n", node.UserId, string(data))
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
	defer conn.Close()
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

// 完成udp数据接收协程
func udpRecvProc() {
	con, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 8081,
	})
	if err != nil {
		fmt.Println("udp监听错误", err)
		return
	}
	defer con.Close()
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
		fmt.Println("Dispatch json解析错误:", err)
		return
	}
	//保存消息到数据库
	if msg.Type == 1 || msg.Type == 3 || msg.Type == 2 {
		if err := SaveMessage(&msg); err != nil {
			fmt.Println("保存消息失败:", err, "原始数据:", string(data))
			return
		}
		fmt.Printf("保存消息成功: ID=%d FormId=%d TargetId=%d Type=%d Content=%s\n",
			msg.ID, msg.FormId, msg.TargetId, msg.Type, msg.Content)
	}
	switch msg.Type {
	case 1:
		//单聊
		sendMsg(msg.TargetId, data)
		sendMsg(msg.FormId, data) //发送端接收回显
	}
}

// 发送消息到指定用户的 DataQueue
func sendMsg(userID int64, msg []byte) {
	rwLocker.RLock()
	node, ok := clientMap[userID]
	rwLocker.RUnlock()
	if !ok {
		fmt.Printf("⚠️ sendMsg失败: 用户%d不在clientMap中(幽灵连接或未上线)\n", userID)
		return
	}
	select {
	case node.DataQueue <- msg:
		fmt.Printf("sendMsg [ws] >>>发送到用户%d成功: %s\n", userID, string(msg))
	default:
		fmt.Printf("⚠️ 消息队列已满 userId=%d\n", userID)
	}
}

// 获取用户的离线消息
func getOfflineMessages(userId int64, msgType int) (messages []Message, err error) {
	err = DB.Where("target_id = ? AND type = ? AND created_at > ?", userId, msgType, time.Now().Add(-24*7*time.Hour)).
		Order("created_at ASC").
		Find(&messages).Error
	fmt.Println("获取离线消息成功", messages)
	return
}

// GetChatHistory 获取与某个用户的聊天记录
func GetChatHistory(userId uint, targetId int, page int, pageSize int) ([]Message, int64, error) {
	ctx := context.Background()
	key := chatHistoryKey(int64(userId), int64(targetId))
	//1.先查询Redis总条数
	total, err := Redis.ZCard(ctx, key).Result()
	if err != nil || total == 0 {
		return GetChatHistoryFromDB(userId, targetId, page, pageSize)
	}
	//2.redis命中,分页读取
	start := int64(pageSize * (page - 1))
	stop := start + int64(pageSize) - 1
	members, err := Redis.ZRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return GetChatHistoryFromDB(userId, targetId, page, pageSize)
	}
	//3.反序列化
	messages := make([]Message, 0, len(members))
	results, err := Redis.ZRevRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		// 处理错误
	}
	for _, z := range results {
		// 从 redis.Z 取出 Member 并断言为 string
		memberStr, ok := z.Member.(string)
		if !ok {
			continue
		}
		var msg Message
		if err := json.Unmarshal([]byte(memberStr), &msg); err == nil {
			messages = append(messages, msg)
		}

	}
	//4.反转数组,显示顺序为正序

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, total, nil
}
func GetChatHistoryFromDB(userId uint, targetId int, page int, pageSize int) ([]Message, int64, error) {
	uid := int64(userId)
	tid := int64(targetId)
	// 1. 查询总数（修复：双向查询）
	var total int64
	DB.Model(&Message{}).
		Where("(form_id = ? AND target_id = ?) OR (form_id = ? AND target_id = ?)", uid, tid, tid, uid).
		Count(&total)

	// 2. 分页查询
	var messages []Message
	DB.Where("(form_id = ? AND target_id = ?) OR (form_id = ? AND target_id = ?)", uid, tid, tid, uid).
		Order("created_at DESC").
		Offset(pageSize * (page - 1)).
		Limit(pageSize).
		Find(&messages)

	// 3. 反转数组为正序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// 4. 异步回写 Redis（不阻塞返回）
	go func() {
		ctx := context.Background()
		key := chatHistoryKey(uid, tid)
		for _, msg := range messages {
			msgJSON, _ := json.Marshal(msg)
			Redis.ZAdd(ctx, key, redis.Z{
				Score:  float64(msg.ID),
				Member: string(msgJSON),
			})
		}
		Redis.Expire(ctx, key, common.HistoryTTL)
	}()

	return messages, total, nil
}
