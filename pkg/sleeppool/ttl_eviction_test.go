package sleeppool

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// TestHashTTLNeverEvictsExistingEntry 复现「key 到期把仍在休眠期的渠道提前恢复」的问题。
//
// 场景：管理端把休眠时长从 10 分钟调小到 1 分钟；之后另一个渠道命中过载写入池子时，
// 如果 hash 的 TTL 按「当前配置 + 宽限」重算，就会把 hash 的到期时间缩短到
// 比「先进入池子的渠道」的 sleep_until 还早，导致该渠道被提前恢复。
func TestHashTTLNeverEvictsExistingEntry(t *testing.T) {
	server := setupMiniRedis(t)

	base := common.GetTimestamp()
	offset := int64(0)
	previousNow := nowFunc
	nowFunc = func() int64 { return base + offset }
	t.Cleanup(func() { nowFunc = previousNow })

	cfg := operation_setting.GetOverloadSleepSetting()

	// 1) 休眠时长 10 分钟时，渠道 1 进入休眠池
	cfg.SleepMinutes = 10
	operation_setting.NormalizeOverloadSleepSetting()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 0))

	// 2) 管理端把休眠时长调小到 1 分钟
	cfg.SleepMinutes = 1
	operation_setting.NormalizeOverloadSleepSetting()

	// 3) 5 分钟后渠道 2 也进入休眠池
	offset = 5 * 60
	server.FastForward(5 * time.Minute)
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 2}, 0))

	// 4) 再过 4 分钟（累计 9 分钟）：渠道 1 的 sleep_until 是 10 分钟，此时仍应在休眠中
	offset = 9 * 60
	server.FastForward(4 * time.Minute)

	sleeping, entry := IsSleeping("m", 1)
	require.NotNil(t, entry, "渠道 1 的记录不应因为 hash 提前过期而消失")
	require.True(t, sleeping, "渠道 1 的休眠不应因 hash 到期而被提前恢复（应睡到第 10 分钟）")
}
