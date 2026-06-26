package models

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type Message struct {
	gorm.Model
	FormId   int64  //发送者id
	TargetId int64  //接收者id
	Type     int    //消息类型 1私聊 2群聊 3广播
	Media    string //消息媒体类型 1 文本 2表情包 3 图片 4 音频
	Content  string //消息内容
	Pic      string //图片
	Url      string //链接
	Desc     string //描述
	Amount   int    //其他数字统计
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
}
func sendProc(node *Node) {
	for {
		select {
		case data := <-node.DataQueue:
			err := node.Conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				fmt.Println("发送消息错误", err)
				return
			}
		}
	}
}
func recvProc(node *Node) {
	for {
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			fmt.Println("接收消息错误", err)
			return
		}
		broadMsg(data)
		fmt.Println("[ws] 接收消息成功", data)
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

// 后端调度逻辑处理
func dispatch(data []byte) {
	msg := Message{}
	err := json.Unmarshal(data, &msg)
	if err != nil {
		fmt.Println("json解析错误", err)
		return
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
func sendMsg(userID int64, msg []byte) {
	rwLocker.RLock()
	node, ok := clientMap[userID]
	rwLocker.RUnlock()
	if !ok {
		fmt.Println("用户不存在")
		return
	}
	node.DataQueue <- msg
}
