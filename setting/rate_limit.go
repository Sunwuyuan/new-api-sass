package setting

import context "context"

import (
	"fmt"
	"math"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// maxRateLimitDurationSeconds is the largest window the count cap is computed
// against (24h). Token-bucket capacity is count*duration; this keeps that
// product inside int64 when the window is at most a day.
const maxRateLimitDurationSeconds = 24 * 60 * 60

// maxModelRequestRateLimitCount is math.MaxInt64 / maxRateLimitDurationSeconds.
// It is the largest count that cannot overflow int64(count)*duration for a
// window of at most 24 hours.
const maxModelRequestRateLimitCount int64 = math.MaxInt64 / maxRateLimitDurationSeconds

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString(tenantCtx context.Context) string {
	TenantState(tenantCtx).ModelRequestRateLimitMutex.RLock()
	defer TenantState(tenantCtx).ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := common.Marshal(TenantState(tenantCtx).ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(tenantCtx context.Context, jsonStr string) error {
	groups := make(map[string][2]int)
	if err := common.Unmarshal([]byte(jsonStr), &groups); err != nil {
		return err
	}
	TenantState(tenantCtx).ModelRequestRateLimitMutex.Lock()
	defer TenantState(tenantCtx).ModelRequestRateLimitMutex.Unlock()
	TenantState(tenantCtx).ModelRequestRateLimitGroup = groups
	return nil
}

func GetGroupRateLimit(tenantCtx context.Context, group string) (totalCount, successCount int, found bool) {
	TenantState(tenantCtx).ModelRequestRateLimitMutex.RLock()
	defer TenantState(tenantCtx).ModelRequestRateLimitMutex.RUnlock()

	if TenantState(tenantCtx).ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := TenantState(tenantCtx).ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := common.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
		if int64(limits[0]) > maxModelRequestRateLimitCount || int64(limits[1]) > maxModelRequestRateLimitCount {
			return fmt.Errorf("group %s [%d, %d] exceeds max rate limit %d", group, limits[0], limits[1], maxModelRequestRateLimitCount)
		}
	}

	return nil
}
