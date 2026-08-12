package common

import "time"

// HistoryTTL  聊天记录的保存时长
const HistoryTTL = 7 * 24 * time.Hour

// FriendRequestRejectionCooldown 好友申请被拒绝后的再次申请冷却时间
const FriendRequestRejectionCooldown = 24 * time.Hour

// MaxCacheMessages 每个会话redis缓存上限
const MaxCacheMessages = 500

// PublishKey Redis Pub/Sub 频道名，用于 WebSocket 消息广播
const PublishKey = "websocket"

const (
	FriendRequestPending   = "pending"
	FriendRequestAccepted  = "accepted"
	FriendRequestRejected  = "rejected"
	FriendRequestCancelled = "cancelled"
	// FriendRequestCanceled 保留旧拼写，避免迁移期调用方编译失败。
	FriendRequestCanceled = FriendRequestCancelled
	FriendRequestExpired  = "expired"
)

const (
	FriendRequestValidity     = 7 * 24 * time.Hour
	NotificationFriendRequest = "friend_request"
)
