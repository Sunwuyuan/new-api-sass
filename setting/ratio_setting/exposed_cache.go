package ratio_setting

import context "context"

import (
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const exposedDataTTL = 30 * time.Second

type exposedCache struct {
	data      gin.H
	expiresAt time.Time
}

var (
	exposedData atomic.Value
	rebuildMu   sync.Mutex
)

func InvalidateExposedDataCache(tenantCtx context.Context) {
	TenantState(tenantCtx).exposedData.Store((*exposedCache)(nil))
}

func cloneGinH(src gin.H) gin.H {
	dst := make(gin.H, len(src))
	maps.Copy(dst, src)
	return dst
}

func GetExposedData(tenantCtx context.Context) gin.H {
	if c, ok := TenantState(tenantCtx).exposedData.Load().(*exposedCache); ok && c != nil && time.Now().Before(c.expiresAt) {
		return cloneGinH(c.data)
	}
	TenantState(tenantCtx).rebuildMu.Lock()
	defer TenantState(tenantCtx).rebuildMu.Unlock()
	if c, ok := TenantState(tenantCtx).exposedData.Load().(*exposedCache); ok && c != nil && time.Now().Before(c.expiresAt) {
		return cloneGinH(c.data)
	}
	newData := gin.H{
		"model_ratio":        GetModelRatioCopy(tenantCtx),
		"completion_ratio":   GetCompletionRatioCopy(tenantCtx),
		"cache_ratio":        GetCacheRatioCopy(tenantCtx),
		"create_cache_ratio": GetCreateCacheRatioCopy(tenantCtx),
		"model_price":        GetModelPriceCopy(tenantCtx),
	}
	TenantState(tenantCtx).exposedData.Store(&exposedCache{
		data:      newData,
		expiresAt: time.Now().Add(exposedDataTTL),
	})
	return cloneGinH(newData)
}
