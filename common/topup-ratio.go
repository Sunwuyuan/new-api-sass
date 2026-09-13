package common

import context "context"

import (
	"sync"
)

var topupGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}
var topupGroupRatioMutex sync.RWMutex

func TopupGroupRatio2JSONString(tenantCtx context.Context) string {
	TenantState(tenantCtx).topupGroupRatioMutex.RLock()
	defer TenantState(tenantCtx).topupGroupRatioMutex.RUnlock()
	jsonBytes, err := Marshal(TenantState(tenantCtx).topupGroupRatio)
	if err != nil {
		SysError("error marshalling topup group ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateTopupGroupRatioByJSONString(tenantCtx context.Context, jsonStr string) error {
	TenantState(tenantCtx).topupGroupRatioMutex.Lock()
	defer TenantState(tenantCtx).topupGroupRatioMutex.Unlock()
	TenantState(tenantCtx).topupGroupRatio = make(map[string]float64)
	return Unmarshal([]byte(jsonStr), &TenantState(tenantCtx).topupGroupRatio)
}

func GetTopupGroupRatio(tenantCtx context.Context, name string) float64 {
	TenantState(tenantCtx).topupGroupRatioMutex.RLock()
	defer TenantState(tenantCtx).topupGroupRatioMutex.RUnlock()
	ratio, ok := TenantState(tenantCtx).topupGroupRatio[name]
	if !ok {
		SysError("topup group ratio not found: " + name)
		return 1
	}
	return ratio
}
