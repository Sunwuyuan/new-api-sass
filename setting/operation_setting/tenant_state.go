// TenantState keeps mutable workspace settings and caches isolated.
package operation_setting

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	atomic "sync/atomic"
)

type WorkspaceState struct {
	*workspaceCaches
	AutomaticDisableKeywords         []string
	AutomaticDisableStatusCodeRanges []StatusCodeRange
	AutomaticRetryStatusCodeRanges   []StatusCodeRange
	CustomCallbackAddress            string
	DemoSiteEnabled                  bool
	EpayId                           string
	EpayKey                          string
	MinTopUp                         int
	PayAddress                       string
	PayMethods                       []map[string]string
	Price                            float64
	SelfUseModeEnabled               bool
	USDExchangeRate                  float64
}

type workspaceCaches struct {
	currentIndex atomic.Pointer[toolPriceIndex]
}

var workspaceStates tenant.Registry[*tenant.Settings[WorkspaceState]]

func tenantSettings(ctx context.Context) *tenant.Settings[WorkspaceState] {
	value, err := workspaceStates.Get(ctx, func() *tenant.Settings[WorkspaceState] {
		return tenant.NewSettings(&WorkspaceState{
			workspaceCaches:                  &workspaceCaches{},
			AutomaticDisableKeywords:         tenant.Clone(AutomaticDisableKeywords),
			AutomaticDisableStatusCodeRanges: tenant.Clone(AutomaticDisableStatusCodeRanges),
			AutomaticRetryStatusCodeRanges:   tenant.Clone(AutomaticRetryStatusCodeRanges),
			CustomCallbackAddress:            tenant.Clone(CustomCallbackAddress),
			DemoSiteEnabled:                  tenant.Clone(DemoSiteEnabled),
			EpayId:                           tenant.Clone(EpayId),
			EpayKey:                          tenant.Clone(EpayKey),
			MinTopUp:                         tenant.Clone(MinTopUp),
			PayAddress:                       tenant.Clone(PayAddress),
			PayMethods:                       tenant.Clone(PayMethods),
			Price:                            tenant.Clone(Price),
			SelfUseModeEnabled:               tenant.Clone(SelfUseModeEnabled),
			USDExchangeRate:                  tenant.Clone(USDExchangeRate),
		})
	})
	if err != nil {
		panic(err)
	}
	return value
}

// TenantState returns a read-only snapshot. Use UpdateTenantSettings to publish changes.
func TenantState(ctx context.Context) *WorkspaceState {
	return tenantSettings(ctx).Load()
}

func UpdateTenantSettings(ctx context.Context, update func(*WorkspaceState)) {
	tenantSettings(ctx).Update(update)
}
