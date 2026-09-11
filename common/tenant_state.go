// TenantState keeps mutable workspace settings and caches isolated.
package common

import (
	context "context"
	rsa "crypto/rsa"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
)

type WorkspaceState struct {
	AutomaticDisableChannelEnabled bool
	AutomaticEnableChannelEnabled  bool
	ChannelDisableThreshold        float64
	DataExportDefaultTime          string
	DataExportEnabled              bool
	DataExportInterval             int
	DefaultCollapseSidebar         bool
	DisplayInCurrencyEnabled       bool
	DisplayTokenStatEnabled        bool
	DrawingEnabled                 bool
	EmailAliasRestrictionEnabled   bool
	EmailDomainRestrictionEnabled  bool
	EmailDomainWhitelist           []string
	EmailVerificationEnabled       bool
	FileDownloadPermission         int
	FileUploadPermission           int
	Footer                         string
	GitHubClientId                 string
	GitHubClientSecret             string
	GitHubOAuthEnabled             bool
	ImageDownloadPermission        int
	ImageUploadPermission          int
	LinuxDOClientId                string
	LinuxDOClientSecret            string
	LinuxDOMinimumTrustLevel       int
	LinuxDOOAuthEnabled            bool
	LogConsumeEnabled              bool
	Logo                           string
	PasswordLoginEnabled           bool
	PasswordRegisterEnabled        bool
	PreConsumedQuota               int
	QuotaForInvitee                int
	QuotaForInviter                int
	QuotaForNewUser                int
	QuotaPerUnit                   float64
	QuotaRemindThreshold           int
	RegisterEnabled                bool
	RetryTimes                     int
	SMTPAccount                    string
	SMTPForceAuthLogin             bool
	SMTPFrom                       string
	SMTPInsecureSkipVerify         bool
	SMTPPort                       int
	SMTPSSLEnabled                 bool
	SMTPServer                     string
	SMTPStartTLSEnabled            bool
	SMTPToken                      string
	SystemName                     string
	TaskEnabled                    bool
	TelegramBotName                string
	TelegramBotToken               string
	TelegramOAuthEnabled           bool
	TopUpLink                      string
	TurnstileCheckEnabled          bool
	TurnstileSecretKey             string
	TurnstileSiteKey               string
	WeChatAccountQRCodeImageURL    string
	WeChatAuthEnabled              bool
	WeChatServerAddress            string
	WeChatServerToken              string
	*workspaceCaches
}

type workspaceCaches struct {
	OptionMapRWMutex        sync.RWMutex
	OptionMap               map[string]string
	passwordEncryptionState struct {
		sync.RWMutex
		privateKey *rsa.PrivateKey
		publicKey  string
		keyID      string
	}
	topupGroupRatio        map[string]float64
	topupGroupRatioMutex   sync.RWMutex
	verificationMap        map[string]verificationValue
	verificationMapMaxSize int
	verificationMutex      sync.Mutex
}

var workspaceStates tenant.Registry[*tenant.Settings[WorkspaceState]]

