// TenantState keeps mutable workspace settings and caches isolated.
package oauth

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
)

type WorkspaceState struct {
	customProviderConflicts map[string]bool
	customProviderSlugs     map[string]bool
	mu                      sync.RWMutex
	providers               map[string]Provider
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			customProviderConflicts: tenant.Clone(customProviderConflicts),
			customProviderSlugs:     tenant.Clone(customProviderSlugs),
			providers:               tenant.Clone(providers),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
