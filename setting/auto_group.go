package setting

import context "context"

import (
	"fmt"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
)

const DefaultMaxTokenAutoGroups = 5

var autoGroups = []string{
	"default",
}

var DefaultUseAutoGroup = false

var maxTokenAutoGroups atomic.Int64

func init() {
	maxTokenAutoGroups.Store(DefaultMaxTokenAutoGroups)
}

func ContainsAutoGroup(tenantCtx context.Context, group string) bool {
	return slices.Contains(TenantState(tenantCtx).autoGroups, group)
}

func UpdateAutoGroupsByJsonString(tenantCtx context.Context, jsonString string) error {
	groups := make([]string, 0)
	if err := common.Unmarshal([]byte(jsonString), &groups); err != nil {
		return err
	}
	UpdateTenantSettings(tenantCtx, func(state *WorkspaceState) { state.autoGroups = groups })
	return nil
}

func AutoGroups2JsonString(tenantCtx context.Context) string {
	jsonBytes, err := common.Marshal(TenantState(tenantCtx).autoGroups)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func GetAutoGroups(tenantCtx context.Context) []string {
	return slices.Clone(TenantState(tenantCtx).autoGroups)
}

func GetMaxTokenAutoGroups(tenantCtx context.Context) int {
	return int(TenantState(tenantCtx).maxTokenAutoGroups.Load())
}

func ValidateMaxTokenAutoGroups(value string) error {
	maxCount, err := strconv.Atoi(value)
	if err != nil || maxCount <= 0 {
		return fmt.Errorf("MaxTokenAutoGroups must be a positive integer")
	}
	return nil
}

func UpdateMaxTokenAutoGroups(tenantCtx context.Context, value string) error {
	if err := ValidateMaxTokenAutoGroups(value); err != nil {
		return err
	}
	maxCount, _ := strconv.Atoi(value)
	TenantState(tenantCtx).maxTokenAutoGroups.Store(int64(maxCount))
	return nil
}
