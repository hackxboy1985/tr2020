package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOverloadSleepSetting_Defaults(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 0, StatusCodes: ""}
	NormalizeOverloadSleepSetting()

	require.Equal(t, DefaultOverloadSleepMinutes, overloadSleepSetting.SleepMinutes)
	require.Equal(t, DefaultOverloadSleepStatusCodes, overloadSleepSetting.StatusCodes)
}

func TestNormalizeOverloadSleepSetting_ClampMinutes(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 99999, StatusCodes: "429"}
	NormalizeOverloadSleepSetting()
	require.Equal(t, DefaultOverloadSleepMinutes, overloadSleepSetting.SleepMinutes)

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 7, StatusCodes: "429"}
	NormalizeOverloadSleepSetting()
	require.Equal(t, 7, overloadSleepSetting.SleepMinutes)
}

func TestNormalizeOverloadSleepSetting_NormalizesStatusCodes(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 2, StatusCodes: " 503 , 429 ,429"}
	NormalizeOverloadSleepSetting()
	require.Equal(t, "429,503", overloadSleepSetting.StatusCodes)
}

func TestIsOverloadStatusCode_DisabledBySwitch(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting = OverloadSleepSetting{Enabled: false, SleepMinutes: 2, StatusCodes: "429,503"}
	NormalizeOverloadSleepSetting()
	require.False(t, IsOverloadStatusCode(429))
	require.False(t, IsOverloadStatusCode(503))
}

func TestIsOverloadStatusCode_MatchesConfiguredCodes(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 2, StatusCodes: "429,503"}
	NormalizeOverloadSleepSetting()
	require.True(t, IsOverloadStatusCode(429))
	require.True(t, IsOverloadStatusCode(503))
	require.False(t, IsOverloadStatusCode(500))
	require.False(t, IsOverloadStatusCode(200))
}

func TestIsOverloadStatusCode_SupportsRanges(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 2, StatusCodes: "429,500-599"}
	NormalizeOverloadSleepSetting()
	require.True(t, IsOverloadStatusCode(429))
	require.True(t, IsOverloadStatusCode(500))
	require.True(t, IsOverloadStatusCode(599))
	require.False(t, IsOverloadStatusCode(499))
}

func TestIsOverloadStatusCode_FollowsConfigChangeWithoutNormalize(t *testing.T) {
	original := overloadSleepSetting
	originalRaw, originalCodes := overloadSleepCodesRaw, overloadSleepCodes
	t.Cleanup(func() {
		overloadSleepSetting = original
		overloadSleepCodesRaw, overloadSleepCodes = originalRaw, originalCodes
	})

	overloadSleepSetting = OverloadSleepSetting{Enabled: true, SleepMinutes: 2, StatusCodes: "429"}
	NormalizeOverloadSleepSetting()
	require.True(t, IsOverloadStatusCode(429))
	require.False(t, IsOverloadStatusCode(503))

	// 直接改内存配置（模拟配置更新路径之一）后无需显式调用 Normalize 也应生效
	overloadSleepSetting.StatusCodes = "503"
	require.False(t, IsOverloadStatusCode(429))
	require.True(t, IsOverloadStatusCode(503))
}

func TestValidateOverloadSleepStatusCodes(t *testing.T) {
	require.NoError(t, ValidateOverloadSleepStatusCodes("429,503"))
	require.NoError(t, ValidateOverloadSleepStatusCodes("429,500-599"))
	require.Error(t, ValidateOverloadSleepStatusCodes(""))
	require.Error(t, ValidateOverloadSleepStatusCodes("abc"))
	require.Error(t, ValidateOverloadSleepStatusCodes("99"))
}

func TestGetOverloadSleepMinutes(t *testing.T) {
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })

	overloadSleepSetting.SleepMinutes = 2
	require.Equal(t, 2, GetOverloadSleepMinutes())

	overloadSleepSetting.SleepMinutes = -1
	require.Equal(t, DefaultOverloadSleepMinutes, GetOverloadSleepMinutes())
}
