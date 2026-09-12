package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

func TestStatus(c *gin.Context) {
	err := model.PingDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "数据库连接失败",
		})
		return
	}
	// 获取HTTP统计信息
	httpStats := middleware.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Server is running",
		"http_stats": httpStats,
	})
	return
}

func GetStatus(c *gin.Context) {

	cs := console_setting.GetConsoleSetting(c.Request.Context())
	passkeySetting := system_setting.PasskeySettingsSnapshot(c.Request.Context())
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RLock()
	defer common.TenantState(c.Request.Context()).OptionMapRWMutex.RUnlock()

	legalSetting := system_setting.GetLegalSettings(c.Request.Context())

	data := gin.H{
		"version":                     common.Version,
		"start_time":                  common.StartTime,
		"email_verification":          common.TenantState(c.Request.Context()).EmailVerificationEnabled,
		"github_oauth":                common.TenantState(c.Request.Context()).GitHubOAuthEnabled,
		"github_client_id":            common.TenantState(c.Request.Context()).GitHubClientId,
		"discord_oauth":               system_setting.GetDiscordSettings(c.Request.Context()).Enabled,
		"discord_client_id":           system_setting.GetDiscordSettings(c.Request.Context()).ClientId,
		"linuxdo_oauth":               common.TenantState(c.Request.Context()).LinuxDOOAuthEnabled,
		"linuxdo_client_id":           common.TenantState(c.Request.Context()).LinuxDOClientId,
		"linuxdo_minimum_trust_level": common.TenantState(c.Request.Context()).LinuxDOMinimumTrustLevel,
		"telegram_oauth":              common.TenantState(c.Request.Context()).TelegramOAuthEnabled,
		"telegram_oauth_configured":   oauth.TelegramConfigurationError(c.Request.Context()) == nil,
		"telegram_bot_name":           common.TenantState(c.Request.Context()).TelegramBotName,
		"theme":                       "default",
		"system_name":                 common.TenantState(c.Request.Context()).SystemName,
		"logo":                        common.TenantState(c.Request.Context()).Logo,
		"footer_html":                 common.TenantState(c.Request.Context()).Footer,
		"platform_footer":             plan.PlatformFooter,
		"platform_footer_locked":      true,
		"wechat_qrcode":               common.TenantState(c.Request.Context()).WeChatAccountQRCodeImageURL,
		"wechat_login":                common.TenantState(c.Request.Context()).WeChatAuthEnabled,
		"server_address":              system_setting.TenantState(c.Request.Context()).ServerAddress,
		"turnstile_check":             common.TenantState(c.Request.Context()).TurnstileCheckEnabled,
		"turnstile_site_key":          common.TenantState(c.Request.Context()).TurnstileSiteKey,
		"docs_link":                   operation_setting.GetGeneralSetting(c.Request.Context()).DocsLink,
		"quota_per_unit":              common.TenantState(c.Request.Context()).QuotaPerUnit,
		// 兼容旧前端：保留 display_in_currency，同时提供新的 quota_display_type
		"display_in_currency":           operation_setting.IsCurrencyDisplay(c.Request.Context()),
		"quota_display_type":            operation_setting.GetQuotaDisplayType(c.Request.Context()),
		"custom_currency_symbol":        operation_setting.GetGeneralSetting(c.Request.Context()).CustomCurrencySymbol,
		"custom_currency_exchange_rate": operation_setting.GetGeneralSetting(c.Request.Context()).CustomCurrencyExchangeRate,
		"enable_batch_update":           common.BatchUpdateEnabled,
		"enable_drawing":                common.TenantState(c.Request.Context()).DrawingEnabled,
		"enable_task":                   common.TenantState(c.Request.Context()).TaskEnabled,
		"enable_data_export":            common.TenantState(c.Request.Context()).DataExportEnabled,
		"data_export_default_time":      common.TenantState(c.Request.Context()).DataExportDefaultTime,
		"default_collapse_sidebar":      common.TenantState(c.Request.Context()).DefaultCollapseSidebar,
		"mj_notify_enabled":             setting.TenantState(c.Request.Context()).MjNotifyEnabled,
		"chats":                         setting.TenantState(c.Request.Context()).Chats,
		"demo_site_enabled":             operation_setting.TenantState(c.Request.Context()).DemoSiteEnabled,
		"self_use_mode_enabled":         operation_setting.TenantState(c.Request.Context()).SelfUseModeEnabled,
		"register_enabled":              common.TenantState(c.Request.Context()).RegisterEnabled,
		"password_login_enabled":        common.TenantState(c.Request.Context()).PasswordLoginEnabled,
		"password_register_enabled":     common.TenantState(c.Request.Context()).PasswordRegisterEnabled,
		"default_use_auto_group":        setting.TenantRuntime(c.Request.Context()).DefaultUseAutoGroup,

		"password_login_encryption_enabled": common.PasswordLoginEncryptionEnabled,

		"usd_exchange_rate": operation_setting.TenantState(c.Request.Context()).USDExchangeRate,
		"price":             operation_setting.TenantState(c.Request.Context()).Price,
		"stripe_unit_price": setting.TenantState(c.Request.Context()).StripeUnitPrice,

		// 面板启用开关
		"api_info_enabled":      cs.ApiInfoEnabled,
		"uptime_kuma_enabled":   cs.UptimeKumaEnabled,
		"announcements_enabled": cs.AnnouncementsEnabled,
		"faq_enabled":           cs.FAQEnabled,

		// 模块管理配置
		"HeaderNavModules":    common.TenantState(c.Request.Context()).OptionMap["HeaderNavModules"],
		"SidebarModulesAdmin": common.TenantState(c.Request.Context()).OptionMap["SidebarModulesAdmin"],

		"oidc_enabled":                system_setting.GetOIDCSettings(c.Request.Context()).Enabled,
		"oidc_client_id":              system_setting.GetOIDCSettings(c.Request.Context()).ClientId,
		"oidc_authorization_endpoint": system_setting.GetOIDCSettings(c.Request.Context()).AuthorizationEndpoint,
		"oidc_display_name":           system_setting.GetOIDCSettings(c.Request.Context()).GetEffectiveDisplayName(),
		"passkey_login":               passkeySetting.Enabled,
		"passkey_display_name":        passkeySetting.RPDisplayName,
		"passkey_rp_id":               passkeySetting.EffectiveRPID(),
		"passkey_rp_ids":              passkeySetting.RelyingPartyIDs(),
		"passkey_origins":             passkeySetting.Origins,
		"passkey_allow_insecure":      passkeySetting.AllowInsecureOrigin,
		"passkey_user_verification":   passkeySetting.UserVerification,
		"passkey_attachment":          passkeySetting.AttachmentPreference,
		"setup":                       true,
		"user_agreement_enabled":      legalSetting.UserAgreement != "",
		"privacy_policy_enabled":      legalSetting.PrivacyPolicy != "",
		"checkin_enabled":             operation_setting.GetCheckinSetting(c.Request.Context()).Enabled,
	}

	if view, ok := plan.FromContext(c.Request.Context()); ok {
		data["platform_footer_locked"] = !view.Capabilities.RemovePlatformFooter
		data["platform_branding_locked"] = !view.Capabilities.CustomBranding
		if !view.Capabilities.CustomBranding {
			data["system_name"] = "New API"
			data["logo"] = ""
		}
		if !view.Capabilities.RemovePlatformFooter {
			data["footer_html"] = ""
		}
	}
	// 根据启用状态注入可选内容
	if cs.ApiInfoEnabled {
		data["api_info"] = console_setting.GetApiInfo(c.Request.Context())
	}
	if cs.AnnouncementsEnabled {
		data["announcements"] = console_setting.GetAnnouncements(c.Request.Context())
	}
	if cs.FAQEnabled {
		data["faq"] = console_setting.GetFAQ(c.Request.Context())
	}

	// Add enabled custom OAuth providers
	customProviders := oauth.GetEnabledCustomProviders(c.Request.Context())
	if len(customProviders) > 0 {
		type CustomOAuthInfo struct {
			Id                    int    `json:"id"`
			Name                  string `json:"name"`
			Slug                  string `json:"slug"`
			Icon                  string `json:"icon"`
			ClientId              string `json:"client_id"`
			AuthorizationEndpoint string `json:"authorization_endpoint"`
			Scopes                string `json:"scopes"`
		}
		providersInfo := make([]CustomOAuthInfo, 0, len(customProviders))
		for _, p := range customProviders {
			config := p.GetConfig()
			providersInfo = append(providersInfo, CustomOAuthInfo{
				Id:                    config.Id,
				Name:                  config.Name,
				Slug:                  config.Slug,
				Icon:                  config.Icon,
				ClientId:              config.ClientId,
				AuthorizationEndpoint: config.AuthorizationEndpoint,
				Scopes:                config.Scopes,
			})
		}
		data["custom_oauth_providers"] = providersInfo
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
	return
}

func GetNotice(c *gin.Context) {
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RLock()
	notice := common.TenantState(c.Request.Context()).OptionMap["Notice"]
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RUnlock()
	serveRevalidatedJSON(c, notice)
}

func GetAbout(c *gin.Context) {
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RLock()
	about := common.TenantState(c.Request.Context()).OptionMap["About"]
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RUnlock()
	serveRevalidatedJSON(c, about)
}

func GetUserAgreement(c *gin.Context) {
	serveRevalidatedJSON(c, system_setting.GetLegalSettings(c.Request.Context()).UserAgreement)
}

func GetPrivacyPolicy(c *gin.Context) {
	serveRevalidatedJSON(c, system_setting.GetLegalSettings(c.Request.Context()).PrivacyPolicy)
}

func GetMidjourney(c *gin.Context) {
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RLock()
	defer common.TenantState(c.Request.Context()).OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.TenantState(c.Request.Context()).OptionMap["Midjourney"],
	})
	return
}

