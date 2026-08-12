package models

import (
	"errors"
	"fmt"
	"time"

	"ginchat/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrFriendRequestExpired = errors.New("好友申请已过期")

// FriendRequest 好友申请。冷却期按发送者到接收者的方向计算。
type FriendRequest struct {
	gorm.Model
	FromUserID   uint       `gorm:"not null;index" json:"fromUserId"`
	ToUserID     uint       `gorm:"not null;index" json:"toUserId"`
	PairKey      string     `gorm:"size:64;not null;index" json:"pairKey"`
	PendingKey   *string    `gorm:"size:64;uniqueIndex:uk_friend_request_pending" json:"-"`
	Remark       string     `gorm:"size:100;not null;default:''" json:"remark"`
	Status       string     `gorm:"size:20;not null;default:pending;index" json:"status"`
	RejectReason string     `gorm:"size:100;not null;default:''" json:"rejectReason"`
	RejectedAt   *time.Time `gorm:"index" json:"rejectedAt"`
	ProcessedAt  *time.Time `gorm:"index" json:"processedAt"`
	ExpiresAt    time.Time  `gorm:"not null;index" json:"expiresAt"`
	IsRead       bool       `gorm:"not null;default:false;index" json:"isRead"`
}

func (table *FriendRequest) TableName() string { return "friend_requests" }

// FriendPairKey 生成与方向无关的用户对标识。
func FriendPairKey(userID, targetID uint) string {
	if userID > targetID {
		userID, targetID = targetID, userID
	}
	return fmt.Sprintf("%d:%d", userID, targetID)
}

// RejectionCooldownUntil 返回拒绝冷却结束时间。
func RejectionCooldownUntil(rejectedAt time.Time) time.Time {
	return rejectedAt.Add(common.FriendRequestRejectionCooldown)
}

// IsInRejectionCooldown 判断当前时间是否仍处于冷却期。恰好满 24 小时时允许再次申请。
func IsInRejectionCooldown(rejectedAt, now time.Time) bool {
	return now.Before(RejectionCooldownUntil(rejectedAt))
}

func IsFriendRequestStatus(status string) bool {
	switch status {
	case common.FriendRequestPending, common.FriendRequestAccepted, common.FriendRequestRejected, common.FriendRequestCancelled, common.FriendRequestExpired:
		return true
	default:
		return false
	}
}

// CanTransitionFriendRequest 只允许状态机定义的流转；accepted -> accepted 用于幂等同意。
func CanTransitionFriendRequest(current, target string) bool {
	if !IsFriendRequestStatus(current) || !IsFriendRequestStatus(target) {
		return false
	}
	if current == common.FriendRequestAccepted && target == common.FriendRequestAccepted {
		return true
	}
	if current != common.FriendRequestPending {
		return false
	}
	switch target {
	case common.FriendRequestAccepted, common.FriendRequestRejected, common.FriendRequestCancelled, common.FriendRequestExpired:
		return true
	default:
		return false
	}
}

// FindPendingFriendRequest 查询双方任意方向的待处理申请。
func FindPendingFriendRequest(userID, targetID uint) (*FriendRequest, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}

	var request FriendRequest
	err := DB.Where(
		"status = ? AND ((from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?))",
		common.FriendRequestPending,
		userID,
		targetID,
		targetID,
		userID,
	).Order("created_at DESC").First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &request, err
}

// FindFriendRequests 查询用户收到或发出的申请，并惰性处理过期记录。
func FindFriendRequests(userID uint, received bool) ([]FriendRequest, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	now := time.Now()
	if err := ExpirePendingFriendRequests(DB, userID, now); err != nil {
		return nil, err
	}
	var query *gorm.DB
	if received {
		query = DB.Where("to_user_id = ?", userID)
	} else {
		query = DB.Where("from_user_id = ?", userID)
	}
	var requests []FriendRequest
	err := query.Order("created_at DESC").Find(&requests).Error
	return requests, err
}

