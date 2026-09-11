package operation_setting

import (
	config "github.com/QuantumNous/new-api/setting/config"

	"testing"

	testtenant "github.com/QuantumNous/new-api/internal/testtenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMonitorSetting_ChannelTestEnabledEnvOverridesEnabledConfig(t *testing.T) {
	orig := *(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting))
	t.Cleanup(func() {
		*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = orig
	})

	t.Setenv("CHANNEL_TEST_ENABLED", "false")
	t.Setenv("CHANNEL_TEST_FREQUENCY", "5")
	*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = MonitorSetting{
		AutoTestChannelEnabled: true,
		AutoTestChannelMinutes: 20,
	}

	setting := GetMonitorSetting(testtenant.Context())

	require.NotNil(t, setting)
	assert.False(t, setting.AutoTestChannelEnabled)
	assert.Equal(t, float64(5), setting.AutoTestChannelMinutes)
}

func TestGetMonitorSetting_ChannelTestEnabledEnvCanEnableDisabledConfig(t *testing.T) {
	orig := *(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting))
	t.Cleanup(func() {
		*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = orig
	})

	t.Setenv("CHANNEL_TEST_ENABLED", "true")
	*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = MonitorSetting{
		AutoTestChannelEnabled: false,
		AutoTestChannelMinutes: 12,
	}

	setting := GetMonitorSetting(testtenant.Context())

	require.NotNil(t, setting)
	assert.True(t, setting.AutoTestChannelEnabled)
	assert.Equal(t, float64(12), setting.AutoTestChannelMinutes)
}

func TestGetMonitorSettingPreservesAutoBanOnlyMode(t *testing.T) {
	orig := *(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting))
	t.Cleanup(func() {
		*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = orig
	})

	t.Setenv("CHANNEL_TEST_ENABLED", "")
	t.Setenv("CHANNEL_TEST_FREQUENCY", "")
	*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = MonitorSetting{ChannelTestMode: ChannelTestModeAutoBanOnly}

	setting := GetMonitorSetting(testtenant.Context())

	require.NotNil(t, setting)
	assert.Equal(t, ChannelTestModeAutoBanOnly, setting.ChannelTestMode)
}

func TestGetMonitorSettingNormalizesChannelTestConcurrency(t *testing.T) {
	orig := *(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting))
	t.Cleanup(func() {
		*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = orig
	})

	tests := []struct {
		name        string
		concurrency int
		want        int
	}{
		{name: "missing uses safe default", concurrency: 0, want: DefaultChannelTestConcurrency},
		{name: "configured value is preserved", concurrency: 8, want: 8},
		{name: "oversized value is capped", concurrency: MaxChannelTestConcurrency + 1, want: MaxChannelTestConcurrency},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			*(config.GlobalConfig.ForTenant(testtenant.Context()).Get("monitor_setting").(*MonitorSetting)) = MonitorSetting{ChannelTestConcurrency: test.concurrency}

			setting := GetMonitorSetting(testtenant.Context())

			require.NotNil(t, setting)
			assert.Equal(t, test.want, setting.ChannelTestConcurrency)
		})
	}
}

func TestValidateChannelTestConcurrency(t *testing.T) {
	require.NoError(t, ValidateChannelTestConcurrency("1"))
	require.NoError(t, ValidateChannelTestConcurrency("32"))
	assert.Error(t, ValidateChannelTestConcurrency("0"))
	assert.Error(t, ValidateChannelTestConcurrency("33"))
	assert.Error(t, ValidateChannelTestConcurrency("1.5"))
}
