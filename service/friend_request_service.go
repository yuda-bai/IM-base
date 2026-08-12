package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"ginchat/common"
	"ginchat/models"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type friendRequestInput struct {
	UserID    uint   `json:"userId" form:"userId"`
	TargetID  uint   `json:"targetId" form:"targetId"`
	RequestID uint   `json:"requestId" form:"requestId"`
	Remark    string `json:"remark" form:"remark"`
	Reason    string `json:"reason" form:"reason"`
	IDs       []uint `json:"ids" form:"ids"`
}

type friendRequestView struct {
	models.FriendRequest
	FromUserName string `json:"fromUserName"`
	ToUserName   string `json:"toUserName"`
}

func authenticatedUser(c *gin.Context) (models.UserBasic, error) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return models.UserBasic{}, errors.New("缺少Authorization")
	}
	parts := strings.Fields(header)
	identity := header
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		identity = parts[1]
	}
	user := models.FindUserByIdentity(identity)
	if user.ID == 0 {
		return models.UserBasic{}, errors.New("身份已失效")
	}
	c.Set("currentUserID", user.ID)
	return user, nil
}

func failFriendRequestAuth(c *gin.Context, err error) {
	common.Fail(c, err.Error())
}

func bindFriendRequestInput(c *gin.Context, input *friendRequestInput) error {
	if err := c.ShouldBind(input); err != nil {
		return err
	}
	if input.UserID > 0 {
		if uid, ok := c.Get("currentUserID"); ok && uintValue(uid) != input.UserID {
			return errors.New("用户身份不一致")
		}
	}
	return nil
}

func uintValue(value interface{}) uint {
	switch v := value.(type) {
	case uint:
		return v
	case uint64:
		return uint(v)
	case int:
		return uint(v)
	case int64:
		return uint(v)
	case string:
		n, _ := strconv.ParseUint(v, 10, 64)
		return uint(n)
	default:
		return 0
	}
}

func writeFriendRequestEvent(userID uint, event string, request *models.FriendRequest) {
	if request == nil {
		return
	}
	from := models.FindUserByID(request.FromUserID)
	payload, err := json.Marshal(map[string]interface{}{
		"type":         "friend_request",
		"event":        event,
		"requestId":    request.ID,
		"fromUserId":   request.FromUserID,
		"fromUserName": from.Name,
		"toUserId":     request.ToUserID,
		"status":       request.Status,
	})
	if err != nil {
		fmt.Println("好友申请事件序列化失败:", err)
		return
	}
	models.PushToUser(userID, payload)
}

func requestIDFromInput(c *gin.Context, input *friendRequestInput) uint {
	if input.RequestID > 0 {
		return input.RequestID
	}
	value, _ := strconv.ParseUint(c.Param("requestId"), 10, 64)
	return uint(value)
}

// CreateFriendRequest 发送好友申请。
func CreateFriendRequest(c *gin.Context) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	var input friendRequestInput
	if err := bindFriendRequestInput(c, &input); err != nil {
		common.Fail(c, "参数错误: "+err.Error())
		return
	}
	if input.TargetID == 0 || input.TargetID == user.ID {
		common.Fail(c, "目标用户无效或不能添加自己")
		return
	}
	target := models.FindUserByID(input.TargetID)
	if target.ID == 0 {
		common.Fail(c, "目标用户不存在")
		return
	}
	if models.SearchFriendById(user.ID, target.ID) {
		common.Fail(c, "已经是好友")
		return
	}
	now := time.Now()
	if err := models.ExpirePendingFriendRequests(models.DB, user.ID, now); err != nil {
		common.Fail(c, "检查申请有效期失败")
		return
	}
	if err := models.ExpirePendingFriendRequestsForPair(models.DB, user.ID, target.ID, now); err != nil {
		common.Fail(c, "检查目标申请有效期失败")
		return
	}
	if pending, err := models.FindPendingFriendRequest(user.ID, target.ID); err != nil {
		common.Fail(c, "检查重复申请失败")
		return
	} else if pending != nil {
		common.Fail(c, "双方已有待处理申请")
		return
	}
	if cooling, until, err := models.FriendRequestRejectionCooldown(user.ID, target.ID, now); err != nil {
		common.Fail(c, "检查拒绝冷却期失败")
		return
	} else if cooling {
		common.Fail(c, fmt.Sprintf("好友申请冷却至%s", until.Format("2006-01-02 15:04:05")))
		return
	}

	var request *models.FriendRequest
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		pairKey := models.FriendPairKey(user.ID, target.ID)
		request = &models.FriendRequest{
			FromUserID: user.ID,
			ToUserID:   target.ID,
			PairKey:    pairKey,
			PendingKey: &pairKey,
			Remark:     truncateRunes(input.Remark, 100),
			Status:     common.FriendRequestPending,
			ExpiresAt:  now.Add(common.FriendRequestValidity),
		}
		if err := tx.Create(request).Error; err != nil {
			return err
		}
		return models.CreateNotificationTx(tx, target.ID, request.ID, "新的好友申请", fmt.Sprintf("%s 向你发送了好友申请", user.Name))
	})
	if err != nil {
		common.Fail(c, "发送好友申请失败")
		return
	}
	writeFriendRequestEvent(target.ID, "created", request)
	common.Success(c, "好友申请已发送", request)
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

