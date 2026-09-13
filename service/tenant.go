package service

import (
	"context"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// RunTenantJobs is one pass of the shared scheduler. Work retains its explicit
// tenant context when it enters the existing bounded goroutine pool.
func RunTenantJobs(ctx context.Context, runnerID string, maintenance bool) {
	if err := model.ExpireStaleSystemTaskLocks(ctx, common.GetTimestamp()); err != nil {
		logger.LogWarn(ctx, "expire workspace task leases: "+err.Error())
	}
	runSystemTaskScheduler(ctx)
	runSystemTaskClaimPass(ctx, runnerID)
	runSubscriptionQuotaResetOnce(ctx)
	if maintenance {
		cleanupAuthArtifacts(ctx)
		runCodexCredentialAutoRefreshOnce(ctx)
	}
}
