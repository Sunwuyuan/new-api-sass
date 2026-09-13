package model

import context "context"

func GetModelEnableGroups(tenantCtx context.Context, modelName string) []string {
	// 确保缓存最新
	GetPricing(tenantCtx)

	if modelName == "" {
		return make([]string, 0)
	}

	TenantState(tenantCtx).modelEnableGroupsLock.RLock()
	groups, ok := TenantState(tenantCtx).modelEnableGroups[modelName]
	TenantState(tenantCtx).modelEnableGroupsLock.RUnlock()
	if !ok {
		return make([]string, 0)
	}
	return groups
}

// GetModelQuotaTypes 返回指定模型的计费类型集合（来自缓存）
func GetModelQuotaTypes(tenantCtx context.Context, modelName string) []int {
	GetPricing(tenantCtx)

	TenantState(tenantCtx).modelEnableGroupsLock.RLock()
	quota, ok := TenantState(tenantCtx).modelQuotaTypeMap[modelName]
	TenantState(tenantCtx).modelEnableGroupsLock.RUnlock()
	if !ok {
		return []int{}
	}
	return []int{quota}
}
