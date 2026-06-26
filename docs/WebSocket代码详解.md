# WebSocket 代码详解

> 文件：`service/user_service.go`  
> 架构：`浏览器 ↔ WebSocket ↔ Gin 服务 ↔ Redis Pub/Sub`

---

## 整体架构

```
浏览器客户端 A                    浏览器客户端 B
     │                               │
     │  WebSocket                    │  WebSocket
     ▼                               ▼
┌──────────┐                   ┌──────────┐
│ SendMsg  │                   │ SendMsg  │
│ (upgrade)│                   │ (upgrade)│
└────┬─────┘                   └────┬─────┘
     │                              │
     ▼                              ▼
┌──────────────────────────────────────────┐
│              Redis Pub/Sub               │
│        频道: utils.PublishKey            │
│                                          │
│  客户端A发布消息 ──► Redis ◄── 推送给客户端B  │
└──────────────────────────────────────────┘
```

---

## 第 1 部分：Upgrader 定义

```go
var upGrade = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return false
    },
}
```

### 什么是 Upgrader？

`websocket.Upgrader` 负责将 HTTP 连接升级为 WebSocket 连接，内部完成 RFC 6455 规定的握手协议：

```
客户端发来:
┌─────────────────────────────────────────────┐
│ GET /ws HTTP/1.1                            │
│ Upgrade: websocket                          │
│ Connection: Upgrade                         │
│ Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ== │  ← 随机 base64
│ Origin: http://localhost:3000               │
└─────────────────────────────────────────────┘

服务端计算并返回:
┌─────────────────────────────────────────────┐
│ HTTP/1.1 101 Switching Protocols            │
│ Upgrade: websocket                          │
│ Connection: Upgrade                         │
│ Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYG...   │  ← SHA1(Key+GUID)
└─────────────────────────────────────────────┘
```

> 服务端把客户端发来的 `Sec-WebSocket-Key` 拼接固定 GUID `258EAFA5-E914-47DA-95CA-C5AB0DC85B11`，做 SHA-1 哈希，再 Base64 编码，得到 `Sec-WebSocket-Accept`。客户端验证通过则握手完成。

### CheckOrigin 是什么？

`CheckOrigin` 检查发起 WebSocket 请求的页面来源（`Origin` 头）是否合法：

| 写法 | 效果 | 风险 |
|------|------|------|
| `return true` | 任何网站都能连 | 跨站 WebSocket 劫持：`evil.com` 页面可以收发你的消息 |
| `return false` | 拒绝所有跨域 | 连你自己的前端都连不上 |
| 白名单校验 | 只允许指定域名 | 推荐方式 |

**正确写法：**

```go
CheckOrigin: func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    allowed := map[string]bool{
        "http://localhost:3000":        true,  // 本地开发
        "https://你的域名.com":          true,  // 线上
    }
    return allowed[origin]
},
```

---

## 第 2 部分：SendMsg — 升级握手

```go
func SendMsg(c *gin.Context) {
    // ① 将 HTTP 连接升级为 WebSocket
    ws, err := upGrade.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        fmt.Println(err)
        return   // 升级失败，HTTP 请求结束
    }

    // ② 函数退出时关闭 WebSocket 连接
    defer func(ws *websocket.Conn) {
        err = ws.Close()
        if err != nil {
            fmt.Println(err)
        }
    }(ws)

    // ③ 进入消息处理循环（阻塞在这里直到客户端断开）
    MsgHandler(ws, c)
}
```

### 调用链

```
HTTP GET /ws  ──►  Gin 路由  ──►  SendMsg  ──►  Upgrade 升级协议
                                                    │
                                                    ▼
                                              MsgHandler (循环处理)
                                                    │
                                     (客户端断开后退出)
                                                    │
                                                    ▼
                                         defer: ws.Close()
```

> `ws` 是 `*websocket.Conn`，之后所有读写操作都围绕这个对象。

---

## 第 3 部分：MsgHandler — 核心消息循环

### 3.1 订阅 Redis 频道

```go
sub := models.Redis.Subscribe(c, utils.PublishKey)
defer sub.Close()                    // 函数退出时取消订阅
fmt.Println("订阅成功")

redisMsgCh := sub.Channel()          // 返回 <-chan *Message，用于接收消息
```

