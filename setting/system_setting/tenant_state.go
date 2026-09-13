// TenantState keeps mutable workspace settings and caches isolated.
package system_setting

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	regexp "regexp"
)

type WorkspaceState struct {
	ServerAddress                      string
	TaskPublicAddress                  string
	WorkerAllowHttpImageRequestEnabled bool
	WorkerUrl                          string
	WorkerValidKey                     string
	taskArtifactStoreBucketPattern     *regexp.Regexp
	taskArtifactStoreRegionPattern     *regexp.Regexp
}

var workspaceStates tenant.Registry[*tenant.Settings[WorkspaceState]]

func tenantSettings(ctx context.Context) *tenant.Settings[WorkspaceState] {
	value, err := workspaceStates.Get(ctx, func() *tenant.Settings[WorkspaceState] {
		return tenant.NewSettings(&WorkspaceState{
			ServerAddress:                      tenant.Clone(ServerAddress),
			TaskPublicAddress:                  tenant.Clone(TaskPublicAddress),
			WorkerAllowHttpImageRequestEnabled: tenant.Clone(WorkerAllowHttpImageRequestEnabled),
			WorkerUrl:                          tenant.Clone(WorkerUrl),
			WorkerValidKey:                     tenant.Clone(WorkerValidKey),
			taskArtifactStoreBucketPattern:     tenant.Clone(taskArtifactStoreBucketPattern),
			taskArtifactStoreRegionPattern:     tenant.Clone(taskArtifactStoreRegionPattern),
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
