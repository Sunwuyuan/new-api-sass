// TenantState keeps mutable workspace settings and caches isolated.
package jsplugin

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
)

type WorkspaceState struct {
	DefaultRegistry *Registry
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			DefaultRegistry: DefaultRegistry.CloneFactory(),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
