package setting

import context "context"

import (
	"slices"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	TaskPluginMarketplaceSourcesKey  = "TaskPluginMarketplaceSources"
	TaskPluginDisabledFactoryKeysKey = "TaskPluginDisabledFactoryKeys"

	officialTaskPluginMarketplaceIndexURL = "https://www.newapi.ai/api/v1/plugins/index.json"
	githubTaskPluginMarketplaceIndexURL   = "https://raw.githubusercontent.com/QuantumNous/new-api-plugins/main/index.json"
)

type TaskPluginMarketplaceSource struct {
	Name     string `json:"name"`
	IndexURL string `json:"index_url"`
}

func defaultTaskPluginMarketplaceSources() []TaskPluginMarketplaceSource {
	return []TaskPluginMarketplaceSource{
		{Name: "Official", IndexURL: officialTaskPluginMarketplaceIndexURL},
		{Name: "GitHub", IndexURL: githubTaskPluginMarketplaceIndexURL},
	}
}

func GetTaskPluginMarketplaceSources(tenantCtx context.Context) []TaskPluginMarketplaceSource {
	common.TenantState(tenantCtx).OptionMapRWMutex.RLock()
	raw := ""
	if common.TenantState(tenantCtx).OptionMap != nil {
		raw = common.TenantState(tenantCtx).OptionMap[TaskPluginMarketplaceSourcesKey]
	}
	common.TenantState(tenantCtx).OptionMapRWMutex.RUnlock()

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultTaskPluginMarketplaceSources()
	}
	var sources []TaskPluginMarketplaceSource
	if err := common.UnmarshalJsonStr(raw, &sources); err != nil {
		return defaultTaskPluginMarketplaceSources()
	}
	if sources == nil {
		return []TaskPluginMarketplaceSource{}
	}
	return sources
}

func TaskPluginMarketplaceSources2JsonString() string {
	encoded, err := common.Marshal(defaultTaskPluginMarketplaceSources())
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func ParseTaskPluginDisabledFactoryKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var keys []string
	if err := common.Unmarshal([]byte(raw), &keys); err != nil {
		return []string{}
	}
	if keys == nil {
		return []string{}
	}
	return keys
}

func GetTaskPluginDisabledFactoryKeys(tenantCtx context.Context) []string {
	common.TenantState(tenantCtx).OptionMapRWMutex.RLock()
	raw := ""
	if common.TenantState(tenantCtx).OptionMap != nil {
		raw = common.TenantState(tenantCtx).OptionMap[TaskPluginDisabledFactoryKeysKey]
	}
	common.TenantState(tenantCtx).OptionMapRWMutex.RUnlock()
	return ParseTaskPluginDisabledFactoryKeys(raw)
}

func SetTaskPluginDisabledFactoryKeysOption(tenantCtx context.Context, keys []string) error {
	normalized := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	sort.Strings(normalized)
	encoded, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	common.TenantState(tenantCtx).OptionMapRWMutex.Lock()
	if common.TenantState(tenantCtx).OptionMap == nil {
		common.TenantState(tenantCtx).OptionMap = make(map[string]string)
	}
	common.TenantState(tenantCtx).OptionMap[TaskPluginDisabledFactoryKeysKey] = string(encoded)
	common.TenantState(tenantCtx).OptionMapRWMutex.Unlock()
	return nil
}

func IsTaskPluginFactoryDisabled(tenantCtx context.Context, key string) bool {
	return slices.Contains(GetTaskPluginDisabledFactoryKeys(tenantCtx), key)
}
