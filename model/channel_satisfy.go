package model

import context "context"

import (
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(tenantCtx context.Context, group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(tenantCtx, group, modelName, channelID)
	}

	TenantState(tenantCtx).channelSyncLock.RLock()
	defer TenantState(tenantCtx).channelSyncLock.RUnlock()

	if TenantState(tenantCtx).group2model2channels == nil {
		return false
	}

	if isChannelIDInList(TenantState(tenantCtx).group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(tenantCtx, modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(TenantState(tenantCtx).group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(tenantCtx context.Context, groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(tenantCtx, g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(tenantCtx context.Context, group string, modelName string, channelID int) bool {
	var count int64
	err := DB.WithContext(tenantCtx).Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(tenantCtx, modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.WithContext(tenantCtx).Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	return slices.Contains(list, channelID)
}
