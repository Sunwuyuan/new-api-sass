package model

import context "context"

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

type PricingPluginVariant struct {
	PluginKey            string                               `json:"plugin_key"`
	PluginName           string                               `json:"plugin_name"`
	Icon                 string                               `json:"icon,omitempty"`
	BillingExpr          string                               `json:"billing_expr"`
	BillingMode          string                               `json:"billing_mode"`
	BillingUsageSchema   map[string]jsplugin.UsageFieldSchema `json:"billing_usage_schema"`
	BillingUsageExamples []jsplugin.UsageExample              `json:"billing_usage_examples,omitempty"`
}

type Pricing struct {
	BillingPluginVariants  []PricingPluginVariant               `json:"billing_plugin_variants,omitempty"`
	ModelName              string                               `json:"model_name"`
	Description            string                               `json:"description,omitempty"`
	Icon                   string                               `json:"icon,omitempty"`
	Tags                   string                               `json:"tags,omitempty"`
	VendorID               int                                  `json:"vendor_id,omitempty"`
	QuotaType              int                                  `json:"quota_type"`
	ModelRatio             float64                              `json:"model_ratio"`
	ModelPrice             float64                              `json:"model_price"`
	OwnerBy                string                               `json:"owner_by"`
	CompletionRatio        float64                              `json:"completion_ratio"`
	CacheRatio             *float64                             `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                             `json:"create_cache_ratio,omitempty"`
	ImageRatio             *float64                             `json:"image_ratio,omitempty"`
	AudioRatio             *float64                             `json:"audio_ratio,omitempty"`
	AudioCompletionRatio   *float64                             `json:"audio_completion_ratio,omitempty"`
	EnableGroup            []string                             `json:"enable_groups"`
	SupportedEndpointTypes []constant.EndpointType              `json:"supported_endpoint_types"`
	BillingMode            string                               `json:"billing_mode,omitempty"`
	BillingExpr            string                               `json:"billing_expr,omitempty"`
	BillingUsageSchema     map[string]jsplugin.UsageFieldSchema `json:"billing_usage_schema,omitempty"`
	BillingUsageExamples   []jsplugin.UsageExample              `json:"billing_usage_examples,omitempty"`
	PricingVersion         string                               `json:"pricing_version,omitempty"`
}

type PricingVendor struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

var (
	pricingMap           []Pricing
	vendorsList          []PricingVendor
	supportedEndpointMap map[string]common.EndpointInfo
	lastGetPricingTime   time.Time
	updatePricingLock    sync.Mutex

	// 缓存映射：模型名 -> 启用分组 / 计费类型
	modelEnableGroups     = make(map[string][]string)
	modelQuotaTypeMap     = make(map[string]int)
	modelEnableGroupsLock = sync.RWMutex{}
)

var (
	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	modelSupportEndpointsLock = sync.RWMutex{}
)

func GetPricing(tenantCtx context.Context) []Pricing {
	if time.Since(TenantState(tenantCtx).lastGetPricingTime) > time.Minute*1 || len(TenantState(tenantCtx).pricingMap) == 0 {
		TenantState(tenantCtx).updatePricingLock.Lock()
		defer TenantState(tenantCtx).updatePricingLock.Unlock()
		// Double check after acquiring the lock
		if time.Since(TenantState(tenantCtx).lastGetPricingTime) > time.Minute*1 || len(TenantState(tenantCtx).pricingMap) == 0 {
			TenantState(tenantCtx).modelSupportEndpointsLock.Lock()
			defer TenantState(tenantCtx).modelSupportEndpointsLock.Unlock()
			updatePricing(tenantCtx)
		}
	}
	return TenantState(tenantCtx).pricingMap
}

func InvalidatePricingCache(tenantCtx context.Context) {
	TenantState(tenantCtx).updatePricingLock.Lock()
	defer TenantState(tenantCtx).updatePricingLock.Unlock()

	TenantState(tenantCtx).pricingMap = nil
	TenantState(tenantCtx).vendorsList = nil
	TenantState(tenantCtx).lastGetPricingTime = time.Time{}
}

// GetVendors 返回当前定价接口使用到的供应商信息
func GetVendors(tenantCtx context.Context) []PricingVendor {
	if time.Since(TenantState(tenantCtx).lastGetPricingTime) > time.Minute*1 || len(TenantState(tenantCtx).pricingMap) == 0 {
		// 保证先刷新一次
		GetPricing(tenantCtx)
	}
	return TenantState(tenantCtx).vendorsList
}

func GetModelSupportEndpointTypes(tenantCtx context.Context, model string) []constant.EndpointType {
	if model == "" {
		return make([]constant.EndpointType, 0)
	}
	TenantState(tenantCtx).modelSupportEndpointsLock.RLock()
	defer TenantState(tenantCtx).modelSupportEndpointsLock.RUnlock()
	if endpoints, ok := TenantState(tenantCtx).modelSupportEndpointTypes[model]; ok {
		return endpoints
	}
	return make([]constant.EndpointType, 0)
}

func getPricingEndpointTypesForAbility(ability AbilityWithChannel, advancedCustomConfigs map[int]*dto.AdvancedCustomConfig) []constant.EndpointType {
	if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
		return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
	}
	if config := advancedCustomConfigs[ability.ChannelId]; config != nil {
		return config.SupportedEndpointTypesForModel(ability.Model)
	}
	return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
}

// loadPricingAdvancedCustomConfigs runs inside updatePricing while
// updatePricingLock is held, and nests channelSyncLock.RLock. This defines the
// global lock order updatePricingLock -> channelSyncLock: any code path holding
// channelSyncLock must release it before touching the pricing cache (see
// InitChannelCache / CacheUpdateChannel), otherwise it deadlocks.
// The returned configs are pointers shared with the channel cache; they are
// replaced wholesale on update and never mutated in place, so reading them after
// RUnlock is safe.
func loadPricingAdvancedCustomConfigs(tenantCtx context.Context, enableAbilities []AbilityWithChannel) map[int]*dto.AdvancedCustomConfig {
	channelIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, ability := range enableAbilities {
		if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if _, exists := seen[ability.ChannelId]; exists {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	if len(channelIDs) == 0 {
		return nil
	}

	configs := make(map[int]*dto.AdvancedCustomConfig, len(channelIDs))
	if common.MemoryCacheEnabled {
		TenantState(tenantCtx).channelSyncLock.RLock()
		defer TenantState(tenantCtx).channelSyncLock.RUnlock()
		for _, channelID := range channelIDs {
			if config := TenantState(tenantCtx).channel2advancedCustomConfig[channelID]; config != nil {
				configs[channelID] = config
			}
		}
		return configs
	}

	for _, channelID := range channelIDs {
		channel, err := CacheGetChannel(tenantCtx, channelID)
		if err != nil {
			common.SysLog(fmt.Sprintf("load advanced custom channel settings error: channel_id=%d, error=%v", channelID, err))
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if config := channel.GetOtherSettings(tenantCtx).AdvancedCustom; config != nil {
			configs[channelID] = config
		}
	}
	return configs
}

func appendPricingEndpoint(endpoints []string, endpoint string) []string {
	if endpoint == "" || common.StringsContains(endpoints, endpoint) {
		return endpoints
	}
	return append(endpoints, endpoint)
}

func updatePricing(tenantCtx context.Context) {
	//modelRatios := common.GetModelRatios()
	enableAbilities, err := GetAllEnableAbilityWithChannels(tenantCtx)
	if err != nil {
		common.SysLog(fmt.Sprintf("GetAllEnableAbilityWithChannels error: %v", err))
		return
	}
	// 预加载模型元数据与供应商一次，避免循环查询
	var allMeta []Model
	_ = DB.WithContext(tenantCtx).Find(&allMeta).Error
	names := make([]string, 0, len(enableAbilities))
	for _, ability := range enableAbilities {
		names = append(names, ability.Model)
	}
	metaMap := resolveModelMetadata(allMeta, names)

	// 预加载供应商
	var vendors []Vendor
	_ = DB.WithContext(tenantCtx).Find(&vendors).Error
	vendorMap := make(map[int]*Vendor)
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}

	// 初始化默认供应商映射
	initDefaultVendorMapping(metaMap, vendorMap, enableAbilities)

	// 构建对前端友好的供应商列表
	TenantState(tenantCtx).vendorsList = make([]PricingVendor, 0, len(vendorMap))
	for _, v := range vendorMap {
		TenantState(tenantCtx).vendorsList = append(TenantState(tenantCtx).vendorsList, PricingVendor{
			ID:          v.Id,
			Name:        v.Name,
			Description: v.Description,
			Icon:        v.Icon,
		})
	}

	modelGroupsMap := make(map[string]*types.Set[string])

	for _, ability := range enableAbilities {
		groups, ok := modelGroupsMap[ability.Model]
		if !ok {
			groups = types.NewSet[string]()
			modelGroupsMap[ability.Model] = groups
		}
		groups.Add(ability.Group)
	}

	//这里使用切片而不是Set，因为一个模型可能支持多个端点类型，并且第一个端点是优先使用端点
	modelSupportEndpointsStr := make(map[string][]string)
	advancedCustomConfigs := loadPricingAdvancedCustomConfigs(tenantCtx, enableAbilities)

	// 先根据已有能力填充原生端点
	for _, ability := range enableAbilities {
		endpoints := modelSupportEndpointsStr[ability.Model]
		channelTypes := getPricingEndpointTypesForAbility(ability, advancedCustomConfigs)
		for _, channelType := range channelTypes {
			if !common.StringsContains(endpoints, string(channelType)) {
				endpoints = append(endpoints, string(channelType))
			}
		}
		modelSupportEndpointsStr[ability.Model] = endpoints
	}

	// 再补充模型自定义端点：若配置有效则追加到已有推断，不再裁剪渠道真实能力
	for modelName, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]any
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			endpoints := modelSupportEndpointsStr[modelName]
			for k, v := range raw {
				switch v.(type) {
				case string, map[string]any:
					endpoints = appendPricingEndpoint(endpoints, k)
				}
			}
			if len(endpoints) > 0 {
				modelSupportEndpointsStr[modelName] = endpoints
			}
		}
	}

	TenantState(tenantCtx).modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	for model, endpoints := range modelSupportEndpointsStr {
		supportedEndpoints := make([]constant.EndpointType, 0)
		for _, endpointStr := range endpoints {
			endpointType := constant.EndpointType(endpointStr)
			supportedEndpoints = append(supportedEndpoints, endpointType)
		}
		TenantState(tenantCtx).modelSupportEndpointTypes[model] = supportedEndpoints
	}

	// 构建全局 supportedEndpointMap（默认 + 自定义覆盖）
	TenantState(tenantCtx).supportedEndpointMap = make(map[string]common.EndpointInfo)
	// 1. 默认端点
	for _, endpoints := range TenantState(tenantCtx).modelSupportEndpointTypes {
		for _, et := range endpoints {
			if info, ok := common.GetDefaultEndpointInfo(et); ok {
				if _, exists := TenantState(tenantCtx).supportedEndpointMap[string(et)]; !exists {
					TenantState(tenantCtx).supportedEndpointMap[string(et)] = info
				}
			}
		}
	}
	// 2. 自定义端点（models 表）覆盖默认
	for _, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]any
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			for k, v := range raw {
				switch val := v.(type) {
				case string:
					TenantState(tenantCtx).supportedEndpointMap[k] = common.EndpointInfo{Path: val, Method: "POST"}
				case map[string]any:
					ep := common.EndpointInfo{Method: "POST"}
					if p, ok := val["path"].(string); ok {
						ep.Path = p
					}
					if m, ok := val["method"].(string); ok {
						ep.Method = strings.ToUpper(m)
					}
					TenantState(tenantCtx).supportedEndpointMap[k] = ep
				default:
					// ignore unsupported types
				}
			}
		}
	}

	TenantState(tenantCtx).pricingMap = make([]Pricing, 0)
	pluginGeneration := jsplugin.TenantState(tenantCtx).DefaultRegistry.Generation()
	for model, groups := range modelGroupsMap {
		pricing := Pricing{
			ModelName:              model,
			EnableGroup:            groups.Items(),
			SupportedEndpointTypes: TenantState(tenantCtx).modelSupportEndpointTypes[model],
		}

		// 补充模型元数据（描述、标签、供应商、状态）
		if meta, ok := metaMap[model]; ok {
			// 若模型被禁用(status!=1)，则直接跳过，不返回给前端
			if meta.Status != 1 {
				continue
			}
			pricing.Description = meta.Description
			pricing.Icon = meta.Icon
			pricing.Tags = meta.Tags
			pricing.VendorID = meta.VendorID
		}
		modelPrice, findPrice := ratio_setting.GetModelPrice(tenantCtx, model, false)
		if findPrice {
			pricing.ModelPrice = modelPrice
			pricing.QuotaType = 1
		} else {
			modelRatio, _, _ := ratio_setting.GetModelRatio(tenantCtx, model)
			pricing.ModelRatio = modelRatio
			pricing.CompletionRatio = ratio_setting.GetCompletionRatio(tenantCtx, model)
			pricing.QuotaType = 0
		}
		if cacheRatio, ok := ratio_setting.GetCacheRatio(tenantCtx, model); ok {
			pricing.CacheRatio = &cacheRatio
		}
		if createCacheRatio, ok := ratio_setting.GetCreateCacheRatio(tenantCtx, model); ok {
			pricing.CreateCacheRatio = &createCacheRatio
		}
		if imageRatio, ok := ratio_setting.GetImageRatio(tenantCtx, model); ok {
			pricing.ImageRatio = &imageRatio
		}
		if ratio_setting.ContainsAudioRatio(tenantCtx, model) {
			audioRatio := ratio_setting.GetAudioRatio(tenantCtx, model)
			pricing.AudioRatio = &audioRatio
		}
		if ratio_setting.ContainsAudioCompletionRatio(tenantCtx, model) {
			audioCompletionRatio := ratio_setting.GetAudioCompletionRatio(tenantCtx, model)
			pricing.AudioCompletionRatio = &audioCompletionRatio
		}
		if billingMode := billing_setting.GetBillingMode(tenantCtx, model); billingMode == "tiered_expr" {
			if expr, ok := billing_setting.GetBillingExpr(tenantCtx, model); ok && strings.TrimSpace(expr) != "" {
				pricing.BillingMode = billingMode
				pricing.BillingExpr = expr
			}
		} else if target, resolved := ResolveTaskModelAlias(tenantCtx, pluginGeneration, model); resolved && target.Declared != "" {
			if tailMode := billing_setting.GetBillingMode(tenantCtx, target.Declared); tailMode == "tiered_expr" {
				if expr, ok := billing_setting.GetBillingExpr(tenantCtx, target.Declared); ok && strings.TrimSpace(expr) != "" {
					pricing.BillingMode = tailMode
					pricing.BillingExpr = expr
				}
			}
		}
		usageModel := model
		plugin, ok := pluginGeneration.GetByModel(model)
		if !ok {
			if target, resolved := ResolveTaskModelAlias(tenantCtx, pluginGeneration, model); resolved {
				plugin, ok = pluginGeneration.Get(target.PluginKey)
				usageModel = target.Declared
			}
		}
		if ok && plugin != nil {
			usageSchema, usageExamples := plugin.Meta.UsageForModel(usageModel)
			pricing.BillingUsageSchema = jsplugin.CloneUsageSchema(usageSchema)
			pricing.BillingUsageExamples = jsplugin.CloneUsageExamples(usageExamples)
		}
		providers := pluginGeneration.PluginsByModel(model)
		hasProviderOverride := false
		for _, provider := range providers {
			if _, configured := billing_setting.GetPluginBillingExpr(tenantCtx, provider.Meta.Key, model); configured {
				hasProviderOverride = true
				break
			}
		}
		if hasProviderOverride || (len(providers) >= 2 && pricing.BillingMode == billing_setting.BillingModeTieredExpr) {
			for _, provider := range providers {
				schema, examples := provider.Meta.UsageForModel(model)
				if schema == nil {
					schema = map[string]jsplugin.UsageFieldSchema{}
				}
				expression, hasExpression := billing_setting.ResolveTaskBillingExpr(tenantCtx, provider.Meta.Key, model, "")
				mode := billing_setting.BillingModeRatio
				if hasExpression || billing_setting.GetBillingMode(tenantCtx, model) == billing_setting.BillingModeTieredExpr {
					mode = billing_setting.BillingModeTieredExpr
				}
				if mode == billing_setting.BillingModeTieredExpr && !billing_setting.TaskExprCompatible(expression, schema) {
					expression = ""
				}
				pricing.BillingPluginVariants = append(pricing.BillingPluginVariants, PricingPluginVariant{
					PluginKey: provider.Meta.Key, PluginName: provider.Meta.Name, Icon: provider.Meta.Icon,
					BillingExpr: expression, BillingMode: mode,
					BillingUsageSchema: jsplugin.CloneUsageSchema(schema), BillingUsageExamples: jsplugin.CloneUsageExamples(examples),
				})
			}
		}
		TenantState(tenantCtx).pricingMap = append(TenantState(tenantCtx).pricingMap, pricing)
	}

	// 防止大更新后数据不通用
	if len(TenantState(tenantCtx).pricingMap) > 0 {
		TenantState(tenantCtx).pricingMap[0].PricingVersion = "5a90f2b86c08bd983a9a2e6d66c255f4eaef9c4bc934386d2b6ae84ef0ff1f1f"
	}

	// 刷新缓存映射，供高并发快速查询
	TenantState(tenantCtx).modelEnableGroupsLock.Lock()
	TenantState(tenantCtx).modelEnableGroups = make(map[string][]string)
	TenantState(tenantCtx).modelQuotaTypeMap = make(map[string]int)
	for _, p := range TenantState(tenantCtx).pricingMap {
		TenantState(tenantCtx).modelEnableGroups[p.ModelName] = p.EnableGroup
		TenantState(tenantCtx).modelQuotaTypeMap[p.ModelName] = p.QuotaType
	}
	TenantState(tenantCtx).modelEnableGroupsLock.Unlock()

	TenantState(tenantCtx).lastGetPricingTime = time.Now()
}

// GetSupportedEndpointMap 返回全局端点到路径的映射
func GetSupportedEndpointMap(tenantCtx context.Context) map[string]common.EndpointInfo {
	return TenantState(tenantCtx).supportedEndpointMap
}
