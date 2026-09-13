package controller

import context "context"

import (
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func isPaymentComplianceConfirmed(tenantCtx context.Context) bool {
	return operation_setting.IsPaymentComplianceConfirmed(tenantCtx)
}

func isStripeTopUpEnabled(tenantCtx context.Context) bool {
	if !isPaymentComplianceConfirmed(tenantCtx) {
		return false
	}
	return strings.TrimSpace(setting.TenantState(tenantCtx).StripeApiSecret) != "" &&
		strings.TrimSpace(setting.TenantState(tenantCtx).StripeWebhookSecret) != "" &&
		strings.TrimSpace(setting.TenantState(tenantCtx).StripePriceId) != ""
}

func isStripeWebhookConfigured(tenantCtx context.Context) bool {
	return strings.TrimSpace(setting.TenantState(tenantCtx).StripeWebhookSecret) != ""
}

func isStripeWebhookEnabled(tenantCtx context.Context) bool {
	return isStripeTopUpEnabled(tenantCtx)
}

func isCreemTopUpEnabled(tenantCtx context.Context) bool {
	if !isPaymentComplianceConfirmed(tenantCtx) {
		return false
	}
	products := strings.TrimSpace(setting.TenantState(tenantCtx).CreemProducts)
	return strings.TrimSpace(setting.TenantState(tenantCtx).CreemApiKey) != "" &&
		products != "" &&
		products != "[]"
}

func isCreemWebhookConfigured(tenantCtx context.Context) bool {
	return strings.TrimSpace(setting.TenantState(tenantCtx).CreemWebhookSecret) != ""
}

func isCreemWebhookEnabled(tenantCtx context.Context) bool {
	return isCreemTopUpEnabled(tenantCtx) && isCreemWebhookConfigured(tenantCtx)
}

func isWaffoTopUpEnabled(tenantCtx context.Context) bool {
	if !isPaymentComplianceConfirmed(tenantCtx) {
		return false
	}
	if !setting.TenantState(tenantCtx).WaffoEnabled {
		return false
	}

	return isWaffoWebhookConfigured(tenantCtx)
}

func isWaffoWebhookConfigured(tenantCtx context.Context) bool {
	if setting.TenantState(tenantCtx).WaffoSandbox {
		return strings.TrimSpace(setting.TenantState(tenantCtx).WaffoSandboxApiKey) != "" &&
			strings.TrimSpace(setting.TenantState(tenantCtx).WaffoSandboxPrivateKey) != "" &&
			strings.TrimSpace(setting.TenantState(tenantCtx).WaffoSandboxPublicCert) != ""
	}

	return strings.TrimSpace(setting.TenantState(tenantCtx).WaffoApiKey) != "" &&
		strings.TrimSpace(setting.TenantState(tenantCtx).WaffoPrivateKey) != "" &&
		strings.TrimSpace(setting.TenantState(tenantCtx).WaffoPublicCert) != ""
}

func isWaffoWebhookEnabled(tenantCtx context.Context) bool {
	return isWaffoTopUpEnabled(tenantCtx)
}

func isWaffoPancakeTopUpEnabled(tenantCtx context.Context) bool {
	if !isPaymentComplianceConfirmed(tenantCtx) {
		return false
	}
	// Presence-of-credentials = enabled. Webhook public keys ship inside
	// the SDK; mode (test/prod) is read from each event.
	return strings.TrimSpace(setting.TenantState(tenantCtx).WaffoPancakeMerchantID) != "" &&
		strings.TrimSpace(setting.TenantState(tenantCtx).WaffoPancakePrivateKey) != "" &&
		strings.TrimSpace(setting.TenantState(tenantCtx).WaffoPancakeProductID) != ""
}

func isWaffoPancakeWebhookConfigured(tenantCtx context.Context) bool {
	return isWaffoPancakeTopUpEnabled(tenantCtx)
}

func isWaffoPancakeWebhookEnabled(tenantCtx context.Context) bool {
	return isWaffoPancakeTopUpEnabled(tenantCtx)
}

func isEpayTopUpEnabled(tenantCtx context.Context) bool {
	if !isPaymentComplianceConfirmed(tenantCtx) {
		return false
	}
	return isEpayWebhookConfigured(tenantCtx) && len(operation_setting.TenantState(tenantCtx).PayMethods) > 0
}

func isEpayWebhookConfigured(tenantCtx context.Context) bool {
	return strings.TrimSpace(operation_setting.TenantState(tenantCtx).PayAddress) != "" &&
		strings.TrimSpace(operation_setting.TenantState(tenantCtx).EpayId) != "" &&
		strings.TrimSpace(operation_setting.TenantState(tenantCtx).EpayKey) != ""
}

func isEpayWebhookEnabled(tenantCtx context.Context) bool {
	return isEpayTopUpEnabled(tenantCtx)
}
