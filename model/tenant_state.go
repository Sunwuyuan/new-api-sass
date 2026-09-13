// TenantState keeps mutable workspace settings and caches isolated.
package model

import (
	context "context"
	common "github.com/QuantumNous/new-api/common"
	constant "github.com/QuantumNous/new-api/constant"
	cachex "github.com/QuantumNous/new-api/pkg/cachex"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
	atomic "sync/atomic"
	time "time"
)

type WorkspaceState struct {
	CacheQuotaData                map[string]*QuotaData
	CacheQuotaDataLock            sync.Mutex
	batchUpdateLocks              []sync.Mutex
	batchUpdateStores             []map[int]int
	channel2advancedCustomConfig  map[int]*dto.AdvancedCustomConfig
	channelPollingLocks           sync.Map
	channelStatusLock             sync.Mutex
	channelSyncLock               sync.RWMutex
	channelsIDM                   map[int]*Channel
	fixLock                       sync.Mutex
	group2model2channels          map[string]map[string][]int
	lastGetPricingTime            time.Time
	metadataMutationMu            sync.Mutex
	modelEnableGroups             map[string][]string
	modelEnableGroupsLock         sync.RWMutex
	modelPricingMutationMu        sync.Mutex
	modelQuotaTypeMap             map[string]int
	modelSupportEndpointTypes     map[string][]constant.EndpointType
	modelSupportEndpointsLock     sync.RWMutex
	passkeyOptionMutex            sync.Mutex
	pricingMap                    []Pricing
	subscriptionPlanCache         *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCache     *cachex.HybridCache[SubscriptionPlanInfo]
	subscriptionPlanInfoCacheOnce sync.Once
	supportedEndpointMap          map[string]common.EndpointInfo
	taskAliasRebuildMu            sync.Mutex
	taskAliasViewPtr              atomic.Pointer[taskAliasView]
	updatePricingLock             sync.Mutex
	vendorsList                   []PricingVendor
}

var workspaceStates tenant.Registry[*WorkspaceState]

func TenantState(ctx context.Context) *WorkspaceState {
	value, err := workspaceStates.Get(ctx, func() *WorkspaceState {
		return &WorkspaceState{
			CacheQuotaData:               tenant.Clone(CacheQuotaData),
			batchUpdateLocks:             make([]sync.Mutex, len(batchUpdateLocks)),
			batchUpdateStores:            tenant.Clone(batchUpdateStores),
			channel2advancedCustomConfig: tenant.Clone(channel2advancedCustomConfig),
			channelsIDM:                  tenant.Clone(channelsIDM),
			group2model2channels:         tenant.Clone(group2model2channels),
			lastGetPricingTime:           tenant.Clone(lastGetPricingTime),
			modelEnableGroups:            tenant.Clone(modelEnableGroups),
			modelQuotaTypeMap:            tenant.Clone(modelQuotaTypeMap),
			modelSupportEndpointTypes:    tenant.Clone(modelSupportEndpointTypes),
			pricingMap:                   tenant.Clone(pricingMap),
			subscriptionPlanCache:        tenant.Clone(subscriptionPlanCache),
			subscriptionPlanInfoCache:    tenant.Clone(subscriptionPlanInfoCache),
			supportedEndpointMap:         tenant.Clone(supportedEndpointMap),
			vendorsList:                  tenant.Clone(vendorsList),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
