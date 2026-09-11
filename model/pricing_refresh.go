package model

import context "context"

// RefreshPricing 强制立即重新计算与定价相关的缓存。
// 该方法用于需要最新数据的内部管理 API，
// 因此会绕过默认的 1 分钟延迟刷新。
func RefreshPricing(tenantCtx context.Context) {
	TenantState(tenantCtx).updatePricingLock.Lock()
	defer TenantState(tenantCtx).updatePricingLock.Unlock()

	TenantState(tenantCtx).modelSupportEndpointsLock.Lock()
	defer TenantState(tenantCtx).modelSupportEndpointsLock.Unlock()

	updatePricing(tenantCtx)
}