func FindFriendRequestByID(requestID uint) (*FriendRequest, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	var request FriendRequest
	err := DB.First(&request, requestID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &request, err
}

func MarkFriendRequestsRead(userID uint, requestIDs []uint) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	query := DB.Model(&FriendRequest{}).Where("to_user_id = ?", userID)
	if len(requestIDs) > 0 {
		query = query.Where("id IN ?", requestIDs)
	}
	return query.Update("is_read", true).Error
}

// FindLatestRejectedFriendRequest 查询同方向最近一次被拒绝的申请。
func FindLatestRejectedFriendRequest(fromUserID, toUserID uint) (*FriendRequest, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}

	var request FriendRequest
	err := DB.Where(
		"from_user_id = ? AND to_user_id = ? AND status = ? AND rejected_at IS NOT NULL",
		fromUserID,
		toUserID,
		common.FriendRequestRejected,
	).Order("rejected_at DESC").First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &request, err
}

// FriendRequestRejectionCooldown 查询发送者是否仍处于拒绝冷却期。
func FriendRequestRejectionCooldown(fromUserID, toUserID uint, now time.Time) (bool, time.Time, error) {
	request, err := FindLatestRejectedFriendRequest(fromUserID, toUserID)
	if err != nil || request == nil || request.RejectedAt == nil {
		return false, time.Time{}, err
	}

	until := RejectionCooldownUntil(*request.RejectedAt)
	return now.Before(until), until, nil
}

// CreateFriendRequest 创建待处理好友申请。
func CreateFriendRequest(fromUserID, toUserID uint, remark string) (*FriendRequest, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}

	pairKey := FriendPairKey(fromUserID, toUserID)
	request := &FriendRequest{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		PairKey:    pairKey,
		PendingKey: &pairKey,
		Remark:     truncateText(remark, 100),
		Status:     common.FriendRequestPending,
		ExpiresAt:  time.Now().Add(common.FriendRequestValidity),
	}
	if err := DB.Create(request).Error; err != nil {
		return nil, err
	}
	return request, nil
}

func truncateText(value string, max int) string {
	if len([]rune(value)) <= max {
		return value
	}
	return string([]rune(value)[:max])
}

// ExpirePendingFriendRequests 将已过期的 pending 申请转为 expired。
func ExpirePendingFriendRequests(db *gorm.DB, userID uint, now time.Time) error {
	if db == nil {
		return errors.New("database is not initialized")
	}
	updates := map[string]interface{}{
		"status":       common.FriendRequestExpired,
		"pending_key":  nil,
		"processed_at": now,
	}
	return db.Model(&FriendRequest{}).
		Where("(from_user_id = ? OR to_user_id = ?) AND status = ? AND expires_at <= ?", userID, userID, common.FriendRequestPending, now).
		Updates(updates).Error
}

func ExpirePendingFriendRequestsForPair(db *gorm.DB, userID, targetID uint, now time.Time) error {
	if db == nil {
		return errors.New("database is not initialized")
	}
	return db.Model(&FriendRequest{}).
		Where("pair_key = ? AND status = ? AND expires_at <= ?", FriendPairKey(userID, targetID), common.FriendRequestPending, now).
		Updates(map[string]interface{}{"status": common.FriendRequestExpired, "pending_key": nil, "processed_at": now}).Error
}

func LockFriendRequest(db *gorm.DB, requestID uint) (*FriendRequest, error) {
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	var request FriendRequest
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, requestID).Error
	if err != nil {
		return nil, err
	}
	return &request, nil
}

// RejectFriendRequest 拒绝待处理申请，并从拒绝时间开始计算 24 小时冷却期。
func RejectFriendRequest(requestID, recipientID uint, reason string, now time.Time) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	return RejectFriendRequestTx(DB, requestID, recipientID, reason, now)
}