```
┌─────────────┐         ┌──────────────┐
│ 本连接所在   │  订阅    │    Redis     │
│ 的 MsgHandler│ ◄────── │ PublishKey   │
│             │  Channel │   频道       │
└─────────────┘         └──────────────┘
                              ▲
                     Publish  │
                   ┌──────────┴──────────┐
                   │ 其他客户端发来的消息   │
                   └─────────────────────┘
```

> **注意：** Redis Pub/Sub 是即发即忘的，不会缓存历史消息。客户端不在线期间的消息会丢失。

### 3.2 启动读消息 goroutine

```go
wsMsgCh := make(chan []byte)
go func() {
    for {
        _, msg, err := ws.ReadMessage()   // ① 阻塞等待客户端发消息
        if err != nil {                    // ② 出错 = 客户端断开
            wsMsgCh <- nil                 // ③ 发送 nil 作为"断开信号"
            return                         // ④ goroutine 退出
        }
        wsMsgCh <- msg                     // ⑤ 把消息投递到 channel
    }
}()
```

#### 为什么放在单独的 goroutine？

```
ws.ReadMessage() 是阻塞的——
如果放在主循环里，主循环就卡在等待客户端消息上，
没法同时处理 Redis 推送过来的消息。

并发模型：
┌──────────────────────────────────────┐
│ goroutine 1:  阻塞读 WebSocket       │
│               读到消息 → wsMsgCh      │
│                                     │
│ goroutine 2 (主):  select 多路复用   │
│                   ├─ wsMsgCh         │
│                   └─ redisMsgCh      │
│                                     │
│ goroutine 3 (Redis driver):          │
│                   维护心跳/重连       │
└──────────────────────────────────────┘
```

### 3.3 主事件循环 — select 多路复用

```
        ┌─────────────────────────┐
        │     for { select {} }   │
        │                         │
        │  ┌──── client 消息  ←─ wsMsgCh (goroutine 写入)
        │  │                      │
        │  ├──── Redis  消息  ←─ redisMsgCh (Redis 推送)
        │  │                      │
        │  └──── 两个 channel     │
        │        谁先到就处理谁    │
        └─────────────────────────┘
```

#### case 1：客户端发来消息

```go
case msg := <-wsMsgCh:
    if msg == nil {                          // goroutine 发来的断开信号
        fmt.Println("客户端断开连接")
        return                              // 退出 MsgHandler → defer 执行 Close()
    }
    // 把客户端的消息发布到 Redis 频道
    err := utils.Publish(c, utils.PublishKey, string(msg))
    if err != nil {
        fmt.Println("发布消息失败:", err)
    }
```

数据流：

```
客户端 A "你好"
    │
    ▼
ws.ReadMessage() → wsMsgCh → select 命中
    │
    ▼
Redis.Publish("PublishKey", "你好")
    │
    ▼
Redis 广播给所有订阅了 "PublishKey" 的客户端
    │
    ├──► 客户端 B 的 redisMsgCh 收到 "你好"
    ├──► 客户端 C 的 redisMsgCh 收到 "你好"
    └──► 客户端 A 自己的 redisMsgCh 也收到 "你好"  ← 自己发的也能收到
```

#### case 2：Redis 推来消息

```go
case redisMsg := <-redisMsgCh:
    tm := time.Now().Format("2006-01-02 15:04:05")   // 格式化时间戳
    m := fmt.Sprintf("[ws][%s]:%s", tm, redisMsg.Payload)  // 组装消息
    fmt.Println("收到Redis消息:", m)
    err := ws.WriteMessage(1, []byte(m))             // 写回给当前客户端
```

#### WriteMessage 参数说明

`WriteMessage(messageType, data)` 第一个参数是帧类型：

| 值 | 常量 | 含义 |
|----|------|------|
| 1 | `websocket.TextMessage` | UTF-8 文本帧 |
| 2 | `websocket.BinaryMessage` | 二进制帧 |
| 8 | `websocket.CloseMessage` | 关闭连接 |
| 9 | `websocket.PingMessage` | 心跳探测 |
| 10 | `websocket.PongMessage` | 心跳应答 |

---

## 完整消息流时序图

