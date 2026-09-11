package common

import context "context"

func GetTrustQuota(tenantCtx context.Context) int {
	return int(10 * TenantState(tenantCtx).QuotaPerUnit)
}
