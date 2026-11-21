package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

// TestOverloadSleepSettingRegistered 验证配置已注册到全局配置管理器，
// 并且能以 overload_sleep_setting.* 的形式被 /api/option/ 读写。
func TestOverloadSleepSettingRegistered(t *testing.T) {
	cfg := config.GlobalConfig.Get("overload_sleep_setting")
	require.NotNil(t, cfg)
	require.Same(t, GetOverloadSleepSetting(), cfg)

	exported := config.GlobalConfig.ExportAllConfigs()
	require.Contains(t, exported, "overload_sleep_setting.enabled")
	require.Contains(t, exported, "overload_sleep_setting.sleep_minutes")
	require.Contains(t, exported, "overload_sleep_setting.status_codes")
}

// TestOverloadSleepSettingExportedDefaults 验证默认值符合需求：
// 总开关默认关闭、休眠默认 2 分钟、过载状态码默认 429,503。
func TestOverloadSleepSettingExportedDefaults(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() {
		overloadSleepSetting = original
		NormalizeOverloadSleepSetting()
	})

	overloadSleepSetting = OverloadSleepSetting{
		Enabled:      false,
		SleepMinutes: DefaultOverloadSleepMinutes,
		StatusCodes:  DefaultOverloadSleepStatusCodes,
	}
	NormalizeOverloadSleepSetting()

	exported := config.GlobalConfig.ExportAllConfigs()
	require.Equal(t, "false", exported["overload_sleep_setting.enabled"])
	require.Equal(t, "2", exported["overload_sleep_setting.sleep_minutes"])
	require.Equal(t, "429,503", exported["overload_sleep_setting.status_codes"])
}

// TestOverloadSleepSettingUpdateViaConfig 验证通过配置管理器更新后，
// 运行时的状态码判定与休眠时长立即生效（模拟 /api/option/ PUT 的路径）。
func TestOverloadSleepSettingUpdateViaConfig(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() {
		overloadSleepSetting = original
		NormalizeOverloadSleepSetting()
	})

	cfg := config.GlobalConfig.Get("overload_sleep_setting")
	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{
		"enabled":       "true",
		"sleep_minutes": "7",
		"status_codes":  "500-599",
	}))
	// 模拟 model.handleConfigUpdate 的后处理
	NormalizeOverloadSleepSetting()

	require.True(t, IsOverloadSleepEnabled())
	require.Equal(t, 7, GetOverloadSleepMinutes())
	require.True(t, IsOverloadStatusCode(503))
	require.False(t, IsOverloadStatusCode(429))
}
