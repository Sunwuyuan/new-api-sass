// TenantState keeps mutable workspace settings and caches isolated.
package authz

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	v2 "github.com/casbin/casbin/v2"
	sync "sync"
)

type WorkspaceState struct {
	enforcer   *v2.SyncedEnforcer
	enforcerMu sync.RWMutex
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			enforcer: tenant.Clone(enforcer),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
