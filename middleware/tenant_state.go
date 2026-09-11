// TenantState keeps mutable workspace settings and caches isolated.
package middleware

import (
	context "context"
	common "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	tenant "github.com/QuantumNous/new-api/tenant"
)

type WorkspaceState struct {
	inMemoryRateLimiter          common.InMemoryRateLimiter
	taskArtifactAnonymousLimiter *taskArtifactAccessLimiter
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			inMemoryRateLimiter:          common.InMemoryRateLimiter{},
			taskArtifactAnonymousLimiter: newTaskArtifactAccessLimiter(system_setting.LoadTaskArtifactAccessLimits()),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
