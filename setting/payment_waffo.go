package setting

import context "context"

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

var (
	WaffoEnabled               bool
	WaffoApiKey                string
	WaffoPrivateKey            string
	WaffoPublicCert            string
	WaffoSandboxPublicCert     string
	WaffoSandboxApiKey         string
	WaffoSandboxPrivateKey     string
	WaffoSandbox               bool
	WaffoMerchantId            string
	WaffoNotifyUrl             string
	WaffoReturnUrl             string
	WaffoSubscriptionReturnUrl string
	WaffoCurrency              string
	WaffoUnitPrice             float64 = 1.0
	WaffoMinTopUp              int     = 1
)

// GetWaffoPayMethods 从 options 读取 Waffo 支付方式配置
func GetWaffoPayMethods(tenantCtx context.Context) []constant.WaffoPayMethod {
	common.TenantState(tenantCtx).OptionMapRWMutex.RLock()
	jsonStr := common.TenantState(tenantCtx).OptionMap["WaffoPayMethods"]
	common.TenantState(tenantCtx).OptionMapRWMutex.RUnlock()

	if jsonStr == "" {
		return copyDefaultWaffoPayMethods()
	}
	var methods []constant.WaffoPayMethod
	if err := common.UnmarshalJsonStr(jsonStr, &methods); err != nil {
		return copyDefaultWaffoPayMethods()
	}
	return methods
}

// SetWaffoPayMethods 序列化 Waffo 支付方式配置并更新 OptionMap
func SetWaffoPayMethods(tenantCtx context.Context, methods []constant.WaffoPayMethod) error {
	jsonBytes, err := common.Marshal(methods)
	if err != nil {
		return err
	}
	common.TenantState(tenantCtx).OptionMapRWMutex.Lock()
	common.TenantState(tenantCtx).OptionMap["WaffoPayMethods"] = string(jsonBytes)
	common.TenantState(tenantCtx).OptionMapRWMutex.Unlock()
	return nil
}

func copyDefaultWaffoPayMethods() []constant.WaffoPayMethod {
	cp := make([]constant.WaffoPayMethod, len(constant.DefaultWaffoPayMethods))
	copy(cp, constant.DefaultWaffoPayMethods)
	return cp
}

// WaffoPayMethods2JsonString 将默认 WaffoPayMethods 序列化为 JSON 字符串（供 InitOptionMap 使用）
func WaffoPayMethods2JsonString() string {
	jsonBytes, err := common.Marshal(constant.DefaultWaffoPayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}
