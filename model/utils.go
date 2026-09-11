package model

import context "context"

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	BatchUpdateTypeUserQuota = iota
	BatchUpdateTypeTokenQuota
	BatchUpdateTypeUsedQuota
	BatchUpdateTypeChannelUsedQuota
	BatchUpdateTypeRequestCount
	BatchUpdateTypeCount // if you add a new type, you need to add a new map and a new lock
)

var batchUpdateStores []map[int]int
var batchUpdateLocks []sync.Mutex

func init() {
	for range BatchUpdateTypeCount {
		batchUpdateStores = append(batchUpdateStores, make(map[int]int))
		batchUpdateLocks = append(batchUpdateLocks, sync.Mutex{})
	}
}

func InitBatchUpdater(tenantCtx context.Context) {
	gopool.Go(func() {
		for {
			time.Sleep(time.Duration(common.BatchUpdateInterval) * time.Second)
			batchUpdate(tenantCtx)
		}
	})
}

func addNewRecord(tenantCtx context.Context, type_ int, id int, value int) {
	TenantState(tenantCtx).batchUpdateLocks[type_].Lock()
	defer TenantState(tenantCtx).batchUpdateLocks[type_].Unlock()
	old, ok := TenantState(tenantCtx).batchUpdateStores[type_][id]
	if !ok {
		TenantState(tenantCtx).batchUpdateStores[type_][id] = value
		return
	}

	sum := old + value
	if (value > 0 && sum < old) || (value < 0 && sum > old) {
		common.SysError(fmt.Sprintf("batch update overflow: type=%d id=%d old=%d value=%d", type_, id, old, value))
		if value > 0 {
			sum = math.MaxInt
		} else {
			sum = math.MinInt
		}
	}
	TenantState(tenantCtx).batchUpdateStores[type_][id] = sum
}

func batchUpdate(tenantCtx context.Context) {
	// check if there's any data to update
	hasData := false
	for i := range BatchUpdateTypeCount {
		TenantState(tenantCtx).batchUpdateLocks[i].Lock()
		if len(TenantState(tenantCtx).batchUpdateStores[i]) > 0 {
			hasData = true
			TenantState(tenantCtx).batchUpdateLocks[i].Unlock()
			break
		}
		TenantState(tenantCtx).batchUpdateLocks[i].Unlock()
	}

	if !hasData {
		return
	}

	common.SysLog("batch update started")
	stores := make([]map[int]int, BatchUpdateTypeCount)
	for i := range BatchUpdateTypeCount {
		TenantState(tenantCtx).batchUpdateLocks[i].Lock()
		stores[i] = TenantState(tenantCtx).batchUpdateStores[i]
		TenantState(tenantCtx).batchUpdateStores[i] = make(map[int]int)
		TenantState(tenantCtx).batchUpdateLocks[i].Unlock()
	}

	for i, store := range stores {
		if i == BatchUpdateTypeUserQuota || i == BatchUpdateTypeUsedQuota || i == BatchUpdateTypeRequestCount {
			continue
		}
		for key, value := range store {
			switch i {
			case BatchUpdateTypeTokenQuota:
				err := increaseTokenQuota(tenantCtx, key, value)
				if err != nil {
					common.SysLog("failed to batch update token quota: " + err.Error())
				}
			case BatchUpdateTypeChannelUsedQuota:
				updateChannelUsedQuota(tenantCtx, key, value)
			}
		}
	}

	userQuotaStore := stores[BatchUpdateTypeUserQuota]
	usedQuotaStore := stores[BatchUpdateTypeUsedQuota]
	requestCountStore := stores[BatchUpdateTypeRequestCount]

	userIDs := make(map[int]struct{}, len(userQuotaStore)+len(usedQuotaStore)+len(requestCountStore))
	for key := range userQuotaStore {
		userIDs[key] = struct{}{}
	}
	for key := range usedQuotaStore {
		userIDs[key] = struct{}{}
	}
	for key := range requestCountStore {
		userIDs[key] = struct{}{}
	}
	for key := range userIDs {
		updateUserQuotaUsedQuotaAndRequestCount(tenantCtx, key, userQuotaStore[key], usedQuotaStore[key], requestCountStore[key])
	}
	common.SysLog("batch update finished")
}

func RecordExist(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func shouldUpdateRedis(fromDB bool, err error) bool {
	return common.RedisEnabled && fromDB && err == nil
}
