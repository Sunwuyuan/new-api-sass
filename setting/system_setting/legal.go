package system_setting

import context "context"

import "github.com/QuantumNous/new-api/setting/config"

type LegalSettings struct {
	UserAgreement string `json:"user_agreement"`
	PrivacyPolicy string `json:"privacy_policy"`
}

var defaultLegalSettings = LegalSettings{
	UserAgreement: "",
	PrivacyPolicy: "",
}

func init() {
	config.GlobalConfig.Register("legal", &defaultLegalSettings)
}

func GetLegalSettings(tenantCtx context.Context) *LegalSettings {
	return config.GlobalConfig.ForTenant(tenantCtx).Get("legal").(*LegalSettings)
}
