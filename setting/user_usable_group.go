package setting

import context "context"

import (
	"maps"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var userUsableGroups = map[string]string{
	"default": "默认分组",
	"vip":     "vip分组",
}
var userUsableGroupsMutex sync.RWMutex

func GetUserUsableGroupsCopy(tenantCtx context.Context) map[string]string {
	TenantState(tenantCtx).userUsableGroupsMutex.RLock()
	defer TenantState(tenantCtx).userUsableGroupsMutex.RUnlock()

	copyUserUsableGroups := make(map[string]string)
	maps.Copy(copyUserUsableGroups, TenantState(tenantCtx).userUsableGroups)
	return copyUserUsableGroups
}

func UserUsableGroups2JSONString(tenantCtx context.Context) string {
	TenantState(tenantCtx).userUsableGroupsMutex.RLock()
	defer TenantState(tenantCtx).userUsableGroupsMutex.RUnlock()

	jsonBytes, err := common.Marshal(TenantState(tenantCtx).userUsableGroups)
	if err != nil {
		common.SysLog("error marshalling user groups: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateUserUsableGroupsByJSONString(tenantCtx context.Context, jsonStr string) error {
	TenantState(tenantCtx).userUsableGroupsMutex.Lock()
	defer TenantState(tenantCtx).userUsableGroupsMutex.Unlock()

	TenantState(tenantCtx).userUsableGroups = make(map[string]string)
	return common.Unmarshal([]byte(jsonStr), &TenantState(tenantCtx).userUsableGroups)
}

func GetUsableGroupDescription(tenantCtx context.Context, groupName string) string {
	TenantState(tenantCtx).userUsableGroupsMutex.RLock()
	defer TenantState(tenantCtx).userUsableGroupsMutex.RUnlock()

	if desc, ok := TenantState(tenantCtx).userUsableGroups[groupName]; ok {
		return desc
	}
	return groupName
}
