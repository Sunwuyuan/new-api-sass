// TenantState keeps mutable workspace settings and caches isolated.
package service

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
)

type WorkspaceState struct {
	cleanupOnce      sync.Once
	notifyLimitStore sync.Map
	rankingCache     map[string]rankingCacheItem
	rankingCacheMu   sync.Mutex
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			rankingCache: tenant.Clone(rankingCache),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