func tenantSettings(ctx context.Context) *tenant.Settings[WorkspaceState] {
	value, err := workspaceStates.Get(ctx, func() *tenant.Settings[WorkspaceState] {
		return tenant.NewSettings(&WorkspaceState{
			workspaceCaches: &workspaceCaches{
				OptionMap:              tenant.Clone(OptionMap),
				topupGroupRatio:        tenant.Clone(topupGroupRatio),
				verificationMap:        tenant.Clone(verificationMap),
				verificationMapMaxSize: tenant.Clone(verificationMapMaxSize),
			},
			AutomaticDisableChannelEnabled: tenant.Clone(AutomaticDisableChannelEnabled),
			AutomaticEnableChannelEnabled:  tenant.Clone(AutomaticEnableChannelEnabled),
			ChannelDisableThreshold:        tenant.Clone(ChannelDisableThreshold),
			DataExportDefaultTime:          tenant.Clone(DataExportDefaultTime),
			DataExportEnabled:              tenant.Clone(DataExportEnabled),
			DataExportInterval:             tenant.Clone(DataExportInterval),
			DefaultCollapseSidebar:         tenant.Clone(DefaultCollapseSidebar),
			DisplayInCurrencyEnabled:       tenant.Clone(DisplayInCurrencyEnabled),
			DisplayTokenStatEnabled:        tenant.Clone(DisplayTokenStatEnabled),
			DrawingEnabled:                 tenant.Clone(DrawingEnabled),
			EmailAliasRestrictionEnabled:   tenant.Clone(EmailAliasRestrictionEnabled),
			EmailDomainRestrictionEnabled:  tenant.Clone(EmailDomainRestrictionEnabled),
			EmailDomainWhitelist:           tenant.Clone(EmailDomainWhitelist),
			EmailVerificationEnabled:       tenant.Clone(EmailVerificationEnabled),
			FileDownloadPermission:         tenant.Clone(FileDownloadPermission),
			FileUploadPermission:           tenant.Clone(FileUploadPermission),
			Footer:                         tenant.Clone(Footer),
			GitHubClientId:                 tenant.Clone(GitHubClientId),
			GitHubClientSecret:             tenant.Clone(GitHubClientSecret),
			GitHubOAuthEnabled:             tenant.Clone(GitHubOAuthEnabled),
			ImageDownloadPermission:        tenant.Clone(ImageDownloadPermission),
			ImageUploadPermission:          tenant.Clone(ImageUploadPermission),
			LinuxDOClientId:                tenant.Clone(LinuxDOClientId),
			LinuxDOClientSecret:            tenant.Clone(LinuxDOClientSecret),
			LinuxDOMinimumTrustLevel:       tenant.Clone(LinuxDOMinimumTrustLevel),
			LinuxDOOAuthEnabled:            tenant.Clone(LinuxDOOAuthEnabled),
			LogConsumeEnabled:              tenant.Clone(LogConsumeEnabled),
			Logo:                           tenant.Clone(Logo),
			PasswordLoginEnabled:           tenant.Clone(PasswordLoginEnabled),
			PasswordRegisterEnabled:        tenant.Clone(PasswordRegisterEnabled),
			PreConsumedQuota:               tenant.Clone(PreConsumedQuota),
			QuotaForInvitee:                tenant.Clone(QuotaForInvitee),
			QuotaForInviter:                tenant.Clone(QuotaForInviter),
			QuotaForNewUser:                tenant.Clone(QuotaForNewUser),
			QuotaPerUnit:                   tenant.Clone(QuotaPerUnit),
			QuotaRemindThreshold:           tenant.Clone(QuotaRemindThreshold),
			RegisterEnabled:                tenant.Clone(RegisterEnabled),
			RetryTimes:                     tenant.Clone(RetryTimes),
			SMTPAccount:                    tenant.Clone(SMTPAccount),
			SMTPForceAuthLogin:             tenant.Clone(SMTPForceAuthLogin),
			SMTPFrom:                       tenant.Clone(SMTPFrom),
			SMTPInsecureSkipVerify:         tenant.Clone(SMTPInsecureSkipVerify),
			SMTPPort:                       tenant.Clone(SMTPPort),
			SMTPSSLEnabled:                 tenant.Clone(SMTPSSLEnabled),
			SMTPServer:                     tenant.Clone(SMTPServer),
			SMTPStartTLSEnabled:            tenant.Clone(SMTPStartTLSEnabled),
			SMTPToken:                      tenant.Clone(SMTPToken),
			SystemName:                     tenant.Clone(SystemName),
			TaskEnabled:                    tenant.Clone(TaskEnabled),
			TelegramBotName:                tenant.Clone(TelegramBotName),
			TelegramBotToken:               tenant.Clone(TelegramBotToken),
			TelegramOAuthEnabled:           tenant.Clone(TelegramOAuthEnabled),
			TopUpLink:                      tenant.Clone(TopUpLink),
			TurnstileCheckEnabled:          tenant.Clone(TurnstileCheckEnabled),
			TurnstileSecretKey:             tenant.Clone(TurnstileSecretKey),
			TurnstileSiteKey:               tenant.Clone(TurnstileSiteKey),
			WeChatAccountQRCodeImageURL:    tenant.Clone(WeChatAccountQRCodeImageURL),
			WeChatAuthEnabled:              tenant.Clone(WeChatAuthEnabled),
			WeChatServerAddress:            tenant.Clone(WeChatServerAddress),
			WeChatServerToken:              tenant.Clone(WeChatServerToken),
		})
	})
	if err != nil {
		panic(err)
	}
	return value
}

// TenantState returns a read-only snapshot. Use UpdateTenantSettings to publish changes.
func TenantState(ctx context.Context) *WorkspaceState {
	return tenantSettings(ctx).Load()
}

func UpdateTenantSettings(ctx context.Context, update func(*WorkspaceState)) {
	tenantSettings(ctx).Update(update)
}
