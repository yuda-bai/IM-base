package common

import "time"

// HistoryTTL  聊天记录的保存时长
const HistoryTTL = 7 * 24 * time.Hour

// MaxCacheMessages 每个会话redis缓存上限
const MaxCacheMessages = 500
