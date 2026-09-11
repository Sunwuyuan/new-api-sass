package service

import context "context"

import (
	"fmt"
	"html"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

func UnbindAccountOAuth(tenantCtx context.Context, identity AuthIdentity, providerID int) error {
	enabled := model.AccountLoginMethods{
		Password: common.TenantState(tenantCtx).PasswordLoginEnabled,
		Passkey:  system_setting.PasskeySettingsSnapshot(tenantCtx).Enabled,
		WeChat:   common.TenantState(tenantCtx).WeChatAuthEnabled,
	}
	for _, provider := range oauth.GetAllProviders(tenantCtx) {
		if !provider.IsEnabled(tenantCtx) {
			continue
		}
		if custom, ok := provider.(*oauth.GenericOAuthProvider); ok {
			enabled.CustomProviderIDs = append(enabled.CustomProviderIDs, custom.GetProviderId())
		} else {
			enabled.OAuthColumns = append(enabled.OAuthColumns, provider.ProviderUserIDColumn())
		}
	}
	return model.UnbindUserOAuthForSession(tenantCtx, identity, providerID, enabled)
}

// NotifyAccountSecurityChange never includes credentials or tokens. The caller
// records delivery failure independently from the already-committed change.
func NotifyAccountSecurityChange(tenantCtx context.Context, email, event string) error {
	if email == "" {
		return nil
	}
	subject := common.TenantState(tenantCtx).SystemName + " — Account security notification"
	content := fmt.Sprintf("<p>Your account security settings have changed: %s.</p><p>If you did not make this change, open your account security settings, revoke other login sessions, and contact your administrator.</p>", html.EscapeString(event))
	return common.SendEmail(tenantCtx, subject, email, content)
}
