package controller

import (
	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting(testtenant.Context())
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

func TestStripeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPISecret := setting.TenantState(testtenant.Context()).StripeApiSecret
	originalWebhookSecret := setting.TenantState(testtenant.Context()).StripeWebhookSecret
	originalPriceID := setting.TenantState(testtenant.Context()).StripePriceId
	t.Cleanup(func() {
		setting.TenantState(testtenant.Context()).StripeApiSecret = originalAPISecret
		setting.TenantState(testtenant.Context()).StripeWebhookSecret = originalWebhookSecret
		setting.TenantState(testtenant.Context()).StripePriceId = originalPriceID
	})

	setting.TenantState(testtenant.Context()).StripeWebhookSecret = ""
	setting.TenantState(testtenant.Context()).StripeApiSecret = "sk_test_123"
	setting.TenantState(testtenant.Context()).StripePriceId = "price_123"
	require.False(t, isStripeWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).StripeWebhookSecret = "whsec_test"
	require.True(t, isStripeWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).StripePriceId = ""
	require.False(t, isStripeWebhookEnabled(testtenant.Context()))
}

func TestCreemWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPIKey := setting.TenantState(testtenant.Context()).CreemApiKey
	originalProducts := setting.TenantState(testtenant.Context()).CreemProducts
	originalWebhookSecret := setting.TenantState(testtenant.Context()).CreemWebhookSecret
	t.Cleanup(func() {
		setting.TenantState(testtenant.Context()).CreemApiKey = originalAPIKey
		setting.TenantState(testtenant.Context()).CreemProducts = originalProducts
		setting.TenantState(testtenant.Context()).CreemWebhookSecret = originalWebhookSecret
	})

	setting.TenantState(testtenant.Context()).CreemWebhookSecret = ""
	setting.TenantState(testtenant.Context()).CreemApiKey = "creem_api_key"
	setting.TenantState(testtenant.Context()).CreemProducts = `[{"productId":"prod_123"}]`
	require.False(t, isCreemWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).CreemWebhookSecret = "creem_secret"
	require.True(t, isCreemWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).CreemProducts = "[]"
	require.False(t, isCreemWebhookEnabled(testtenant.Context()))
}

func TestWaffoWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.TenantState(testtenant.Context()).WaffoEnabled
	originalSandbox := setting.TenantState(testtenant.Context()).WaffoSandbox
	originalAPIKey := setting.TenantState(testtenant.Context()).WaffoApiKey
	originalPrivateKey := setting.TenantState(testtenant.Context()).WaffoPrivateKey
	originalPublicCert := setting.TenantState(testtenant.Context()).WaffoPublicCert
	originalSandboxAPIKey := setting.TenantState(testtenant.Context()).WaffoSandboxApiKey
	originalSandboxPrivateKey := setting.TenantState(testtenant.Context()).WaffoSandboxPrivateKey
	originalSandboxPublicCert := setting.TenantState(testtenant.Context()).WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.TenantState(testtenant.Context()).WaffoEnabled = originalEnabled
		setting.TenantState(testtenant.Context()).WaffoSandbox = originalSandbox
		setting.TenantState(testtenant.Context()).WaffoApiKey = originalAPIKey
		setting.TenantState(testtenant.Context()).WaffoPrivateKey = originalPrivateKey
		setting.TenantState(testtenant.Context()).WaffoPublicCert = originalPublicCert
		setting.TenantState(testtenant.Context()).WaffoSandboxApiKey = originalSandboxAPIKey
		setting.TenantState(testtenant.Context()).WaffoSandboxPrivateKey = originalSandboxPrivateKey
		setting.TenantState(testtenant.Context()).WaffoSandboxPublicCert = originalSandboxPublicCert
	})

	setting.TenantState(testtenant.Context()).WaffoEnabled = true
	setting.TenantState(testtenant.Context()).WaffoSandbox = false
	setting.TenantState(testtenant.Context()).WaffoApiKey = ""
	setting.TenantState(testtenant.Context()).WaffoPrivateKey = "private"
	setting.TenantState(testtenant.Context()).WaffoPublicCert = "public"
	require.False(t, isWaffoWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoApiKey = "api"
	require.True(t, isWaffoWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoEnabled = false
	require.False(t, isWaffoWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoEnabled = true
	setting.TenantState(testtenant.Context()).WaffoSandbox = true
	setting.TenantState(testtenant.Context()).WaffoSandboxApiKey = ""
	setting.TenantState(testtenant.Context()).WaffoSandboxPrivateKey = "sandbox_private"
	setting.TenantState(testtenant.Context()).WaffoSandboxPublicCert = "sandbox_public"
	require.False(t, isWaffoWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoSandboxApiKey = "sandbox_api"
	require.True(t, isWaffoWebhookEnabled(testtenant.Context()))
}

func TestWaffoPancakeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMerchantID := setting.TenantState(testtenant.Context()).WaffoPancakeMerchantID
	originalPrivateKey := setting.TenantState(testtenant.Context()).WaffoPancakePrivateKey
	originalProductID := setting.TenantState(testtenant.Context()).WaffoPancakeProductID
	t.Cleanup(func() {
		setting.TenantState(testtenant.Context()).WaffoPancakeMerchantID = originalMerchantID
		setting.TenantState(testtenant.Context()).WaffoPancakePrivateKey = originalPrivateKey
		setting.TenantState(testtenant.Context()).WaffoPancakeProductID = originalProductID
	})

	// Presence of all three credentials enables the gateway. Webhook public
	// keys are bundled in the SDK and there is no separate Enabled toggle —
	// clear any of the three fields to disable.
	setting.TenantState(testtenant.Context()).WaffoPancakeMerchantID = ""
	setting.TenantState(testtenant.Context()).WaffoPancakePrivateKey = "private"
	setting.TenantState(testtenant.Context()).WaffoPancakeProductID = "product"
	require.False(t, isWaffoPancakeWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoPancakeMerchantID = "merchant"
	require.True(t, isWaffoPancakeWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoPancakeProductID = ""
	require.False(t, isWaffoPancakeWebhookEnabled(testtenant.Context()))

	setting.TenantState(testtenant.Context()).WaffoPancakeProductID = "product"
	setting.TenantState(testtenant.Context()).WaffoPancakePrivateKey = ""
	require.False(t, isWaffoPancakeWebhookEnabled(testtenant.Context()))
}

func TestEpayWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.TenantState(testtenant.Context()).PayAddress
	originalEpayID := operation_setting.TenantState(testtenant.Context()).EpayId
	originalEpayKey := operation_setting.TenantState(testtenant.Context()).EpayKey
	originalPayMethods := operation_setting.TenantState(testtenant.Context()).PayMethods
	t.Cleanup(func() {
		operation_setting.TenantState(testtenant.Context()).PayAddress = originalPayAddress
		operation_setting.TenantState(testtenant.Context()).EpayId = originalEpayID
		operation_setting.TenantState(testtenant.Context()).EpayKey = originalEpayKey
		operation_setting.TenantState(testtenant.Context()).PayMethods = originalPayMethods
	})

	operation_setting.TenantState(testtenant.Context()).PayAddress = "https://pay.example.com"
	operation_setting.TenantState(testtenant.Context()).EpayId = "epay_id"
	operation_setting.TenantState(testtenant.Context()).EpayKey = ""
	operation_setting.TenantState(testtenant.Context()).PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled(testtenant.Context()))

	operation_setting.TenantState(testtenant.Context()).EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled(testtenant.Context()))

	operation_setting.TenantState(testtenant.Context()).PayMethods = nil
	require.False(t, isEpayWebhookEnabled(testtenant.Context()))
}
