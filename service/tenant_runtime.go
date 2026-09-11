// TenantState keeps mutable workspace settings and caches isolated.
package service

import (
	context "context"
	cachex "github.com/QuantumNous/new-api/pkg/cachex"
	tenant "github.com/QuantumNous/new-api/tenant"
	http "net/http"
	sync "sync"
	"sync/atomic"
)

type WorkspaceRuntime struct {
	codexCredentialRefreshRunning       atomic.Bool
	subscriptionResetRunning            atomic.Bool
	subscriptionCleanupLast             atomic.Int64
	channelAffinityCache                *cachex.HybridCache[int]
	channelAffinityCacheOnce            sync.Once
	channelAffinityRegexCache           sync.Map
	channelAffinityUsageCacheStatsCache *cachex.HybridCache[ChannelAffinityUsageCacheCounters]
	channelAffinityUsageCacheStatsLocks [64]sync.Mutex
	channelAffinityUsageCacheStatsOnce  sync.Once
	httpClient                          *http.Client
	legacyProxyURLWarnings              sync.Map
	proxyClients                        proxyHTTPClientCache
	ssrfProtectedHTTPClient             *http.Client
}

var workspaceRuntimes tenant.Registry[*WorkspaceRuntime]

func TenantRuntime(ctx context.Context) *WorkspaceRuntime {
	value, err := workspaceRuntimes.Get(ctx, func() *WorkspaceRuntime {
		return &WorkspaceRuntime{
			channelAffinityCache:                tenant.Clone(channelAffinityCache),
			channelAffinityUsageCacheStatsCache: tenant.Clone(channelAffinityUsageCacheStatsCache),
			httpClient:                          tenant.Clone(httpClient),
			proxyClients:                        proxyHTTPClientCache{clients: make(map[string]*http.Client), aliases: make(map[string]string)},
			ssrfProtectedHTTPClient:             tenant.Clone(ssrfProtectedHTTPClient),
		}
	})
	if err != nil {
		panic(err)
	}
	return value
}
