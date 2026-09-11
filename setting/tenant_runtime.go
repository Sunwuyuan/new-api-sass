// TenantState keeps mutable workspace settings and caches isolated.
package setting

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
)

type WorkspaceRuntime struct {
	DefaultUseAutoGroup bool
}

var workspaceRuntimes tenant.Registry[*tenant.Settings[WorkspaceRuntime]]

func tenantRuntimeSettings(ctx context.Context) *tenant.Settings[WorkspaceRuntime] {
	value, err := workspaceRuntimes.Get(ctx, func() *tenant.Settings[WorkspaceRuntime] {
		return tenant.NewSettings(&WorkspaceRuntime{
			DefaultUseAutoGroup: tenant.Clone(DefaultUseAutoGroup),
		})
	})
	if err != nil {
		panic(err)
	}
	return value
}

// TenantRuntime returns a read-only snapshot. Use UpdateTenantRuntime to publish changes.
func TenantRuntime(ctx context.Context) *WorkspaceRuntime {
	return tenantRuntimeSettings(ctx).Load()
}

func UpdateTenantRuntime(ctx context.Context, update func(*WorkspaceRuntime)) {
	tenantRuntimeSettings(ctx).Update(update)
}
