package service

import context "context"

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/bytedance/gopkg/util/gopool"
)

// notifyLimitStore is used for in-memory rate limiting when Redis is disabled
var (
	notifyLimitStore sync.Map
	cleanupOnce      sync.Once
)

type limitCount struct {
	Count     int
	Timestamp time.Time
}

func getDuration() time.Duration {
	minute := constant.NotificationLimitDurationMinute
	return time.Duration(minute) * time.Minute
}

// startCleanupTask starts a background task to clean up expired entries
func startCleanupTask(tenantCtx context.Context) {
	gopool.Go(func() {
		for {
			time.Sleep(time.Hour)
			now := time.Now()
			TenantState(tenantCtx).notifyLimitStore.Range(func(key, value any) bool {
				if limit, ok := value.(limitCount); ok {
					if now.Sub(limit.Timestamp) >= getDuration() {
						TenantState(tenantCtx).notifyLimitStore.Delete(key)
					}
				}
				return true
			})
		}
	})
}

// CheckNotificationLimit checks if the user has exceeded their notification limit
// Returns true if the user can send notification, false if limit exceeded
func CheckNotificationLimit(tenantCtx context.Context, userId int, notifyType string) (bool, error) {
	if common.RedisEnabled {
		return checkRedisLimit(tenantCtx, userId, notifyType)
	}
	return checkMemoryLimit(tenantCtx, userId, notifyType)
}

func checkRedisLimit(tenantCtx context.Context, userId int, notifyType string) (bool, error) {
	key := fmt.Sprintf("notify_limit:%d:%s:%s", userId, notifyType, time.Now().Format("2006010215"))

	// Get current count
	count, err := common.RedisGet(tenantCtx, key)
	if err != nil && err.Error() != "redis: nil" {
		return false, fmt.Errorf("failed to get notification count: %w", err)
	}

	// If key doesn't exist, initialize it
	if count == "" {
		err = common.RedisSet(tenantCtx, key, "1", getDuration())
		return true, err
	}

	currentCount, _ := strconv.Atoi(count)
	limit := constant.NotifyLimitCount

	// Check if limit is already reached
	if currentCount >= limit {
		return false, nil
	}

	// Only increment if under limit
	err = common.RedisIncr(tenantCtx, key, 1)
	if err != nil {
		return false, fmt.Errorf("failed to increment notification count: %w", err)
	}

	return true, nil
}

func checkMemoryLimit(tenantCtx context.Context, userId int, notifyType string) (bool, error) {
	// Ensure cleanup task is started
	TenantState(tenantCtx).cleanupOnce.Do(func() { startCleanupTask(tenantCtx) })

	key := fmt.Sprintf("%d:%s:%s", userId, notifyType, time.Now().Format("2006010215"))
	now := time.Now()

	// Get current limit count or initialize new one
	var currentLimit limitCount
	if value, ok := TenantState(tenantCtx).notifyLimitStore.Load(key); ok {
		currentLimit = value.(limitCount)
		// Check if the entry has expired
		if now.Sub(currentLimit.Timestamp) >= getDuration() {
			currentLimit = limitCount{Count: 0, Timestamp: now}
		}
	} else {
		currentLimit = limitCount{Count: 0, Timestamp: now}
	}

	// Increment count
	currentLimit.Count++

	// Check against limits
	limit := constant.NotifyLimitCount

	// Store updated count
	TenantState(tenantCtx).notifyLimitStore.Store(key, currentLimit)

	return currentLimit.Count <= limit, nil
}