func RejectFriendRequestTx(db *gorm.DB, requestID, recipientID uint, reason string, now time.Time) error {
	if db == nil {
		return errors.New("database is not initialized")
	}

	result := db.Model(&FriendRequest{}).
		Where(
			"id = ? AND to_user_id = ? AND status = ?",
			requestID,
			recipientID,
			common.FriendRequestPending,
		).
		Updates(map[string]interface{}{
			"status":        common.FriendRequestRejected,
			"reject_reason": truncateText(reason, 100),
			"rejected_at":   now,
			"processed_at":  now,
			"pending_key":   nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func CancelFriendRequest(requestID, senderID uint, now time.Time) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	result := DB.Model(&FriendRequest{}).
		Where("id = ? AND from_user_id = ? AND status = ?", requestID, senderID, common.FriendRequestPending).
		Updates(map[string]interface{}{
			"status":       common.FriendRequestCancelled,
			"pending_key":  nil,
			"processed_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// NotificationTypeUpdate 保留旧调用方式，requestID 为好友申请主键。
func NotificationTypeUpdate(userID int, requestID int, status string) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	if !IsFriendRequestStatus(status) || status == common.FriendRequestPending {
		return errors.New("invalid friend request status transition")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		request, err := LockFriendRequest(tx, uint(requestID))
		if err != nil {
			return err
		}
		now := time.Now()
		actorID := uint(userID)
		if status == common.FriendRequestAccepted {
			if request.ToUserID != actorID || !CanTransitionFriendRequest(request.Status, status) {
				return errors.New("only recipient can accept pending friend request")
			}
			if request.Status == common.FriendRequestAccepted {
				return CreateFriendshipTx(tx, request.FromUserID, request.ToUserID)
			}
			if !request.ExpiresAt.IsZero() && !now.Before(request.ExpiresAt) {
				return errors.New("friend request expired")
			}
			if err := CreateFriendshipTx(tx, request.FromUserID, request.ToUserID); err != nil {
				return err
			}
			if err := tx.Model(request).Updates(map[string]interface{}{"status": status, "pending_key": nil, "processed_at": now}).Error; err != nil {
				return err
			}
			return CreateNotificationTx(tx, request.FromUserID, request.ID, "好友申请已通过", "你的好友申请已被同意")
		}
		if status == common.FriendRequestRejected {
			if request.ToUserID != actorID || !CanTransitionFriendRequest(request.Status, status) {
				return errors.New("only recipient can reject pending friend request")
			}
			return RejectFriendRequestTx(tx, request.ID, actorID, "", now)
		}
		if status == common.FriendRequestCancelled {
			if request.FromUserID != actorID || !CanTransitionFriendRequest(request.Status, status) {
				return errors.New("only sender can cancel pending friend request")
			}
			return cancelFriendRequestTx(tx, request.ID, actorID, now)
		}
		if status == common.FriendRequestExpired {
			if !CanTransitionFriendRequest(request.Status, status) || request.ExpiresAt.IsZero() || now.Before(request.ExpiresAt) {
				return errors.New("friend request is not expired")
			}
			return expireFriendRequestTx(tx, request.ID, now)
		}
		return errors.New("unsupported friend request status transition")
	})
}

// NotificationTypePending 检查双方是否存在待处理好友申请。
func NotificationTypePending(userID int, friendID int) (bool, error) {
	if userID <= 0 || friendID <= 0 {
		return false, nil
	}
	request, err := FindPendingFriendRequest(uint(userID), uint(friendID))
	return request != nil, err
}

// 同意申请好友
func AcceptFriendRequest(requestID, recipientID uint) error {
	return NotificationTypeUpdate(int(recipientID), int(requestID), common.FriendRequestAccepted)
}

func cancelFriendRequestTx(db *gorm.DB, requestID, senderID uint, now time.Time) error {
	result := db.Model(&FriendRequest{}).
		Where("id = ? AND from_user_id = ? AND status = ?", requestID, senderID, common.FriendRequestPending).
		Updates(map[string]interface{}{"status": common.FriendRequestCancelled, "pending_key": nil, "processed_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func expireFriendRequestTx(db *gorm.DB, requestID uint, now time.Time) error {
	result := db.Model(&FriendRequest{}).
		Where("id = ? AND status = ?", requestID, common.FriendRequestPending).
		Updates(map[string]interface{}{"status": common.FriendRequestExpired, "pending_key": nil, "processed_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
