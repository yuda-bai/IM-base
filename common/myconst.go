package common

import "time"

// HistoryTTL  聊天记录的保存时长
const HistoryTTL = 7 * 24 * time.Hour

// MaxCacheMessages 每个会话redis缓存上限
const MaxCacheMessages = 500

// PublishKey Redis Pub/Sub 频道名，用于 WebSocket 消息广播
const PublishKey = "websocket"