func listFriendRequests(c *gin.Context, received bool) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	requests, err := models.FindFriendRequests(user.ID, received)
	if err != nil {
		common.Fail(c, "获取好友申请失败")
		return
	}
	views := make([]friendRequestView, 0, len(requests))
	for _, request := range requests {
		from := models.FindUserByID(request.FromUserID)
		to := models.FindUserByID(request.ToUserID)
		views = append(views, friendRequestView{FriendRequest: request, FromUserName: from.Name, ToUserName: to.Name})
	}
	common.Success(c, "获取好友申请成功", views)
}

func GetReceivedFriendRequests(c *gin.Context) { listFriendRequests(c, true) }
func GetSentFriendRequests(c *gin.Context)     { listFriendRequests(c, false) }

func GetFriendRequestUnreadCount(c *gin.Context) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	_ = models.ExpirePendingFriendRequests(models.DB, user.ID, time.Now())
	var count int64
	if err := models.DB.Model(&models.FriendRequest{}).Where("to_user_id = ? AND status = ? AND is_read = ?", user.ID, common.FriendRequestPending, false).Count(&count).Error; err != nil {
		common.Fail(c, "获取未读数量失败")
		return
	}
	common.Success(c, "获取未读数量成功", map[string]int64{"count": count})
}

func MarkFriendRequestsRead(c *gin.Context) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	var input friendRequestInput
	if err := bindFriendRequestInput(c, &input); err != nil {
		common.Fail(c, "参数错误: "+err.Error())
		return
	}
	if err := models.MarkFriendRequestsRead(user.ID, input.IDs); err != nil {
		common.Fail(c, "标记好友申请已读失败")
		return
	}
	common.Success(c, "好友申请已读", nil)
}

func AcceptFriendRequest(c *gin.Context) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	var input friendRequestInput
	if err := bindFriendRequestInput(c, &input); err != nil {
		common.Fail(c, "参数错误: "+err.Error())
		return
	}
	requestID := requestIDFromInput(c, &input)
	if requestID == 0 {
		common.Fail(c, "好友申请ID无效")
		return
	}
	var request models.FriendRequest
	alreadyAccepted := false
	expired := false
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		locked, lockErr := models.LockFriendRequest(tx, requestID)
		if lockErr != nil {
			return lockErr
		}
		request = *locked
		if request.ToUserID != user.ID {
			return errors.New("只有被申请人可以同意")
		}
		if request.Status == common.FriendRequestAccepted {
			alreadyAccepted = true
			if err := models.CreateFriendshipTx(tx, request.FromUserID, request.ToUserID); err != nil {
				return err
			}
			return nil
		}
		if request.Status != common.FriendRequestPending {
			return errors.New("好友申请状态不允许同意")
		}
		now := time.Now()
		if !request.ExpiresAt.IsZero() && !now.Before(request.ExpiresAt) {
			if err := tx.Model(&request).Updates(map[string]interface{}{"status": common.FriendRequestExpired, "pending_key": nil, "processed_at": now}).Error; err != nil {
				return err
			}
			expired = true
			request.Status = common.FriendRequestExpired
			request.ProcessedAt = &now
			return nil
		}
		if err := models.CreateFriendshipTx(tx, request.FromUserID, request.ToUserID); err != nil {
			return err
		}
		now = time.Now()
		if err := tx.Model(&request).Updates(map[string]interface{}{"status": common.FriendRequestAccepted, "pending_key": nil, "processed_at": now}).Error; err != nil {
			return err
		}
		request.Status = common.FriendRequestAccepted
		request.ProcessedAt = &now
		if err := models.CreateNotificationTx(tx, request.FromUserID, request.ID, "好友申请已通过", fmt.Sprintf("%s 已同意你的好友申请", user.Name)); err != nil {
			return err
		}
		return models.CreateNotificationTx(tx, request.ToUserID, request.ID, "好友关系已建立", fmt.Sprintf("你已与 %s 成为好友", models.FindUserByID(request.FromUserID).Name))
	})
	if err != nil {
		common.Fail(c, err.Error())
		return
	}
	if expired {
		common.Fail(c, models.ErrFriendRequestExpired.Error())
		return
	}
	if !alreadyAccepted {
		writeFriendRequestEvent(request.FromUserID, "accepted", &request)
	}
	writeFriendRequestEvent(request.ToUserID, "accepted", &request)
	common.Success(c, "好友申请已通过", &request)
}

