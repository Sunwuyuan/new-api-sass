package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPluginEnabledOptionUpdatesRegistry(t *testing.T) {
	originalEnabled := constant.TenantRuntime(testtenant.Context()).TaskPluginEnabled
	originalMap := common.TenantState(testtenant.Context()).OptionMap
	common.TenantState(testtenant.Context()).OptionMap = map[string]string{}
	const key = "option-master-off"
	source := `
export const meta = {apiVersion: 1, key: "option-master-off", name: "Option Master", version: "1.0.0", author: {name: "Test"}, models: ["option-master-model"], fetchMode: "per_task"};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`
	_, err := jsplugin.TenantState(testtenant.Context()).DefaultRegistry.RegisterFactory(source, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		constant.TenantRuntime(testtenant.Context()).TaskPluginEnabled = originalEnabled
		jsplugin.TenantState(testtenant.Context()).DefaultRegistry.SetEnabled(originalEnabled)
		common.TenantState(testtenant.Context()).OptionMap = originalMap
	})

	_, ok := jsplugin.TenantState(testtenant.Context()).DefaultRegistry.Get(key)
	require.True(t, ok)

	require.NoError(t, updateOptionMap(testtenant.Context(), "TaskPluginEnabled", "false"))

	assert.False(t, constant.TenantRuntime(testtenant.Context()).TaskPluginEnabled)
	assert.Equal(t, "false", common.TenantState(testtenant.Context()).OptionMap["TaskPluginEnabled"])
	_, ok = jsplugin.TenantState(testtenant.Context()).DefaultRegistry.Get(key)
	assert.False(t, ok)

	require.NoError(t, updateOptionMap(testtenant.Context(), "TaskPluginEnabled", "true"))
	_, ok = jsplugin.TenantState(testtenant.Context()).DefaultRegistry.Get(key)
	assert.True(t, ok)
}

func TestTaskPluginDisabledFactoryKeysOptionUpdatesRegistry(t *testing.T) {
	originalMap := common.TenantState(testtenant.Context()).OptionMap
	common.TenantState(testtenant.Context()).OptionMap = map[string]string{}
	const key = "option-factory-off"
	source := `
export const meta = {apiVersion: 1, key: "option-factory-off", name: "Option Factory", version: "1.0.0", author: {name: "Test"}, models: ["option-factory-model"], fetchMode: "per_task"};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`
	_, err := jsplugin.TenantState(testtenant.Context()).DefaultRegistry.RegisterFactory(source, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		jsplugin.TenantState(testtenant.Context()).DefaultRegistry.SetDisabledFactoryKeys(nil)
		common.TenantState(testtenant.Context()).OptionMap = originalMap
	})

	_, ok := jsplugin.TenantState(testtenant.Context()).DefaultRegistry.Get(key)
	require.True(t, ok)

	require.NoError(t, updateOptionMap(testtenant.Context(), setting.TaskPluginDisabledFactoryKeysKey, `["option-factory-off"]`))

	assert.Equal(t, `["option-factory-off"]`, common.TenantState(testtenant.Context()).OptionMap[setting.TaskPluginDisabledFactoryKeysKey])
	_, ok = jsplugin.TenantState(testtenant.Context()).DefaultRegistry.Get(key)
	assert.False(t, ok)
	assert.Equal(t, []string{key}, jsplugin.TenantState(testtenant.Context()).DefaultRegistry.Snapshot().DisabledFactory)
}
