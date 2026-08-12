package models

import (
	"testing"
	"time"

	"ginchat/common"
)

func TestRejectionCooldownBoundary(t *testing.T) {
	rejectedAt := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)

	if !IsInRejectionCooldown(rejectedAt, rejectedAt.Add(24*time.Hour-time.Nanosecond)) {
		t.Fatal("满 24 小时前应处于冷却期")
	}
	if IsInRejectionCooldown(rejectedAt, rejectedAt.Add(24*time.Hour)) {
		t.Fatal("恰好满 24 小时应允许再次申请")
	}
	if IsInRejectionCooldown(rejectedAt, rejectedAt.Add(25*time.Hour)) {
		t.Fatal("超过 24 小时不应处于冷却期")
	}
}

func TestRejectionCooldownUntil(t *testing.T) {
	rejectedAt := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	want := time.Date(2026, time.August, 12, 10, 0, 0, 0, rejectedAt.Location())
	if got := RejectionCooldownUntil(rejectedAt); !got.Equal(want) {
		t.Fatalf("冷却结束时间为 %s，期望 %s", got, want)
	}
}

func TestFriendPairKeyIsDirectionIndependent(t *testing.T) {
	if FriendPairKey(12, 3) != "3:12" || FriendPairKey(3, 12) != "3:12" {
		t.Fatal("同一用户对的 pair key 应与申请方向无关")
	}
}

func TestFriendRequestStatusTransitions(t *testing.T) {
	valid := []struct {
		current string
		target  string
		want    bool
	}{
		{common.FriendRequestPending, common.FriendRequestAccepted, true},
		{common.FriendRequestPending, common.FriendRequestRejected, true},
		{common.FriendRequestPending, common.FriendRequestCancelled, true},
		{common.FriendRequestPending, common.FriendRequestExpired, true},
		{common.FriendRequestAccepted, common.FriendRequestAccepted, true},
		{common.FriendRequestAccepted, common.FriendRequestRejected, false},
		{common.FriendRequestRejected, common.FriendRequestAccepted, false},
		{common.FriendRequestCancelled, common.FriendRequestPending, false},
	}
	for _, test := range valid {
		if got := CanTransitionFriendRequest(test.current, test.target); got != test.want {
			t.Fatalf("状态转换 %s -> %s = %v，期望 %v", test.current, test.target, got, test.want)
		}
	}
}
