package service

import context "context"

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func cleanupAuthArtifacts(tenantCtx context.Context) {
	now := time.Now()
	count, err := model.CountUserSessionsCreatedSince(tenantCtx, 0, now.Add(-time.Hour).Unix())
	if err != nil {
		common.SysError("failed to count hourly user session issuance: " + err.Error())
	} else if count > int64(common.UserSessionHourlyAlertThreshold) {
		common.SysError(fmt.Sprintf(
			"hourly user session issuance exceeded alert threshold: count=%d threshold=%d window_seconds=%d",
			count,
			common.UserSessionHourlyAlertThreshold,
			int64(time.Hour/time.Second),
		))
	}
	if err := model.DeleteExpiredUserSessions(tenantCtx, now.Unix()); err != nil {
		common.SysError("failed to delete expired user sessions: " + err.Error())
	}
	if err := model.DeleteOldRevokedUserSessions(tenantCtx, now.Unix()); err != nil {
		common.SysError("failed to delete old revoked user sessions: " + err.Error())
	}
	if err := model.DeleteExpiredAuthFlows(tenantCtx, now); err != nil {
		common.SysError("failed to delete expired authentication flows: " + err.Error())
	}
}
