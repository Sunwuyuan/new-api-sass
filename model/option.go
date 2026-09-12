package model

import context "context"

import (
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/plan"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

type Option struct {
	TenantID int64  `json:"-" gorm:"primaryKey;autoIncrement:false"`
	Key      string `json:"key" gorm:"primaryKey"`
	Value    string `json:"value"`
}

func AllOption(tenantCtx context.Context) ([]*Option, error) {
	var options []*Option
	var err error
	err = DB.WithContext(tenantCtx).Find(&options).Error
	return options, err
}

func InitOptionMap(tenantCtx context.Context) {
	common.TenantState(tenantCtx).OptionMapRWMutex.Lock()
	common.TenantState(tenantCtx).OptionMap = make(map[string]string)

	// 添加原有的系统配置
	common.TenantState(tenantCtx).OptionMap["FileUploadPermission"] = strconv.Itoa(common.TenantState(tenantCtx).FileUploadPermission)
	common.TenantState(tenantCtx).OptionMap["FileDownloadPermission"] = strconv.Itoa(common.TenantState(tenantCtx).FileDownloadPermission)
	common.TenantState(tenantCtx).OptionMap["ImageUploadPermission"] = strconv.Itoa(common.TenantState(tenantCtx).ImageUploadPermission)
	common.TenantState(tenantCtx).OptionMap["ImageDownloadPermission"] = strconv.Itoa(common.TenantState(tenantCtx).ImageDownloadPermission)
	common.TenantState(tenantCtx).OptionMap["PasswordLoginEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).PasswordLoginEnabled)
	common.TenantState(tenantCtx).OptionMap["PasswordRegisterEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).PasswordRegisterEnabled)
	common.TenantState(tenantCtx).OptionMap["EmailVerificationEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).EmailVerificationEnabled)
	common.TenantState(tenantCtx).OptionMap["GitHubOAuthEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).GitHubOAuthEnabled)
	common.TenantState(tenantCtx).OptionMap["LinuxDOOAuthEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).LinuxDOOAuthEnabled)
	common.TenantState(tenantCtx).OptionMap["TelegramOAuthEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).TelegramOAuthEnabled)
	common.TenantState(tenantCtx).OptionMap["WeChatAuthEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).WeChatAuthEnabled)
	common.TenantState(tenantCtx).OptionMap["TurnstileCheckEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).TurnstileCheckEnabled)
	common.TenantState(tenantCtx).OptionMap["RegisterEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).RegisterEnabled)
	common.TenantState(tenantCtx).OptionMap["AutomaticDisableChannelEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).AutomaticDisableChannelEnabled)
	common.TenantState(tenantCtx).OptionMap["AutomaticEnableChannelEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).AutomaticEnableChannelEnabled)
	common.TenantState(tenantCtx).OptionMap["LogConsumeEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).LogConsumeEnabled)
	common.TenantState(tenantCtx).OptionMap["PlatformMailEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).PlatformMailEnabled)
	common.TenantState(tenantCtx).OptionMap["DisplayInCurrencyEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).DisplayInCurrencyEnabled)
	common.TenantState(tenantCtx).OptionMap["DisplayTokenStatEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).DisplayTokenStatEnabled)
	common.TenantState(tenantCtx).OptionMap["DrawingEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).DrawingEnabled)
	common.TenantState(tenantCtx).OptionMap["TaskEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).TaskEnabled)
	common.TenantState(tenantCtx).OptionMap["TaskPluginEnabled"] = strconv.FormatBool(constant.TenantRuntime(tenantCtx).TaskPluginEnabled)
	jsplugin.TenantState(tenantCtx).DefaultRegistry.SetEnabled(constant.TenantRuntime(tenantCtx).TaskPluginEnabled)
	common.TenantState(tenantCtx).OptionMap[setting.TaskPluginMarketplaceSourcesKey] = setting.TaskPluginMarketplaceSources2JsonString()
	common.TenantState(tenantCtx).OptionMap[setting.TaskPluginDisabledFactoryKeysKey] = "[]"
	jsplugin.TenantState(tenantCtx).DefaultRegistry.SetDisabledFactoryKeys(nil)
	common.TenantState(tenantCtx).OptionMap["DataExportEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).DataExportEnabled)
	common.TenantState(tenantCtx).OptionMap["ChannelDisableThreshold"] = strconv.FormatFloat(common.TenantState(tenantCtx).ChannelDisableThreshold, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["EmailDomainRestrictionEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).EmailDomainRestrictionEnabled)
	common.TenantState(tenantCtx).OptionMap["EmailAliasRestrictionEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).EmailAliasRestrictionEnabled)
	common.TenantState(tenantCtx).OptionMap["EmailDomainWhitelist"] = strings.Join(common.TenantState(tenantCtx).EmailDomainWhitelist, ",")
	common.TenantState(tenantCtx).OptionMap["SMTPServer"] = ""
	common.TenantState(tenantCtx).OptionMap["SMTPFrom"] = ""
	common.TenantState(tenantCtx).OptionMap["SMTPPort"] = strconv.Itoa(common.TenantState(tenantCtx).SMTPPort)
	common.TenantState(tenantCtx).OptionMap["SMTPAccount"] = ""
	common.TenantState(tenantCtx).OptionMap["SMTPToken"] = ""
	common.TenantState(tenantCtx).OptionMap["SMTPSSLEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).SMTPSSLEnabled)
	common.TenantState(tenantCtx).OptionMap["SMTPStartTLSEnabled"] = strconv.FormatBool(common.TenantState(tenantCtx).SMTPStartTLSEnabled)
	common.TenantState(tenantCtx).OptionMap["SMTPInsecureSkipVerify"] = strconv.FormatBool(common.TenantState(tenantCtx).SMTPInsecureSkipVerify)
	common.TenantState(tenantCtx).OptionMap["SMTPForceAuthLogin"] = strconv.FormatBool(common.TenantState(tenantCtx).SMTPForceAuthLogin)
	common.TenantState(tenantCtx).OptionMap["Notice"] = ""
	common.TenantState(tenantCtx).OptionMap["About"] = ""
	common.TenantState(tenantCtx).OptionMap["HomePageContent"] = ""
	common.TenantState(tenantCtx).OptionMap["Footer"] = common.TenantState(tenantCtx).Footer
	common.TenantState(tenantCtx).OptionMap["SystemName"] = common.TenantState(tenantCtx).SystemName
	common.TenantState(tenantCtx).OptionMap["Logo"] = common.TenantState(tenantCtx).Logo
	common.TenantState(tenantCtx).OptionMap["ServerAddress"] = ""
	common.TenantState(tenantCtx).OptionMap["TaskPublicAddress"] = system_setting.TenantState(tenantCtx).TaskPublicAddress
	common.TenantState(tenantCtx).OptionMap["WorkerUrl"] = system_setting.TenantState(tenantCtx).WorkerUrl
	common.TenantState(tenantCtx).OptionMap["WorkerValidKey"] = system_setting.TenantState(tenantCtx).WorkerValidKey
	common.TenantState(tenantCtx).OptionMap["WorkerAllowHttpImageRequestEnabled"] = strconv.FormatBool(system_setting.TenantState(tenantCtx).WorkerAllowHttpImageRequestEnabled)
	common.TenantState(tenantCtx).OptionMap["PayAddress"] = ""
	common.TenantState(tenantCtx).OptionMap["CustomCallbackAddress"] = ""
	common.TenantState(tenantCtx).OptionMap["EpayId"] = ""
	common.TenantState(tenantCtx).OptionMap["EpayKey"] = ""
	common.TenantState(tenantCtx).OptionMap["Price"] = strconv.FormatFloat(operation_setting.TenantState(tenantCtx).Price, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["USDExchangeRate"] = strconv.FormatFloat(operation_setting.TenantState(tenantCtx).USDExchangeRate, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["MinTopUp"] = strconv.Itoa(operation_setting.TenantState(tenantCtx).MinTopUp)
	common.TenantState(tenantCtx).OptionMap["StripeMinTopUp"] = strconv.Itoa(setting.TenantState(tenantCtx).StripeMinTopUp)
	common.TenantState(tenantCtx).OptionMap["StripeApiSecret"] = setting.TenantState(tenantCtx).StripeApiSecret
	common.TenantState(tenantCtx).OptionMap["StripeWebhookSecret"] = setting.TenantState(tenantCtx).StripeWebhookSecret
	common.TenantState(tenantCtx).OptionMap["StripePriceId"] = setting.TenantState(tenantCtx).StripePriceId
	common.TenantState(tenantCtx).OptionMap["StripeUnitPrice"] = strconv.FormatFloat(setting.TenantState(tenantCtx).StripeUnitPrice, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["StripePromotionCodesEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).StripePromotionCodesEnabled)
	common.TenantState(tenantCtx).OptionMap["CreemApiKey"] = setting.TenantState(tenantCtx).CreemApiKey
	common.TenantState(tenantCtx).OptionMap["CreemProducts"] = setting.TenantState(tenantCtx).CreemProducts
	common.TenantState(tenantCtx).OptionMap["CreemTestMode"] = strconv.FormatBool(setting.TenantState(tenantCtx).CreemTestMode)
	common.TenantState(tenantCtx).OptionMap["CreemWebhookSecret"] = setting.TenantState(tenantCtx).CreemWebhookSecret
	common.TenantState(tenantCtx).OptionMap["WaffoEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).WaffoEnabled)
	common.TenantState(tenantCtx).OptionMap["WaffoApiKey"] = setting.TenantState(tenantCtx).WaffoApiKey
	common.TenantState(tenantCtx).OptionMap["WaffoPrivateKey"] = setting.TenantState(tenantCtx).WaffoPrivateKey
	common.TenantState(tenantCtx).OptionMap["WaffoPublicCert"] = setting.TenantState(tenantCtx).WaffoPublicCert
	common.TenantState(tenantCtx).OptionMap["WaffoSandboxPublicCert"] = setting.TenantState(tenantCtx).WaffoSandboxPublicCert
	common.TenantState(tenantCtx).OptionMap["WaffoSandboxApiKey"] = setting.TenantState(tenantCtx).WaffoSandboxApiKey
	common.TenantState(tenantCtx).OptionMap["WaffoSandboxPrivateKey"] = setting.TenantState(tenantCtx).WaffoSandboxPrivateKey
	common.TenantState(tenantCtx).OptionMap["WaffoSandbox"] = strconv.FormatBool(setting.TenantState(tenantCtx).WaffoSandbox)
	common.TenantState(tenantCtx).OptionMap["WaffoMerchantId"] = setting.TenantState(tenantCtx).WaffoMerchantId
	common.TenantState(tenantCtx).OptionMap["WaffoNotifyUrl"] = setting.TenantState(tenantCtx).WaffoNotifyUrl
	common.TenantState(tenantCtx).OptionMap["WaffoReturnUrl"] = setting.TenantState(tenantCtx).WaffoReturnUrl
	common.TenantState(tenantCtx).OptionMap["WaffoSubscriptionReturnUrl"] = setting.TenantState(tenantCtx).WaffoSubscriptionReturnUrl
	common.TenantState(tenantCtx).OptionMap["WaffoCurrency"] = setting.TenantState(tenantCtx).WaffoCurrency
	common.TenantState(tenantCtx).OptionMap["WaffoUnitPrice"] = strconv.FormatFloat(setting.TenantState(tenantCtx).WaffoUnitPrice, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["WaffoMinTopUp"] = strconv.Itoa(setting.TenantState(tenantCtx).WaffoMinTopUp)
	common.TenantState(tenantCtx).OptionMap["WaffoPayMethods"] = setting.WaffoPayMethods2JsonString()
	common.TenantState(tenantCtx).OptionMap["WaffoPancakeMerchantID"] = setting.TenantState(tenantCtx).WaffoPancakeMerchantID
	common.TenantState(tenantCtx).OptionMap["WaffoPancakePrivateKey"] = setting.TenantState(tenantCtx).WaffoPancakePrivateKey
	common.TenantState(tenantCtx).OptionMap["WaffoPancakeReturnURL"] = setting.TenantState(tenantCtx).WaffoPancakeReturnURL
	common.TenantState(tenantCtx).OptionMap["WaffoPancakeUnitPrice"] = strconv.FormatFloat(setting.TenantState(tenantCtx).WaffoPancakeUnitPrice, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["WaffoPancakeMinTopUp"] = strconv.Itoa(setting.TenantState(tenantCtx).WaffoPancakeMinTopUp)
	common.TenantState(tenantCtx).OptionMap["WaffoPancakeStoreID"] = setting.TenantState(tenantCtx).WaffoPancakeStoreID
	common.TenantState(tenantCtx).OptionMap["WaffoPancakeProductID"] = setting.TenantState(tenantCtx).WaffoPancakeProductID
	common.TenantState(tenantCtx).OptionMap["TopupGroupRatio"] = common.TopupGroupRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["Chats"] = setting.Chats2JsonString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["AutoGroups"] = setting.AutoGroups2JsonString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["DefaultUseAutoGroup"] = strconv.FormatBool(setting.TenantRuntime(tenantCtx).DefaultUseAutoGroup)
	common.TenantState(tenantCtx).OptionMap["MaxTokenAutoGroups"] = strconv.Itoa(setting.GetMaxTokenAutoGroups(tenantCtx))
	common.TenantState(tenantCtx).OptionMap["PayMethods"] = operation_setting.PayMethods2JsonString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["GitHubClientId"] = ""
	common.TenantState(tenantCtx).OptionMap["GitHubClientSecret"] = ""
	common.TenantState(tenantCtx).OptionMap["TelegramBotToken"] = ""
	common.TenantState(tenantCtx).OptionMap["TelegramBotName"] = ""
	common.TenantState(tenantCtx).OptionMap["WeChatServerAddress"] = ""
	common.TenantState(tenantCtx).OptionMap["WeChatServerToken"] = ""
	common.TenantState(tenantCtx).OptionMap["WeChatAccountQRCodeImageURL"] = ""
	common.TenantState(tenantCtx).OptionMap["TurnstileSiteKey"] = ""
	common.TenantState(tenantCtx).OptionMap["TurnstileSecretKey"] = ""
	common.TenantState(tenantCtx).OptionMap["QuotaForNewUser"] = strconv.Itoa(common.TenantState(tenantCtx).QuotaForNewUser)
	common.TenantState(tenantCtx).OptionMap["QuotaForInviter"] = strconv.Itoa(common.TenantState(tenantCtx).QuotaForInviter)
	common.TenantState(tenantCtx).OptionMap["QuotaForInvitee"] = strconv.Itoa(common.TenantState(tenantCtx).QuotaForInvitee)
	common.TenantState(tenantCtx).OptionMap["QuotaRemindThreshold"] = strconv.Itoa(common.TenantState(tenantCtx).QuotaRemindThreshold)
	common.TenantState(tenantCtx).OptionMap["PreConsumedQuota"] = strconv.Itoa(common.TenantState(tenantCtx).PreConsumedQuota)
	common.TenantState(tenantCtx).OptionMap["ModelRequestRateLimitCount"] = strconv.Itoa(setting.TenantState(tenantCtx).ModelRequestRateLimitCount)
	common.TenantState(tenantCtx).OptionMap["ModelRequestRateLimitDurationMinutes"] = strconv.Itoa(setting.TenantState(tenantCtx).ModelRequestRateLimitDurationMinutes)
	common.TenantState(tenantCtx).OptionMap["ModelRequestRateLimitSuccessCount"] = strconv.Itoa(setting.TenantState(tenantCtx).ModelRequestRateLimitSuccessCount)
	common.TenantState(tenantCtx).OptionMap["ModelRequestRateLimitGroup"] = setting.ModelRequestRateLimitGroup2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["ModelRatio"] = ratio_setting.ModelRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["ModelPrice"] = ratio_setting.ModelPrice2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["CacheRatio"] = ratio_setting.CacheRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["CreateCacheRatio"] = ratio_setting.CreateCacheRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["GroupRatio"] = ratio_setting.GroupRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["GroupGroupRatio"] = ratio_setting.GroupGroupRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["UserUsableGroups"] = setting.UserUsableGroups2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["CompletionRatio"] = ratio_setting.CompletionRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["ImageRatio"] = ratio_setting.ImageRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["AudioRatio"] = ratio_setting.AudioRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["AudioCompletionRatio"] = ratio_setting.AudioCompletionRatio2JSONString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["TopUpLink"] = common.TenantState(tenantCtx).TopUpLink
	//common.OptionMap["ChatLink"] = common.ChatLink
	//common.OptionMap["ChatLink2"] = common.ChatLink2
	common.TenantState(tenantCtx).OptionMap["QuotaPerUnit"] = strconv.FormatFloat(common.TenantState(tenantCtx).QuotaPerUnit, 'f', -1, 64)
	common.TenantState(tenantCtx).OptionMap["RetryTimes"] = strconv.Itoa(common.TenantState(tenantCtx).RetryTimes)
	common.TenantState(tenantCtx).OptionMap["DataExportInterval"] = strconv.Itoa(common.TenantState(tenantCtx).DataExportInterval)
	common.TenantState(tenantCtx).OptionMap["DataExportDefaultTime"] = common.TenantState(tenantCtx).DataExportDefaultTime
	common.TenantState(tenantCtx).OptionMap["DefaultCollapseSidebar"] = strconv.FormatBool(common.TenantState(tenantCtx).DefaultCollapseSidebar)
	common.TenantState(tenantCtx).OptionMap["MjNotifyEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).MjNotifyEnabled)
	common.TenantState(tenantCtx).OptionMap["MjAccountFilterEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).MjAccountFilterEnabled)
	common.TenantState(tenantCtx).OptionMap["MjModeClearEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).MjModeClearEnabled)
	common.TenantState(tenantCtx).OptionMap["MjForwardUrlEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).MjForwardUrlEnabled)
	common.TenantState(tenantCtx).OptionMap["MjActionCheckSuccessEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).MjActionCheckSuccessEnabled)
	common.TenantState(tenantCtx).OptionMap["CheckSensitiveEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).CheckSensitiveEnabled)
	common.TenantState(tenantCtx).OptionMap["DemoSiteEnabled"] = strconv.FormatBool(operation_setting.TenantState(tenantCtx).DemoSiteEnabled)
	common.TenantState(tenantCtx).OptionMap["SelfUseModeEnabled"] = strconv.FormatBool(operation_setting.TenantState(tenantCtx).SelfUseModeEnabled)
	common.TenantState(tenantCtx).OptionMap["ModelRequestRateLimitEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).ModelRequestRateLimitEnabled)
	common.TenantState(tenantCtx).OptionMap["CheckSensitiveOnPromptEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).CheckSensitiveOnPromptEnabled)
	common.TenantState(tenantCtx).OptionMap["StopOnSensitiveEnabled"] = strconv.FormatBool(setting.TenantState(tenantCtx).StopOnSensitiveEnabled)
	common.TenantState(tenantCtx).OptionMap["SensitiveWords"] = setting.SensitiveWordsToString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["StreamCacheQueueLength"] = strconv.Itoa(setting.TenantState(tenantCtx).StreamCacheQueueLength)
	common.TenantState(tenantCtx).OptionMap["AutomaticDisableKeywords"] = operation_setting.AutomaticDisableKeywordsToString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["AutomaticDisableStatusCodes"] = operation_setting.AutomaticDisableStatusCodesToString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["AutomaticRetryStatusCodes"] = operation_setting.AutomaticRetryStatusCodesToString(tenantCtx)
	common.TenantState(tenantCtx).OptionMap["ExposeRatioEnabled"] = strconv.FormatBool(ratio_setting.IsExposeRatioEnabled(tenantCtx))

	// 自动添加所有注册的模型配置
	modelConfigs := config.GlobalConfig.ForTenant(tenantCtx).ExportAllConfigs()
	maps.Copy(common.TenantState(tenantCtx).OptionMap, modelConfigs)

	common.TenantState(tenantCtx).OptionMapRWMutex.Unlock()
	loadOptionsFromDatabase(tenantCtx)
}

func loadOptionsFromDatabase(tenantCtx context.Context) {
	TenantState(tenantCtx).passkeyOptionMutex.Lock()
	defer TenantState(tenantCtx).passkeyOptionMutex.Unlock()
	options, _ := AllOption(tenantCtx)
	passkeyOptions := make(map[string]string)
	for _, option := range options {
		if IsPasskeyDomainOption(option.Key) {
			passkeyOptions[option.Key] = option.Value
			continue
		}
		err := updateOptionMap(tenantCtx, option.Key, option.Value)
		if err != nil {
			common.SysLog("failed to update option map: " + err.Error())
		}
	}
	applyPasskeyDomainOptions(tenantCtx, passkeyOptions)
}

func SyncOptions(tenantCtx context.Context, frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing options from database")
		loadOptionsFromDatabase(tenantCtx)
	}
}

func validateOptionValue(key string, value string) error {
	if key == operation_setting.ToolPriceOptionKey {
		return operation_setting.ValidateToolPricesJSON(value)
	}
	if key == operation_setting.ChannelTestConcurrencyOptionKey {
		return operation_setting.ValidateChannelTestConcurrency(value)
	}
	if key == "MaxTokenAutoGroups" {
		return setting.ValidateMaxTokenAutoGroups(value)
	}
	return nil
}

func UpdateOption(tenantCtx context.Context, key string, value string) error {
	if err := plan.ValidateOption(tenantCtx, DB, key); err != nil {
		return err
	}
	if IsPasskeyDomainOption(key) {
		_, err := UpdatePasskeyDomainOptions(tenantCtx, map[string]string{key: value}, false, "")
		return err
	}
	if IsModelPricingOption(key) {
		return UpdateModelPricingOptions(tenantCtx, map[string]string{key: value})
	}
	if err := validateOptionValue(key, value); err != nil {
		return err
	}
	// Save to database first
	option := Option{
		Key: key,
	}
	// https://gorm.io/docs/update.html#Save-All-Fields
	if err := DB.WithContext(tenantCtx).FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
		return err
	}
	if err := DB.WithContext(tenantCtx).Model(&Option{}).Where("tenant_id = ? AND "+commonKeyCol+" = ?", option.TenantID, key).Update("value", value).Error; err != nil {
		return err
	}
	// Update OptionMap
	return updateOptionMap(tenantCtx, key, value)
}

// UpdateOptionsBulk persists multiple key/value pairs in a single database
// transaction, then dispatches them through updateOptionMap in one pass. If
// any DB write fails the whole transaction rolls back and no in-memory state
// is touched — safe for callers that must commit a set of related options
// atomically (e.g. payment gateway binding).
func UpdateOptionsBulk(tenantCtx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	for key := range values {
		if err := plan.ValidateOption(tenantCtx, DB, key); err != nil {
			return err
		}
	}
	for key := range values {
		if IsPasskeyDomainOption(key) {
			_, err := UpdatePasskeyDomainOptions(tenantCtx, values, false, "")
			return err
		}
	}
	for key, value := range values {
		if err := validateOptionValue(key, value); err != nil {
			return err
		}
	}
	err := DB.WithContext(tenantCtx).Transaction(func(tx *gorm.DB) error {
		for k, v := range values {
			option := Option{Key: k}
			if err := tx.FirstOrCreate(&option, Option{Key: k}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Option{}).Where(Option{Key: k}).Update("value", v).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for k, v := range values {
		if err := updateOptionMap(tenantCtx, k, v); err != nil {
			return err
		}
	}
	return nil
}

func updateOptionMap(tenantCtx context.Context, key string, value string) (err error) {
	if key == retiredThemeOptionKey {
		common.TenantState(tenantCtx).OptionMapRWMutex.Lock()
		delete(common.TenantState(tenantCtx).OptionMap, key)
		common.TenantState(tenantCtx).OptionMapRWMutex.Unlock()
		return nil
	}
	common.TenantState(tenantCtx).OptionMapRWMutex.Lock()
	defer common.TenantState(tenantCtx).OptionMapRWMutex.Unlock()
	common.TenantState(tenantCtx).OptionMap[key] = value

	// 检查是否是模型配置 - 使用更规范的方式处理
	if handleConfigUpdate(tenantCtx, key, value) {
		return nil // 已由配置系统处理
	}

	// 处理传统配置项...
	if strings.HasSuffix(key, "Permission") {
		intValue, _ := strconv.Atoi(value)
		switch key {
		case "FileUploadPermission":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.FileUploadPermission = intValue })
		case "FileDownloadPermission":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.FileDownloadPermission = intValue })
		case "ImageUploadPermission":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.ImageUploadPermission = intValue })
		case "ImageDownloadPermission":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.ImageDownloadPermission = intValue })
		}
	}
	if strings.HasSuffix(key, "Enabled") || key == "DefaultCollapseSidebar" || key == "DefaultUseAutoGroup" || key == "SMTPForceAuthLogin" || key == "SMTPInsecureSkipVerify" {
		boolValue := value == "true"
		switch key {
		case "PasswordRegisterEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.PasswordRegisterEnabled = boolValue })
		case "PasswordLoginEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.PasswordLoginEnabled = boolValue })
		case "EmailVerificationEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.EmailVerificationEnabled = boolValue })
		case "GitHubOAuthEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.GitHubOAuthEnabled = boolValue })
		case "LinuxDOOAuthEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.LinuxDOOAuthEnabled = boolValue })
		case "WeChatAuthEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.WeChatAuthEnabled = boolValue })
		case "TelegramOAuthEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TelegramOAuthEnabled = boolValue })
		case "TurnstileCheckEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TurnstileCheckEnabled = boolValue })
		case "RegisterEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.RegisterEnabled = boolValue })
		case "EmailDomainRestrictionEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.EmailDomainRestrictionEnabled = boolValue })
		case "EmailAliasRestrictionEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.EmailAliasRestrictionEnabled = boolValue })
		case "AutomaticDisableChannelEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.AutomaticDisableChannelEnabled = boolValue })
		case "AutomaticEnableChannelEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.AutomaticEnableChannelEnabled = boolValue })
		case "LogConsumeEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.LogConsumeEnabled = boolValue })
		case "PlatformMailEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.PlatformMailEnabled = boolValue })
		case "DisplayInCurrencyEnabled":
			// 兼容旧字段：同步到新配置 general_setting.quota_display_type（运行时生效）
			// true -> USD, false -> TOKENS
			newVal := "USD"
			if !boolValue {
				newVal = "TOKENS"
			}
			_ = config.GlobalConfig.ForTenant(tenantCtx).Update("general_setting", map[string]string{"quota_display_type": newVal})
		case "DisplayTokenStatEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.DisplayTokenStatEnabled = boolValue })
		case "DrawingEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.DrawingEnabled = boolValue })
		case "TaskEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TaskEnabled = boolValue })
		case "TaskPluginEnabled":
			constant.UpdateTenantRuntime(tenantCtx, func(state *constant.WorkspaceRuntime) { state.TaskPluginEnabled = boolValue })
			jsplugin.TenantState(tenantCtx).DefaultRegistry.SetEnabled(boolValue)
		case "DataExportEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.DataExportEnabled = boolValue })
		case "DefaultCollapseSidebar":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.DefaultCollapseSidebar = boolValue })
		case "MjNotifyEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.MjNotifyEnabled = boolValue })
		case "MjAccountFilterEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.MjAccountFilterEnabled = boolValue })
		case "MjModeClearEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.MjModeClearEnabled = boolValue })
		case "MjForwardUrlEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.MjForwardUrlEnabled = boolValue })
		case "MjActionCheckSuccessEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.MjActionCheckSuccessEnabled = boolValue })
		case "CheckSensitiveEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.CheckSensitiveEnabled = boolValue })
		case "DemoSiteEnabled":
			operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.DemoSiteEnabled = boolValue })
		case "SelfUseModeEnabled":
			operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.SelfUseModeEnabled = boolValue })
		case "CheckSensitiveOnPromptEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.CheckSensitiveOnPromptEnabled = boolValue })
		case "ModelRequestRateLimitEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.ModelRequestRateLimitEnabled = boolValue })
		case "StopOnSensitiveEnabled":
			setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StopOnSensitiveEnabled = boolValue })
		case "SMTPSSLEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPSSLEnabled = boolValue })
		case "SMTPStartTLSEnabled":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPStartTLSEnabled = boolValue })
		case "SMTPInsecureSkipVerify":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPInsecureSkipVerify = boolValue })
		case "SMTPForceAuthLogin":
			common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPForceAuthLogin = boolValue })
		case "WorkerAllowHttpImageRequestEnabled":
			system_setting.UpdateTenantSettings(tenantCtx, func(state *system_setting.WorkspaceState) { state.WorkerAllowHttpImageRequestEnabled = boolValue })
		case "DefaultUseAutoGroup":
			setting.UpdateTenantRuntime(tenantCtx, func(state *setting.WorkspaceRuntime) { state.DefaultUseAutoGroup = boolValue })
		case "ExposeRatioEnabled":
			ratio_setting.SetExposeRatioEnabled(tenantCtx, boolValue)
		}
	}
	if key == setting.TaskPluginDisabledFactoryKeysKey {
		jsplugin.TenantState(tenantCtx).DefaultRegistry.SetDisabledFactoryKeys(setting.ParseTaskPluginDisabledFactoryKeys(value))
	}
	switch key {
	case "EmailDomainWhitelist":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.EmailDomainWhitelist = strings.Split(value, ",") })
	case "SMTPServer":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPServer = value })
	case "SMTPPort":
		intValue, _ := strconv.Atoi(value)
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPPort = intValue })
	case "SMTPAccount":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPAccount = value })
	case "SMTPFrom":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPFrom = value })
	case "SMTPToken":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SMTPToken = value })
	case "ServerAddress":
		system_setting.UpdateTenantSettings(tenantCtx, func(state *system_setting.WorkspaceState) { state.ServerAddress = value })
	case "TaskPublicAddress":
		system_setting.UpdateTenantSettings(tenantCtx, func(state *system_setting.WorkspaceState) { state.TaskPublicAddress = value })
	case "WorkerUrl":
		system_setting.UpdateTenantSettings(tenantCtx, func(state *system_setting.WorkspaceState) { state.WorkerUrl = value })
	case "WorkerValidKey":
		system_setting.UpdateTenantSettings(tenantCtx, func(state *system_setting.WorkspaceState) { state.WorkerValidKey = value })
	case "PayAddress":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.PayAddress = value })
	case "Chats":
		err = setting.UpdateChatsByJsonString(tenantCtx, value)
	case "AutoGroups":
		err = setting.UpdateAutoGroupsByJsonString(tenantCtx, value)
	case "MaxTokenAutoGroups":
		err = setting.UpdateMaxTokenAutoGroups(tenantCtx, value)
	case "CustomCallbackAddress":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.CustomCallbackAddress = value })
	case "EpayId":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.EpayId = value })
	case "EpayKey":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.EpayKey = value })
	case "Price":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.Price, _ = strconv.ParseFloat(value, 64) })
	case "USDExchangeRate":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) {
			state.USDExchangeRate, _ = strconv.ParseFloat(value, 64)
		})
	case "MinTopUp":
		operation_setting.UpdateTenantSettings(tenantCtx, func(state *operation_setting.WorkspaceState) { state.MinTopUp, _ = strconv.Atoi(value) })
	case "StripeApiSecret":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StripeApiSecret = value })
	case "StripeWebhookSecret":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StripeWebhookSecret = value })
	case "StripePriceId":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StripePriceId = value })
	case "StripeUnitPrice":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StripeUnitPrice, _ = strconv.ParseFloat(value, 64) })
	case "StripeMinTopUp":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StripeMinTopUp, _ = strconv.Atoi(value) })
	case "StripePromotionCodesEnabled":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StripePromotionCodesEnabled = value == "true" })
	case "CreemApiKey":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.CreemApiKey = value })
	case "CreemProducts":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.CreemProducts = value })
	case "CreemTestMode":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.CreemTestMode = value == "true" })
	case "CreemWebhookSecret":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.CreemWebhookSecret = value })
	case "WaffoEnabled":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoEnabled = value == "true" })
	case "WaffoApiKey":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoApiKey = value })
	case "WaffoPrivateKey":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPrivateKey = value })
	case "WaffoPublicCert":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPublicCert = value })
	case "WaffoSandboxPublicCert":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoSandboxPublicCert = value })
	case "WaffoSandboxApiKey":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoSandboxApiKey = value })
	case "WaffoSandboxPrivateKey":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoSandboxPrivateKey = value })
	case "WaffoSandbox":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoSandbox = value == "true" })
	case "WaffoMerchantId":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoMerchantId = value })
	case "WaffoNotifyUrl":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoNotifyUrl = value })
	case "WaffoReturnUrl":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoReturnUrl = value })
	case "WaffoSubscriptionReturnUrl":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoSubscriptionReturnUrl = value })
	case "WaffoCurrency":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoCurrency = value })
	case "WaffoUnitPrice":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoUnitPrice, _ = strconv.ParseFloat(value, 64) })
	case "WaffoMinTopUp":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoMinTopUp, _ = strconv.Atoi(value) })
	case "WaffoPancakeMerchantID":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakeMerchantID = value })
	case "WaffoPancakePrivateKey":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakePrivateKey = value })
	case "WaffoPancakeReturnURL":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakeReturnURL = value })
	case "WaffoPancakeStoreID":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakeStoreID = value })
	case "WaffoPancakeProductID":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakeProductID = value })
	case "WaffoPancakeUnitPrice":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakeUnitPrice, _ = strconv.ParseFloat(value, 64) })
	case "WaffoPancakeMinTopUp":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.WaffoPancakeMinTopUp, _ = strconv.Atoi(value) })
	case "TopupGroupRatio":
		err = common.UpdateTopupGroupRatioByJSONString(tenantCtx, value)
	case "GitHubClientId":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.GitHubClientId = value })
	case "GitHubClientSecret":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.GitHubClientSecret = value })
	case "LinuxDOClientId":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.LinuxDOClientId = value })
	case "LinuxDOClientSecret":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.LinuxDOClientSecret = value })
	case "LinuxDOMinimumTrustLevel":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.LinuxDOMinimumTrustLevel, _ = strconv.Atoi(value) })
	case "Footer":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.Footer = value })
	case "SystemName":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.SystemName = value })
	case "Logo":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.Logo = value })
	case "WeChatServerAddress":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.WeChatServerAddress = value })
	case "WeChatServerToken":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.WeChatServerToken = value })
	case "WeChatAccountQRCodeImageURL":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.WeChatAccountQRCodeImageURL = value })
	case "TelegramBotToken":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TelegramBotToken = value })
	case "TelegramBotName":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TelegramBotName = value })
	case "TurnstileSiteKey":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TurnstileSiteKey = value })
	case "TurnstileSecretKey":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TurnstileSecretKey = value })
	case "QuotaForNewUser":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.QuotaForNewUser, _ = strconv.Atoi(value) })
	case "QuotaForInviter":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.QuotaForInviter, _ = strconv.Atoi(value) })
	case "QuotaForInvitee":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.QuotaForInvitee, _ = strconv.Atoi(value) })
	case "QuotaRemindThreshold":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.QuotaRemindThreshold, _ = strconv.Atoi(value) })
	case "PreConsumedQuota":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.PreConsumedQuota, _ = strconv.Atoi(value) })
	case "ModelRequestRateLimitCount":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.ModelRequestRateLimitCount, _ = strconv.Atoi(value) })
	case "ModelRequestRateLimitDurationMinutes":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) {
			state.ModelRequestRateLimitDurationMinutes, _ = strconv.Atoi(value)
		})
	case "ModelRequestRateLimitSuccessCount":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.ModelRequestRateLimitSuccessCount, _ = strconv.Atoi(value) })
	case "ModelRequestRateLimitGroup":
		err = setting.UpdateModelRequestRateLimitGroupByJSONString(tenantCtx, value)
	case "RetryTimes":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.RetryTimes, _ = strconv.Atoi(value) })
	case "DataExportInterval":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.DataExportInterval, _ = strconv.Atoi(value) })
	case "DataExportDefaultTime":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.DataExportDefaultTime = value })
	case "ModelRatio":
		err = ratio_setting.UpdateModelRatioByJSONString(tenantCtx, value)
	case "GroupRatio":
		err = ratio_setting.UpdateGroupRatioByJSONString(tenantCtx, value)
	case "GroupGroupRatio":
		err = ratio_setting.UpdateGroupGroupRatioByJSONString(tenantCtx, value)
	case "UserUsableGroups":
		err = setting.UpdateUserUsableGroupsByJSONString(tenantCtx, value)
	case "CompletionRatio":
		err = ratio_setting.UpdateCompletionRatioByJSONString(tenantCtx, value)
	case "ModelPrice":
		err = ratio_setting.UpdateModelPriceByJSONString(tenantCtx, value)
	case "CacheRatio":
		err = ratio_setting.UpdateCacheRatioByJSONString(tenantCtx, value)
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(tenantCtx, value)
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(tenantCtx, value)
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(tenantCtx, value)
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(tenantCtx, value)
	case "TopUpLink":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.TopUpLink = value })
	//case "ChatLink":
	//	common.ChatLink = value
	//case "ChatLink2":
	//	common.ChatLink2 = value
	case "ChannelDisableThreshold":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.ChannelDisableThreshold, _ = strconv.ParseFloat(value, 64) })
	case "QuotaPerUnit":
		common.UpdateTenantSettings(tenantCtx, func(state *common.WorkspaceState) { state.QuotaPerUnit, _ = strconv.ParseFloat(value, 64) })
	case "SensitiveWords":
		setting.SensitiveWordsFromString(tenantCtx, value)
	case "AutomaticDisableKeywords":
		operation_setting.AutomaticDisableKeywordsFromString(tenantCtx, value)
	case "AutomaticDisableStatusCodes":
		err = operation_setting.AutomaticDisableStatusCodesFromString(tenantCtx, value)
	case "AutomaticRetryStatusCodes":
		err = operation_setting.AutomaticRetryStatusCodesFromString(tenantCtx, value)
	case "StreamCacheQueueLength":
		setting.UpdateTenantSettings(tenantCtx, func(state *setting.WorkspaceState) { state.StreamCacheQueueLength, _ = strconv.Atoi(value) })
	case "PayMethods":
		err = operation_setting.UpdatePayMethodsByJsonString(tenantCtx, value)
	case "WaffoPayMethods":
		// WaffoPayMethods is read directly from OptionMap via setting.GetWaffoPayMethods().
		// The value is already stored in OptionMap at the top of this function (line: common.OptionMap[key] = value).
		// No additional in-memory variable to update.
	}
	return err
}

// handleConfigUpdate 处理分层配置更新，返回是否已处理
func handleConfigUpdate(tenantCtx context.Context, key, value string) bool {
	if key == operation_setting.ToolPriceOptionKey {
		operation_setting.LoadToolPricesFromJSONString(tenantCtx, value)
		return true
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return false // 不是分层配置
	}

	configName := parts[0]
	configKey := parts[1]

	// 获取配置对象
	cfg := config.GlobalConfig.ForTenant(tenantCtx).Get(configName)
	if cfg == nil {
		return false // 未注册的配置
	}

	// 更新配置
	configMap := map[string]string{
		configKey: value,
	}
	config.GlobalConfig.ForTenant(tenantCtx).Update(configName, configMap)

	// 特定配置的后处理
	if configName == "performance_setting" {
		performance_setting.UpdateAndSync(tenantCtx)
	} else if configName == "billing_setting" {
		InvalidatePricingCache(tenantCtx)
		ratio_setting.InvalidateExposedDataCache(tenantCtx)
	}

	return true // 已处理
}

func RefreshTenantSettings(ctx context.Context) { loadOptionsFromDatabase(ctx) }
