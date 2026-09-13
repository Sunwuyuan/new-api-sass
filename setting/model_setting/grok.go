package model_setting

import context "context"

import "github.com/QuantumNous/new-api/setting/config"

// GrokSettings defines Grok model configuration.
type GrokSettings struct {
	ViolationDeductionEnabled bool    `json:"violation_deduction_enabled"`
	ViolationDeductionAmount  float64 `json:"violation_deduction_amount"`
}

var defaultGrokSettings = GrokSettings{
	ViolationDeductionEnabled: true,
	ViolationDeductionAmount:  0.05,
}

var grokSettings = defaultGrokSettings

func init() {
	config.GlobalConfig.Register("grok", &grokSettings)
}

func GetGrokSettings(tenantCtx context.Context) *GrokSettings {
	return config.GlobalConfig.ForTenant(tenantCtx).Get("grok").(*GrokSettings)
}
