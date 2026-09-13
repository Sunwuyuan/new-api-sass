package ratio_setting

import context "context"

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting(tenantCtx context.Context) *GroupRatioSetting {
	return config.GlobalConfig.ForTenant(tenantCtx).Get("group_ratio_setting").(*GroupRatioSetting)
}

func GetGroupRatioCopy(tenantCtx context.Context) map[string]float64 {
	return GetGroupRatioSetting(tenantCtx).GroupRatio.ReadAll()
}

func ContainsGroupRatio(tenantCtx context.Context, name string) bool {
	_, ok := GetGroupRatioSetting(tenantCtx).GroupRatio.Get(name)
	return ok
}

func GroupRatio2JSONString(tenantCtx context.Context) string {
	return GetGroupRatioSetting(tenantCtx).GroupRatio.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(tenantCtx context.Context, jsonStr string) error {
	return types.LoadFromJsonString(GetGroupRatioSetting(tenantCtx).GroupRatio, jsonStr)
}

func GetGroupRatio(tenantCtx context.Context, name string) float64 {
	ratio, ok := GetGroupRatioSetting(tenantCtx).GroupRatio.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(tenantCtx context.Context, userGroup, usingGroup string) (float64, bool) {
	gp, ok := GetGroupRatioSetting(tenantCtx).GroupGroupRatio.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString(tenantCtx context.Context) string {
	return GetGroupRatioSetting(tenantCtx).GroupGroupRatio.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(tenantCtx context.Context, jsonStr string) error {
	return types.LoadFromJsonString(GetGroupRatioSetting(tenantCtx).GroupGroupRatio, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := common.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}
