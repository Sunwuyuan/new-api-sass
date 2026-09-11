// TenantState keeps mutable workspace settings and caches isolated.
package model_setting

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
)

type WorkspaceState struct {
	thinkingBlacklistCache thinkingBlacklistCompiled
	thinkingBlacklistMu    sync.RWMutex
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			thinkingBlacklistCache: tenant.Clone(thinkingBlacklistCache),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
