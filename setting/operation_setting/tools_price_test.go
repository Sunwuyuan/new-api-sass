package operation_setting

import (
	config "github.com/QuantumNous/new-api/setting/config"

	"maps"
	"math"
	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveToolPrices(t *testing.T) {
	t.Helper()
	original := make(map[string]float64, len((config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices))
	maps.Copy(original, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices)
	t.Cleanup(func() {
		(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices = original
		RebuildToolPriceIndex(testtenant.Context())
	})
}

func TestToolPriceHardcodedFallbacksSurviveMissingOperatorConfig(t *testing.T) {
	preserveToolPrices(t)
	(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices = map[string]float64{}
	RebuildToolPriceIndex(testtenant.Context())

	expectedDefaults := map[string]float64{
		"web_search":         10,
		"web_search_preview": 10,
		"file_search":        2.5,
		"google_search":      14,
		"image_generation":   150,
	}
	for name, expected := range expectedDefaults {
		assert.Equal(t, expected, GetToolPrice(testtenant.Context(), name), name)
	}
	assert.Equal(t, 25.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "gpt-4o-2024-11-20"))
	assert.Equal(t, 25.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "gpt-4.1-mini"))
}

func TestToolPriceOperatorOverridePrecedenceAndExplicitZero(t *testing.T) {
	preserveToolPrices(t)
	(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices = map[string]float64{
		"image_generation":                 0,
		"web_search":                       12,
		"web_search_preview":               0,
		"web_search_preview:gpt-4o*":       30,
		"web_search_preview:gpt-4o-mini*":  0,
		"web_search_preview:custom-model*": 7,
	}
	RebuildToolPriceIndex(testtenant.Context())

	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "image_generation"))
	assert.Equal(t, 12.0, GetToolPrice(testtenant.Context(), "web_search"))
	assert.Equal(t, 0.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "o1"))
	assert.Equal(t, 30.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "gpt-4o"))
	assert.Equal(t, 0.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "gpt-4o-mini"))
	assert.Equal(t, 25.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "gpt-4.1"))
	assert.Equal(t, 7.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "custom-model-v2"))

	delete((config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices, "web_search_preview:gpt-4o*")
	RebuildToolPriceIndex(testtenant.Context())
	assert.Equal(t, 25.0, GetToolPriceForModel(testtenant.Context(), "web_search_preview", "gpt-4o"))

	delete((config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices, "web_search")
	RebuildToolPriceIndex(testtenant.Context())
	assert.Equal(t, 10.0, GetToolPrice(testtenant.Context(), "web_search"))
}

func TestToolPriceCustomFunctionHasNoHardcodedFallback(t *testing.T) {
	preserveToolPrices(t)
	(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices = map[string]float64{}
	RebuildToolPriceIndex(testtenant.Context())

	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "lookup_customer"))

	(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices["lookup_customer"] = 5
	RebuildToolPriceIndex(testtenant.Context())
	assert.Equal(t, 5.0, GetToolPrice(testtenant.Context(), "lookup_customer"))

	(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices["lookup_customer"] = 0
	RebuildToolPriceIndex(testtenant.Context())
	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "lookup_customer"))
}

func TestValidateToolPricesJSON(t *testing.T) {
	valid := []string{
		`{}`,
		`{"web_search":0}`,
		`{"web_search":10,"custom_fn":2.5}`,
	}
	for _, value := range valid {
		assert.NoError(t, ValidateToolPricesJSON(value), value)
	}

	invalid := []string{
		`null`,
		`[]`,
		`{"web_search":null}`,
		`{"web_search":true}`,
		`{"web_search":"0"}`,
		`{"web_search":-1}`,
		`{"web_search":1e999}`,
		`{"web_search":`,
	}
	for _, value := range invalid {
		assert.Error(t, ValidateToolPricesJSON(value), value)
	}
}

func TestLoadToolPricesFromJSONStringReplacesMapAndKeepsValidSiblings(t *testing.T) {
	preserveToolPrices(t)

	LoadToolPricesFromJSONString(testtenant.Context(), `{
		"web_search": 0,
		"custom_fn": 3,
		"file_search": null,
		"google_search": -1,
		"image_generation": "0"
	}`)

	require.Len(t, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices, 2)
	assert.Equal(t, 0.0, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices["web_search"])
	assert.Equal(t, 3.0, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices["custom_fn"])
	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "web_search"))
	assert.Equal(t, 3.0, GetToolPrice(testtenant.Context(), "custom_fn"))
	assert.Equal(t, 2.5, GetToolPrice(testtenant.Context(), "file_search"))
	assert.Equal(t, 14.0, GetToolPrice(testtenant.Context(), "google_search"))
	assert.Equal(t, 150.0, GetToolPrice(testtenant.Context(), "image_generation"))

	LoadToolPricesFromJSONString(testtenant.Context(), `{"image_generation":0}`)
	require.Len(t, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices, 1)
	assert.NotContains(t, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices, "web_search")
	assert.NotContains(t, (config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices, "custom_fn")
	assert.Equal(t, 10.0, GetToolPrice(testtenant.Context(), "web_search"))
	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "custom_fn"))
	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "image_generation"))
}

func TestRebuildToolPriceIndexIgnoresInvalidDirectValues(t *testing.T) {
	preserveToolPrices(t)
	(config.GlobalConfig.ForTenant(testtenant.Context()).Get("tool_price_setting").(*ToolPriceSetting)).Prices = map[string]float64{
		"web_search":       -1,
		"file_search":      math.Inf(1),
		"image_generation": math.NaN(),
		"custom_fn":        math.NaN(),
	}
	RebuildToolPriceIndex(testtenant.Context())

	assert.Equal(t, 10.0, GetToolPrice(testtenant.Context(), "web_search"))
	assert.Equal(t, 2.5, GetToolPrice(testtenant.Context(), "file_search"))
	assert.Equal(t, 150.0, GetToolPrice(testtenant.Context(), "image_generation"))
	assert.Equal(t, 0.0, GetToolPrice(testtenant.Context(), "custom_fn"))
}
