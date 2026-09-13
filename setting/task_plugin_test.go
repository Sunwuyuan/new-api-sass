package setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTaskPluginDisabledFactoryKeysTest(t *testing.T) {
	t.Helper()
	originalMap := common.TenantState(testtenant.Context()).OptionMap
	common.TenantState(testtenant.Context()).OptionMapRWMutex.Lock()
	common.TenantState(testtenant.Context()).OptionMap = map[string]string{}
	common.TenantState(testtenant.Context()).OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.TenantState(testtenant.Context()).OptionMapRWMutex.Lock()
		common.TenantState(testtenant.Context()).OptionMap = originalMap
		common.TenantState(testtenant.Context()).OptionMapRWMutex.Unlock()
	})
}

func TestTaskPluginDisabledFactoryKeysRoundTripAndDedupe(t *testing.T) {
	setupTaskPluginDisabledFactoryKeysTest(t)

	assert.Empty(t, GetTaskPluginDisabledFactoryKeys(testtenant.Context()))
	assert.False(t, IsTaskPluginFactoryDisabled(testtenant.Context(), "kling"))

	require.NoError(t, SetTaskPluginDisabledFactoryKeysOption(testtenant.Context(), []string{"kling", "sora", "kling", " hailuo "}))
	assert.Equal(t, []string{"hailuo", "kling", "sora"}, GetTaskPluginDisabledFactoryKeys(testtenant.Context()))
	assert.Equal(t, `["hailuo","kling","sora"]`, common.TenantState(testtenant.Context()).OptionMap[TaskPluginDisabledFactoryKeysKey])
	assert.True(t, IsTaskPluginFactoryDisabled(testtenant.Context(), "kling"))
	assert.True(t, IsTaskPluginFactoryDisabled(testtenant.Context(), "hailuo"))
	assert.False(t, IsTaskPluginFactoryDisabled(testtenant.Context(), "google"))
}

func TestTaskPluginDisabledFactoryKeysBadJSONReturnsEmpty(t *testing.T) {
	setupTaskPluginDisabledFactoryKeysTest(t)

	for _, testCase := range []struct {
		name string
		raw  string
	}{
		{name: "absent", raw: ""},
		{name: "null", raw: "null"},
		{name: "object", raw: "{}"},
		{name: "number", raw: "1"},
		{name: "truncated", raw: `["kling"`},
		{name: "not json", raw: "kling"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			common.TenantState(testtenant.Context()).OptionMapRWMutex.Lock()
			if testCase.raw == "" {
				delete(common.TenantState(testtenant.Context()).OptionMap, TaskPluginDisabledFactoryKeysKey)
			} else {
				common.TenantState(testtenant.Context()).OptionMap[TaskPluginDisabledFactoryKeysKey] = testCase.raw
			}
			common.TenantState(testtenant.Context()).OptionMapRWMutex.Unlock()

			assert.Empty(t, GetTaskPluginDisabledFactoryKeys(testtenant.Context()))
			assert.False(t, IsTaskPluginFactoryDisabled(testtenant.Context(), "kling"))
		})
	}
}
