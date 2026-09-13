package service

import (
	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
)

func TestPaymentReturnURLUsesSuppliedDefaultDashboardPath(t *testing.T) {
	previousAddress := system_setting.TenantState(testtenant.Context()).ServerAddress
	system_setting.TenantState(testtenant.Context()).ServerAddress = "https://dashboard.example.com/"
	t.Cleanup(func() { system_setting.TenantState(testtenant.Context()).ServerAddress = previousAddress })

	assert.Equal(t, "https://dashboard.example.com/wallet", PaymentReturnURL(testtenant.Context(), "/wallet"))
}