```
时间 ──────────────────────────────────────────────────────►

客户端A                     Go服务端                    Redis
   │                          │                          │
   │──── WebSocket 连接 ─────►│                          │
   │                          │──── Subscribe ──────────►│
   │                          │                          │
   │──── "你好" ─────────────►│                          │
   │                          │──── Publish("你好") ────►│
   │                          │                          │
   │                          │◄─── 广播 "你好" ────────│
   │◄─── "[ws][16:00:01]:你好" ──│                          │
   │                          │                          │
   │              (消息同时推送给所有订阅的客户端)          │
   │                          │                          │
   │                          │◄─── B 发的消息 ─────────│
   │◄─── "[ws][16:00:02]:Hi" ──│                          │
   │                          │                          │
   │──── 断开连接 ────────────►│                          │
   │                          │──── Unsubscribe ────────►│
   │                          │                          │
   │              goroutine 读到 err → wsMsgCh ← nil     │
   │              select 命中 → return                   │
   │              defer ws.Close()                       │
   │              defer sub.Close()                      │
```

---

## 当前代码存在的问题

### 1. CheckOrigin 返回 false 会拒绝所有连接

```go
// ❌ 当前
return false  // 任何网站都被拒绝，包括你自己的前端

// ✅ 开发阶段
return true   // 先放开，上线后再限制

// ✅ 生产阶段
origin := r.Header.Get("Origin")
return origin == "https://你的域名.com"
```

### 2. 并发写没有保护

```go
// gorilla/websocket 明确说明：不支持并发写
// 问题场景：
// - goroutine: ws.ReadMessage()  ← 安全，只有一个 goroutine 读
// - 主循环: ws.WriteMessage()    ← 只有一个，但 MsgHandler 内可能多次调用

// ✅ 修复方案：加写锁
var writeMu sync.Mutex

writeMu.Lock()
err := ws.WriteMessage(1, data)
writeMu.Unlock()
```

### 3. 缺少心跳检测

```go
// ❌ 当前：客户端网线拔掉后，ReadMessage 可能永不返回错误
//         导致 goroutine 泄漏，连接永远不释放

// ✅ 加上读写超时和 Pong 处理器：
ws.SetReadDeadline(time.Now().Add(60 * time.Second))
ws.SetPongHandler(func(string) error {
    ws.SetReadDeadline(time.Now().Add(60 * time.Second))
    return nil
})

// 每 50 秒发一次 Ping
ticker := time.NewTicker(50 * time.Second)
go func() {
    for range ticker.C {
        ws.WriteMessage(websocket.PingMessage, nil)
    }
}()
```

### 4. 没有发送关闭帧

```go
// ❌ 当前：客户端断开时直接 return，没有通知对端

// ✅ 优雅关闭：
ws.WriteMessage(websocket.CloseMessage,
    websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
```

### 5. ReadMessage 的 messageType 被丢弃

```go
// ❌ 当前
_, msg, err := ws.ReadMessage()  // 忽略了消息类型

// ✅ 如果客户端发的是 BinaryMessage，回显时应用同样类型
msgType, msg, err := ws.ReadMessage()
// ... 处理 ...
ws.WriteMessage(msgType, response)
```

---

## 核心知识点总结

| 概念 | 说明 |
|------|------|
| **WebSocket 握手** | HTTP Upgrade → 101 Switching Protocols → 协议切换完成 |
| **Upgrader** | gorilla/websocket 提供的握手器，处理协议升级 |
| **CheckOrigin** | 防止跨站 WebSocket 劫持，应校验 Origin 白名单 |
| **ReadMessage** | 阻塞读取，单独放 goroutine 以免卡住主循环 |
| **channel + select** | Go 的 CSP 并发模型，同时监听多个数据源 |
| **Redis Pub/Sub** | 消息广播中间件，跨进程分发实时消息 |
| **nil 作为哨兵值** | goroutine 向 channel 发 nil 表示客户端断开 |
| **Ping/Pong** | WebSocket 协议层心跳，检测死连接 |
| **defer 链** | `defer ws.Close()` + `defer sub.Close()` 保证资源释放 |

---

## 一句话总结

> 这份代码实现了一个 **WebSocket ↔ Redis Pub/Sub 桥接层**：
> 客户端发消息 → 发布到 Redis → Redis 广播 → 推给所有订阅客户端。
> 核心技巧是用 **goroutine + channel + select** 把"阻塞读 WebSocket"和"接收 Redis 推送"两个异步事件统一到一个事件循环里。
