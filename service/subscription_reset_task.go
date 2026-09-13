package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	subscriptionResetBatchSize  = 300
	subscriptionCleanupInterval = 30 * time.Minute
)

func runSubscriptionQuotaResetOnce(tenantCtx context.Context) {
	if !TenantRuntime(tenantCtx).subscriptionResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer TenantRuntime(tenantCtx).subscriptionResetRunning.Store(false)

	ctx := tenantCtx
	totalReset := 0
	totalExpired := 0
	for {
		n, err := model.ExpireDueSubscriptions(tenantCtx, subscriptionResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription expire task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalExpired += n
		if n < subscriptionResetBatchSize {
			break
		}
	}
	for {
		n, err := model.ResetDueSubscriptions(tenantCtx, subscriptionResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription quota reset task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalReset += n
		if n < subscriptionResetBatchSize {
			break
		}
	}
	lastCleanup := time.Unix(TenantRuntime(tenantCtx).subscriptionCleanupLast.Load(), 0)
	if time.Since(lastCleanup) >= subscriptionCleanupInterval {
		if _, err := model.CleanupSubscriptionPreConsumeRecords(tenantCtx, 7*24*3600); err == nil {
			TenantRuntime(tenantCtx).subscriptionCleanupLast.Store(time.Now().Unix())
		}
	}
	if common.DebugEnabled && (totalReset > 0 || totalExpired > 0) {
		logger.LogDebug(ctx, "subscription maintenance: reset_count=%d, expired_count=%d", totalReset, totalExpired)
	}
}