func GetHomePageContent(c *gin.Context) {
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RLock()
	homePageContent := common.TenantState(c.Request.Context()).OptionMap["HomePageContent"]
	common.TenantState(c.Request.Context()).OptionMapRWMutex.RUnlock()
	serveRevalidatedJSON(c, homePageContent)
}

func SendEmailVerification(c *gin.Context) {
	email, err := service.ValidateAccountEmail(c.Request.Context(), c.Query("email"))
	if err != nil {
		writeSecurityOperationError(c, err)
		return
	}

	if model.IsEmailAlreadyTaken(c.Request.Context(), email) {
		common.ApiErrorI18n(c, i18n.MsgUserEmailAlreadyTaken)
		return
	}
	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(c.Request.Context(), email, code, common.EmailVerificationPurpose)
	subject := fmt.Sprintf("%s邮箱验证邮件", common.TenantState(c.Request.Context()).SystemName)
	content := fmt.Sprintf("<p>您好，你正在进行%s邮箱验证。</p>"+
		"<p>您的验证码为: <strong>%s</strong></p>"+
		"<p>验证码 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.TenantState(c.Request.Context()).SystemName, code, common.VerificationValidMinutes)
	err = common.SendEmail(c.Request.Context(), subject, email, content)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func SendPasswordResetEmail(c *gin.Context) {
	email := model.NormalizeEmail(c.Query("email"))
	if err := common.Validate.Var(email, "required,email"); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if _, err := model.GetUniqueUserByEmail(c.Request.Context(), email); err == nil {
		code := common.GenerateVerificationCode(0)
		common.RegisterVerificationCodeWithKey(c.Request.Context(), email, code, common.PasswordResetPurpose)
		link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", system_setting.TenantState(c.Request.Context()).ServerAddress, email, code)
		subject := fmt.Sprintf("%s密码重置", common.TenantState(c.Request.Context()).SystemName)
		content := fmt.Sprintf("<p>您好，你正在进行%s密码重置。</p>"+
			"<p>点击 <a href='%s'>此处</a> 进行密码重置。</p>"+
			"<p>如果链接无法点击，请尝试点击下面的链接或将其复制到浏览器中打开：<br> %s </p>"+
			"<p>重置链接 %d 分钟内有效，如果不是本人操作，请忽略。</p>", common.TenantState(c.Request.Context()).SystemName, link, link, common.VerificationValidMinutes)
		err := common.SendEmail(c.Request.Context(), subject, email, content)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send password reset email to %s: %s", email, err.Error()))
		}
	} else if err != nil && !errors.Is(err, model.ErrEmailNotFound) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("skip password reset email for %s: %s", email, err.Error()))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func ResetPassword(c *gin.Context) {
	var req PasswordResetRequest
	err := common.DecodeJson(c.Request.Body, &req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	req.Email = model.NormalizeEmail(req.Email)
	if req.Email == "" || req.Token == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if !common.VerifyCodeWithKey(c.Request.Context(), req.Email, req.Token, common.PasswordResetPurpose) {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordResetLinkInvalid)
		return
	}
	password := common.GenerateVerificationCode(12)
	err = model.ResetUserPasswordByEmail(c.Request.Context(), req.Email, password)
	if err != nil {
		if errors.Is(err, model.ErrEmailNotFound) || errors.Is(err, model.ErrEmailAmbiguous) {
			common.ApiErrorI18n(c, i18n.MsgUserPasswordResetLinkInvalid)
			return
		}
		common.ApiError(c, err)
		return
	}
	common.DeleteKey(c.Request.Context(), req.Email, common.PasswordResetPurpose)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
	return
}
