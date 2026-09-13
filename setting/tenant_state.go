// TenantState keeps mutable workspace settings and caches isolated.
package setting

import (
	context "context"
	tenant "github.com/QuantumNous/new-api/tenant"
	sync "sync"
	atomic "sync/atomic"
)

type WorkspaceState struct {
	*workspaceCaches
	Chats                                []map[string]string
	CheckSensitiveEnabled                bool
	CheckSensitiveOnPromptEnabled        bool
	CreemApiKey                          string
	CreemProducts                        string
	CreemTestMode                        bool
	CreemWebhookSecret                   string
	MjAccountFilterEnabled               bool
	MjActionCheckSuccessEnabled          bool
	MjForwardUrlEnabled                  bool
	MjModeClearEnabled                   bool
	MjNotifyEnabled                      bool
	ModelRequestRateLimitCount           int
	ModelRequestRateLimitDurationMinutes int
	ModelRequestRateLimitEnabled         bool
	ModelRequestRateLimitSuccessCount    int
	SensitiveWords                       []string
	StopOnSensitiveEnabled               bool
	StreamCacheQueueLength               int
	StripeApiSecret                      string
	StripeMinTopUp                       int
	StripePriceId                        string
	StripePromotionCodesEnabled          bool
	StripeUnitPrice                      float64
	StripeWebhookSecret                  string
	WaffoApiKey                          string
	WaffoCurrency                        string
	WaffoEnabled                         bool
	WaffoMerchantId                      string
	WaffoMinTopUp                        int
	WaffoNotifyUrl                       string
	WaffoPancakeMerchantID               string
	WaffoPancakeMinTopUp                 int
	WaffoPancakePrivateKey               string
	WaffoPancakeProductID                string
	WaffoPancakeReturnURL                string
	WaffoPancakeStoreID                  string
	WaffoPancakeUnitPrice                float64
	WaffoPrivateKey                      string
	WaffoPublicCert                      string
	WaffoReturnUrl                       string
	WaffoSandbox                         bool
	WaffoSandboxApiKey                   string
	WaffoSandboxPrivateKey               string
	WaffoSandboxPublicCert               string
	WaffoSubscriptionReturnUrl           string
	WaffoUnitPrice                       float64
	autoGroups                           []string
}

type workspaceCaches struct {
	ModelRequestRateLimitGroup map[string][2]int
	ModelRequestRateLimitMutex sync.RWMutex
	maxTokenAutoGroups         atomic.Int64
	userUsableGroups           map[string]string
	userUsableGroupsMutex      sync.RWMutex
}

var workspaceStates tenant.Registry[*tenant.Settings[WorkspaceState]]

func tenantSettings(ctx context.Context) *tenant.Settings[WorkspaceState] {
	value, err := workspaceStates.Get(ctx, func() *tenant.Settings[WorkspaceState] {
		state := &WorkspaceState{
			workspaceCaches: &workspaceCaches{
				ModelRequestRateLimitGroup: tenant.Clone(ModelRequestRateLimitGroup),
				userUsableGroups:           tenant.Clone(userUsableGroups),
			},
			Chats:                                tenant.Clone(Chats),
			CheckSensitiveEnabled:                tenant.Clone(CheckSensitiveEnabled),
			CheckSensitiveOnPromptEnabled:        tenant.Clone(CheckSensitiveOnPromptEnabled),
			CreemApiKey:                          tenant.Clone(CreemApiKey),
			CreemProducts:                        tenant.Clone(CreemProducts),
			CreemTestMode:                        tenant.Clone(CreemTestMode),
			CreemWebhookSecret:                   tenant.Clone(CreemWebhookSecret),
			MjAccountFilterEnabled:               tenant.Clone(MjAccountFilterEnabled),
			MjActionCheckSuccessEnabled:          tenant.Clone(MjActionCheckSuccessEnabled),
			MjForwardUrlEnabled:                  tenant.Clone(MjForwardUrlEnabled),
			MjModeClearEnabled:                   tenant.Clone(MjModeClearEnabled),
			MjNotifyEnabled:                      tenant.Clone(MjNotifyEnabled),
			ModelRequestRateLimitCount:           tenant.Clone(ModelRequestRateLimitCount),
			ModelRequestRateLimitDurationMinutes: tenant.Clone(ModelRequestRateLimitDurationMinutes),
			ModelRequestRateLimitEnabled:         tenant.Clone(ModelRequestRateLimitEnabled),
			ModelRequestRateLimitSuccessCount:    tenant.Clone(ModelRequestRateLimitSuccessCount),
			SensitiveWords:                       tenant.Clone(SensitiveWords),
			StopOnSensitiveEnabled:               tenant.Clone(StopOnSensitiveEnabled),
			StreamCacheQueueLength:               tenant.Clone(StreamCacheQueueLength),
			StripeApiSecret:                      tenant.Clone(StripeApiSecret),
			StripeMinTopUp:                       tenant.Clone(StripeMinTopUp),
			StripePriceId:                        tenant.Clone(StripePriceId),
			StripePromotionCodesEnabled:          tenant.Clone(StripePromotionCodesEnabled),
			StripeUnitPrice:                      tenant.Clone(StripeUnitPrice),
			StripeWebhookSecret:                  tenant.Clone(StripeWebhookSecret),
			WaffoApiKey:                          tenant.Clone(WaffoApiKey),
			WaffoCurrency:                        tenant.Clone(WaffoCurrency),
			WaffoEnabled:                         tenant.Clone(WaffoEnabled),
			WaffoMerchantId:                      tenant.Clone(WaffoMerchantId),
			WaffoMinTopUp:                        tenant.Clone(WaffoMinTopUp),
			WaffoNotifyUrl:                       tenant.Clone(WaffoNotifyUrl),
			WaffoPancakeMerchantID:               tenant.Clone(WaffoPancakeMerchantID),
			WaffoPancakeMinTopUp:                 tenant.Clone(WaffoPancakeMinTopUp),
			WaffoPancakePrivateKey:               tenant.Clone(WaffoPancakePrivateKey),
			WaffoPancakeProductID:                tenant.Clone(WaffoPancakeProductID),
			WaffoPancakeReturnURL:                tenant.Clone(WaffoPancakeReturnURL),
			WaffoPancakeStoreID:                  tenant.Clone(WaffoPancakeStoreID),
			WaffoPancakeUnitPrice:                tenant.Clone(WaffoPancakeUnitPrice),
			WaffoPrivateKey:                      tenant.Clone(WaffoPrivateKey),
			WaffoPublicCert:                      tenant.Clone(WaffoPublicCert),
			WaffoReturnUrl:                       tenant.Clone(WaffoReturnUrl),
			WaffoSandbox:                         tenant.Clone(WaffoSandbox),
			WaffoSandboxApiKey:                   tenant.Clone(WaffoSandboxApiKey),
			WaffoSandboxPrivateKey:               tenant.Clone(WaffoSandboxPrivateKey),
			WaffoSandboxPublicCert:               tenant.Clone(WaffoSandboxPublicCert),
			WaffoSubscriptionReturnUrl:           tenant.Clone(WaffoSubscriptionReturnUrl),
			WaffoUnitPrice:                       tenant.Clone(WaffoUnitPrice),
			autoGroups:                           tenant.Clone(autoGroups),
		}
		state.maxTokenAutoGroups.Store(DefaultMaxTokenAutoGroups)
		return tenant.NewSettings(state)
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
