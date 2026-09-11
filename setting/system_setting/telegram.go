package system_setting

import context "context"

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type TelegramSettings struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

var telegramSettings TelegramSettings

func init() {
	config.GlobalConfig.Register("telegram", &telegramSettings)
}

func GetTelegramSettings(tenantCtx context.Context) *TelegramSettings {
	return config.GlobalConfig.ForTenant(tenantCtx).Get("telegram").(*TelegramSettings)
}

func (s *TelegramSettings) IsConfigured() bool {
	return strings.TrimSpace(s.ClientID) != "" && strings.TrimSpace(s.ClientSecret) != ""
}
