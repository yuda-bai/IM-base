# GinChat — 高性能即时通讯（IM）系统

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue)](https://go.dev/)
[![Vue Version](https://img.shields.io/badge/Vue-3.4-brightgreen)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

基于 **Go (Gin) + Vue 3 + MySQL + Redis + WebSocket** 的单体 IM 即时通讯系统。支持私聊实时消息、离线消息、图片/语音发送、联系人管理。

---

## 功能特性

- **实时私聊** — WebSocket 长连接，消息实时送达
- **离线消息** — 对方不在线时消息存入 Redis，上线后自动推送
- **图片/语音** — 支持图片和语音上传发送
- **好友管理** — 搜索用户、添加好友
- **聊天记录** — 分页查询历史消息，Redis 缓存热数据
- **Swagger 文档** — 内置 API 文档，开发调试方便
- **Docker 部署** — 多阶段构建，compose 一键编排

---

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 后端框架 | Go 1.25 + Gin | REST API + WebSocket |
| 前端 | Vue 3 + Vite + Element Plus | SPA 单页应用 |
| 数据库 | MySQL 8.0 + GORM | 用户/好友/消息持久化 |
| 缓存 | Redis 7 | 消息缓存 + Pub/Sub + 离线消息 |
| 文档 | Swagger (swaggo) | 自动生成 API 文档 |
| 部署 | Docker Compose + Nginx | 容器化多服务编排 |

---

## 快速开始

### 前置条件

- Go ≥ 1.25
- Node.js ≥ 18
- MySQL 8.0
- Redis 7

### 1. 克隆项目

```bash
git clone https://github.com/yuda-bai/IM-base.git
cd IM-base
```

### 2. 配置

编辑 `config/app.yml`：

```yaml
mysql:
  dns: "root:YOUR_PASSWORD@tcp(127.0.0.1:3306)/ginchat?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  addr: "127.0.0.1:6379"
  password: "YOUR_PASSWORD"
  db: 0
  pool_size: 30
  min_idle_conns: 10
```

### 3. 启动后端

```bash
# 创建数据库
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS ginchat DEFAULT CHARSET utf8mb4"

# 启动（自动创建表）
go run main.go
# 访问 http://localhost:8080/index → {"message":"pong"}
```

### 4. 启动前端

```bash
cd frontend
npm install
npm run dev
# 访问 http://localhost:10086
```

### 5. Docker 一键部署

```bash
docker compose up -d --build
```

---

## 项目结构

```
ginchat/
├── main.go                  # 入口：初始化配置 → DB → Redis → 路由
├── config/
│   ├── app.yml              # 开发环境配置
│   └── app.docker.yml       # Docker 环境配置
├── models/                  # 数据层
│   ├── user_basic.go        # 用户模型 & CRUD
│   ├── contact.go           # 好友关系
│   ├── message.go           # 消息模型 & WebSocket 核心
│   └── group.go             # 群组模型
├── service/                 # 业务层
│   ├── user_service.go      # 登录/注册/增删好友/消息
│   ├── upload.go            # 图片/语音上传
│   └── index.go             # 健康检查
├── router/
│   └── router.go            # Gin 路由注册
├── utils/
│   └── system_init.go       # Viper + GORM + Redis 初始化
├── common/                  # 公共工具（响应格式、常量）
├── docs/                    # Swagger 文档
├── frontend/                # Vue 3 前端
│   └── src/
│       ├── views/           # Login / Register / ChatRoom
│       ├── components/      # ChatArea / Sidebar / UserProfile
│       ├── stores/          # Pinia 状态管理
│       ├── api/             # Axios 请求封装
│       └── utils/           # WebSocket 客户端
├── Dockerfile               # 多阶段 Go 构建
├── docker-compose.yml       # MySQL + Redis + App
└── deploy.sh                # 一键编译部署脚本
```

---

## API 速览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/index` | 健康检查 |
| POST | `/user/FindUserByNameAndPassword` | 登录 |
| GET | `/user/CreateUser` | 注册 |
| GET | `/user/GetUserList` | 用户列表 |
| GET | `/user/SearchFriend` | 好友列表 |
| POST | `/user/AddFriend` | 添加好友 |
| GET | `/user/GetChatRecord` | 分页聊天记录 |
| GET | `/user/SendUserMsg` | WebSocket 连接端点 |
| POST | `/user/SendMessageHttp` | HTTP 发送消息 |
| POST | `/user/UploadImage` | 图片上传 |
| POST | `/user/UploadAudio` | 语音上传 |
| GET | `/swagger/*any` | Swagger 文档 |

---

## WebSocket 通信协议

### 连接

```
ws://host/user/SendUserMsg?userId=2
```

### 发送消息

```json
{
  "userId": 2,
  "userName": "张三",
  "content": "你好",
  "type": 1,
  "media": "1",
  "targetId": 3
}
```

| 字段 | 值 | 说明 |
|------|------|------|
| Type | 1 | 私聊 |
| Type | 2 | 群聊 |
| Media | 1 | 文本 |
| Media | 3 | 图片 |
| Media | 4 | 语音 |

---

## 部署

### 生产部署

```bash
# 本地编译 Linux 二进制
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ginchat_server .

# 前端构建
cd frontend && npm run build

# 上传部署（详见 docs/部署上线指南.md）
```

### 版本管理

```bash
bash deploy.sh              # 一键构建部署
bash deploy.sh list         # 列出所有版本
bash deploy.sh status       # 当前运行状态
bash deploy.sh rollback v1  # 回滚到指定版本
```

---

## 架构图

```
┌─────────────────────────────────────┐
│           用户浏览器 (Vue 3)          │
│      http://your-domain.com         │
└────────────┬────────────────────────┘
             │  HTTP / WebSocket
             ▼
┌─────────────────────────────────────┐
│          Nginx :80/:443              │
│  ┌──── frontend/dist ────────────┐  │
│  │ /user/*  → proxy_pass :8080   │  │
│  │ WebSocket Upgrade              │  │
│  └────────────────────────────────┘  │
└────────────┬────────────────────────┘
             │
             ▼
┌─────────────────────────────────────┐
│         Go (Gin) :8080              │
│  ┌─ REST API ──┬── WebSocket ────┐  │
│  │  GORM ORM   │  gorilla/ws     │  │
│  └──────┬──────┴──────┬──────────┘  │
└─────────┼─────────────┼─────────────┘
          │             │
          ▼             ▼
     ┌────────┐   ┌──────────┐
     │ MySQL  │   │  Redis   │
     │ :3306  │   │  :6379   │
     └────────┘   └──────────┘
```

---

## 待办 (Roadmap)

- [ ] JWT Token 认证
- [ ] 群聊功能
- [ ] 消息已读回执
- [ ] 端到端加密
- [ ] 消息撤回
- [ ] 单元测试覆盖
- [ ] CI/CD (GitHub Actions)

---

## 许可证

MIT License — 详见 [LICENSE](LICENSE)
