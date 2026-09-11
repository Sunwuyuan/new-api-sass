package controller

import (
	"maps"
	"testing"

	"github.com/QuantumNous/new-api/common"
	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestFormatWaffoPancakeAmount_UsesDisplayPriceString(t *testing.T) {
	testCases := []struct {
		name     string
		amount   float64
		expected string
	}{
		{name: "whole amount", amount: 29, expected: "29.00"},
		{name: "decimal amount", amount: 29.9, expected: "29.90"},
		{name: "round half up to cents", amount: 29.999, expected: "30.00"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, formatWaffoPancakeAmount(tc.amount))
		})
	}
}

func TestGetWaffoPancakePayMoney(t *testing.T) {
	originalUnitPrice := setting.TenantState(testtenant.Context()).WaffoPancakeUnitPrice
	originalQuotaDisplayType := operation_setting.GetGeneralSetting(testtenant.Context()).QuotaDisplayType
	originalDiscounts := make(map[int]float64, len(operation_setting.GetPaymentSetting(testtenant.Context()).AmountDiscount))
	maps.Copy(originalDiscounts, operation_setting.GetPaymentSetting(testtenant.Context()).AmountDiscount)
	originalTopupGroupRatio := common.TopupGroupRatio2JSONString(testtenant.Context())

	t.Cleanup(func() {
		setting.TenantState(testtenant.Context()).WaffoPancakeUnitPrice = originalUnitPrice
		operation_setting.GetGeneralSetting(testtenant.Context()).QuotaDisplayType = originalQuotaDisplayType
		operation_setting.GetPaymentSetting(testtenant.Context()).AmountDiscount = originalDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(testtenant.Context(), originalTopupGroupRatio))
	})

	setting.TenantState(testtenant.Context()).WaffoPancakeUnitPrice = 2.5
	operation_setting.GetPaymentSetting(testtenant.Context()).AmountDiscount = map[int]float64{
		10: 0.8,
		int(common.TenantState(testtenant.Context()).QuotaPerUnit * 3): 0.5,
		20: 0,
	}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(testtenant.Context(), `{"default":1,"vip":1.2}`))

	testCases := []struct {
		name             string
		amount           int64
		group            string
		quotaDisplayType string
		expected         float64
	}{
		{
			name:             "currency display applies unit price group ratio and discount",
			amount:           10,
			group:            "vip",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         24,
		},
		{
			name:             "tokens display converts quota to display units before pricing",
			amount:           int64(common.TenantState(testtenant.Context()).QuotaPerUnit * 3),
			group:            "vip",
			quotaDisplayType: operation_setting.QuotaDisplayTypeTokens,
			expected:         4.5,
		},
		{
			name:             "non-positive discount falls back to no discount",
			amount:           20,
			group:            "default",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			expected:         50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			operation_setting.GetGeneralSetting(testtenant.Context()).QuotaDisplayType = tc.quotaDisplayType
			actual := getWaffoPancakePayMoney(testtenant.Context(), tc.amount, tc.group)
			require.InDelta(t, tc.expected, actual, 0.000001)
		})
	}
}
