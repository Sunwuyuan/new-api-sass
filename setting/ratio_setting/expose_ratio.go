package ratio_setting

import context "context"

import "sync/atomic"

var exposeRatioEnabled atomic.Bool

func init() {
	exposeRatioEnabled.Store(false)
}

func SetExposeRatioEnabled(tenantCtx context.Context, enabled bool) {
	TenantState(tenantCtx).exposeRatioEnabled.Store(enabled)
}

func IsExposeRatioEnabled(tenantCtx context.Context) bool {
	return TenantState(tenantCtx).exposeRatioEnabled.Load()
}
