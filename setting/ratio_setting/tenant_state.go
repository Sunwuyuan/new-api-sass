// TenantState keeps mutable workspace settings and caches isolated.
package ratio_setting

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	types "github.com/QuantumNous/new-api/types"
	sync "sync"
	atomic "sync/atomic"
)

type WorkspaceState struct {
	audioCompletionRatioMap *types.RWMap[string, float64]
	audioRatioMap           *types.RWMap[string, float64]
	cacheRatioMap           *types.RWMap[string, float64]
	completionRatioMap      *types.RWMap[string, float64]
	createCacheRatioMap     *types.RWMap[string, float64]
	exposeRatioEnabled      atomic.Bool
	exposedData             atomic.Value
	groupGroupRatioMap      *types.RWMap[string, map[string]float64]
	groupRatioMap           *types.RWMap[string, float64]
	imageRatioMap           *types.RWMap[string, float64]
	modelPriceMap           *types.RWMap[string, float64]
	modelRatioMap           *types.RWMap[string, float64]
	rebuildMu               sync.Mutex
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			audioCompletionRatioMap: types.NewRWMap[string, float64](),
			audioRatioMap:           types.NewRWMap[string, float64](),
			cacheRatioMap:           types.NewRWMap[string, float64](),
			completionRatioMap:      types.NewRWMap[string, float64](),
			createCacheRatioMap:     types.NewRWMap[string, float64](),
			groupGroupRatioMap:      tenant.Clone(groupGroupRatioMap),
			groupRatioMap:           tenant.Clone(groupRatioMap),
			imageRatioMap:           types.NewRWMap[string, float64](),
			modelPriceMap:           types.NewRWMap[string, float64](),
			modelRatioMap:           types.NewRWMap[string, float64](),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
