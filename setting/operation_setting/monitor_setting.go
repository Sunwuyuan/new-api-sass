package operation_setting

import context "context"

import (
	"fmt"
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes float64 `json:"auto_test_channel_minutes"`
	ChannelTestMode        string  `json:"channel_test_mode"`
	ChannelTestConcurrency int     `json:"channel_test_concurrency"`
}

const (
	ChannelTestModeScheduledAll    = "scheduled_all"
	ChannelTestModeAutoBanOnly     = "auto_ban_only"
	ChannelTestModePassiveRecovery = "passive_recovery"

	ChannelTestConcurrencyOptionKey = "monitor_setting.channel_test_concurrency"
	DefaultChannelTestConcurrency   = 1
	MaxChannelTestConcurrency       = 32
)

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled: false,
	AutoTestChannelMinutes: 10,
	ChannelTestMode:        ChannelTestModeScheduledAll,
	ChannelTestConcurrency: DefaultChannelTestConcurrency,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting(tenantCtx context.Context) *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			(*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).AutoTestChannelEnabled = true
			(*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).AutoTestChannelMinutes = float64(frequency)
			(*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).ChannelTestMode = ChannelTestModeScheduledAll
		}
	}
	if enabled, ok := os.LookupEnv("CHANNEL_TEST_ENABLED"); ok {
		parsed, err := strconv.ParseBool(enabled)
		if err == nil {
			(*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).AutoTestChannelEnabled = parsed
		}
	}
	switch (*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).ChannelTestMode {
	case ChannelTestModeAutoBanOnly, ChannelTestModePassiveRecovery:
	default:
		(*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).ChannelTestMode = ChannelTestModeScheduledAll
	}
	(*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).ChannelTestConcurrency = NormalizeChannelTestConcurrency((*(config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting))).ChannelTestConcurrency)
	return config.GlobalConfig.ForTenant(tenantCtx).Get("monitor_setting").(*MonitorSetting)
}

func NormalizeChannelTestConcurrency(concurrency int) int {
	if concurrency < 1 {
		return DefaultChannelTestConcurrency
	}
	if concurrency > MaxChannelTestConcurrency {
		return MaxChannelTestConcurrency
	}
	return concurrency
}

func ValidateChannelTestConcurrency(value string) error {
	concurrency, err := strconv.Atoi(value)
	if err != nil || concurrency < 1 || concurrency > MaxChannelTestConcurrency {
		return fmt.Errorf("channel test concurrency must be between 1 and %d", MaxChannelTestConcurrency)
	}
	return nil
}
