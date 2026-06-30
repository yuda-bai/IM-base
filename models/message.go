package models

import (
	"context"
	"fmt"
	"ginchat/common"

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
	UserId    int64     // 用于 recvProc/sendProc 退出时按值精确清理 clientMap
	closeOnce sync.Once // 确保清理逻辑只执行一次，防止 close(DataQueue) panic
}

// Close 安全关闭节点，sync.Once 保证只执行一次
func (n *Node) Close(reason string) {
	n.closeOnce.Do(func() {
		close(n.DataQueue)
		rwLocker.Lock()
		// 指针比较防止误删重连后的新连接
		if existing, ok := clientMap[n.UserId]; ok && existing == n {
			delete(clientMap, n.UserId)
		}
		rwLocker.Unlock()
		fmt.Printf("🗑 用户%d已从clientMap移除(原因:%s),当前在线:%d人\n", n.UserId, reason, len(clientMap))
	})
}

// 映射关系
var clientMap = make(map[int64]*Node)

// 读写锁
var rwLocker sync.RWMutex

// SaveMessage 消息持久化 保存在数据库,redis缓存（使用 Pipeline 减少网络往返）
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
	key := chatHistoryKey(message.FormId, message.TargetId)
	offlineKey := offlineMessageKey(message.TargetId)

	//3.批量写入 Redis + 裁剪（Pipeline 合并 6 条命令为 1 次网络往返）
	pipe := Redis.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(message.ID), Member: string(msgJSON)})
	pipe.Expire(ctx, key, common.HistoryTTL)
	// ZRemRangeByRank: 保留最后 MaxCacheMessages 条，删除更早的（0 到 -(N+1)）
	pipe.ZRemRangeByRank(ctx, key, 0, -(common.MaxCacheMessages + 1))
	pipe.ZAdd(ctx, offlineKey, redis.Z{Score: float64(message.ID), Member: string(msgJSON)})
	pipe.Expire(ctx, offlineKey, common.HistoryTTL)
	pipe.ZRemRangeByRank(ctx, offlineKey, 0, -(common.MaxCacheMessages + 1))
	if _, err := pipe.Exec(ctx); err != nil {
		fmt.Println("Redis Pipeline 写入失败:", err)
	}

	return nil
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
		oldNode.Close("重复登录替换") // sync.Once 安全关闭 DataQueue 并清理 clientMap
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
	defer node.Close("sendProc退出") // sync.Once 保证只清理一次
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
	messages, err := getOfflineMessages(userId)
	if err != nil {
		fmt.Println("获取离线消息错误", err)
		return
	}
	if len(messages) == 0 {
		return
	}
	fmt.Printf("发送离线消息一共%d给用户%d\n", len(messages), userId)
	ids := make([]uint, 0, len(messages))
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
		ids = append(ids, message.ID)
		if err != nil {
			fmt.Println("离线消息json解析错误:", err)
			continue
		}
		sendMsg(userId, jsonData)
	}
	fmt.Printf("已发送 %d 条离线消息给用户 %d\n", len(messages), userId)
}

// 接收客户端消息（含心跳超时检测）
func recvProc(node *Node) {
	defer node.Close("recvProc连接断开") // sync.Once 保证只清理一次（同时关闭 DataQueue 唤醒 sendProc）
	for {
		// 心跳检测：60秒无消息则判定连接僵死
		_ = node.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			fmt.Printf("接收消息错误 userId=%d: %v\n", node.UserId, err)
			return // defer 会自动清理 DataQueue 和 clientMap
		}
		Dispatch(data)
		fmt.Printf("recvProc<<< [ws] userId=%d 接收消息成功: %s\n", node.UserId, string(data))
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
	case 2:
		//群聊：TODO 从群组获取成员列表并广播
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
func getOfflineMessages(userId int64) (messages []Message, err error) {
	ctx := context.Background()
	offlineKey := offlineMessageKey(userId)

	//1.先从Redis中获取
	members, err := Redis.ZRange(ctx, offlineKey, 0, common.MaxCacheMessages-1).Result()
	if err == nil && len(members) > 0 {
		for _, member := range members {
			var msg Message
			if json.Unmarshal([]byte(member), &msg) == nil {
				messages = append(messages, msg)
			}
		}
		//推送后清空Redis
		Redis.Del(ctx, offlineKey)
		return messages, nil
	}
	//2.Redis未命中,从DB中获取（限制条数防止内存爆炸）
	err = DB.Where("target_id = ? AND is_offline = ? AND created_at > ?", userId, true,
		time.Now().Add(-7*24*time.Hour)).
		Limit(common.MaxCacheMessages).
		Find(&messages).Error
	return messages, err
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
	//3.反序列化（members 已是分页后的正序数据，无需全量拉取）
	messages := make([]Message, 0, len(members))
	for _, z := range members {
		memberStr, ok := z.Member.(string)
		if !ok {
			continue
		}
		var msg Message
		if err := json.Unmarshal([]byte(memberStr), &msg); err == nil {
			messages = append(messages, msg)
		}
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

	// 4. 同步回写 Redis（使用 Pipeline 批量写入，避免逐条网络往返）
	if len(messages) > 0 {
		ctx := context.Background()
		key := chatHistoryKey(uid, tid)
		pipe := Redis.Pipeline()
		for _, msg := range messages {
			msgJSON, _ := json.Marshal(msg)
			pipe.ZAdd(ctx, key, redis.Z{
				Score:  float64(msg.ID),
				Member: string(msgJSON),
			})
		}
		pipe.Expire(ctx, key, common.HistoryTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			fmt.Println("Redis Pipeline 回写失败:", err)
		}
	}

	return messages, total, nil
}