func RejectFriendRequestV2(c *gin.Context) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	var input friendRequestInput
	if err := bindFriendRequestInput(c, &input); err != nil {
		common.Fail(c, "参数错误: "+err.Error())
		return
	}
	requestID := requestIDFromInput(c, &input)
	if requestID == 0 {
		common.Fail(c, "好友申请ID无效")
		return
	}
	now := time.Now()
	var request models.FriendRequest
	expired := false
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		locked, lockErr := models.LockFriendRequest(tx, requestID)
		if lockErr != nil {
			return lockErr
		}
		request = *locked
		if request.ToUserID != user.ID {
			return errors.New("无权处理该好友申请")
		}
		if request.Status != common.FriendRequestPending {
			return errors.New("好友申请状态不允许拒绝")
		}
		if !request.ExpiresAt.IsZero() && !now.Before(request.ExpiresAt) {
			if err := tx.Model(&request).Updates(map[string]interface{}{"status": common.FriendRequestExpired, "pending_key": nil, "processed_at": now}).Error; err != nil {
				return err
			}
			expired = true
			request.Status = common.FriendRequestExpired
			request.ProcessedAt = &now
			return nil
		}
		if err := models.RejectFriendRequestTx(tx, requestID, user.ID, input.Reason, now); err != nil {
			return err
		}
		return models.CreateNotificationTx(tx, request.FromUserID, request.ID, "好友申请未通过", fmt.Sprintf("%s 拒绝了你的好友申请", user.Name))
	})
	if err != nil {
		common.Fail(c, "拒绝好友申请失败")
		return
	}
	if expired {
		common.Fail(c, models.ErrFriendRequestExpired.Error())
		return
	}
	request.Status = common.FriendRequestRejected
	request.RejectReason = truncateRunes(input.Reason, 100)
	request.ProcessedAt = &now
	writeFriendRequestEvent(request.FromUserID, "rejected", &request)
	common.Success(c, "好友申请已拒绝", &request)
}

func CancelFriendRequest(c *gin.Context) {
	user, err := authenticatedUser(c)
	if err != nil {
		failFriendRequestAuth(c, err)
		return
	}
	var input friendRequestInput
	if err := bindFriendRequestInput(c, &input); err != nil {
		common.Fail(c, "参数错误: "+err.Error())
		return
	}
	requestID := requestIDFromInput(c, &input)
	request, err := models.FindFriendRequestByID(requestID)
	if err != nil || request == nil || request.FromUserID != user.ID {
		common.Fail(c, "无权撤回该好友申请")
		return
	}
	if request.Status != common.FriendRequestPending {
		common.Fail(c, "好友申请状态不允许撤回")
		return
	}
	now := time.Now()
	if err := models.CancelFriendRequest(requestID, user.ID, now); err != nil {
		common.Fail(c, "撤回好友申请失败")
		return
	}
	request.Status = common.FriendRequestCancelled
	request.ProcessedAt = &now
	writeFriendRequestEvent(request.ToUserID, "cancelled", request)
	common.Success(c, "好友申请已撤回", request)
}
