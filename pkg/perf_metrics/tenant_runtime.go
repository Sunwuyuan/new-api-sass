// TenantState keeps mutable workspace settings and caches isolated.
package perfmetrics

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
)

type WorkspaceRuntime struct {
	hotBuckets sync.Map
}

var workspaceRuntimes tenant.Registry[*WorkspaceRuntime]

func TenantRuntime(ctx context.Context) *WorkspaceRuntime {
	value, err := workspaceRuntimes.Get(ctx, func() *WorkspaceRuntime {
		return &WorkspaceRuntime{}
	})
	if err != nil {
		panic(err)
	}
	return value
}
