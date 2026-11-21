package operation_setting

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

// 过载休眠池（Overload Sleep Pool）配置
//
// 需求：上游返回「过载」状态码（默认 429、503）后，将该模型 + 该渠道放入 Redis
// 休眠池，休眠指定分钟数。休眠期内后续请求命中该 (模型, 渠道) 组合时跳过该渠道，
// 直接使用下一渠道，并在日志表写一条「跳过」记录。
const (
	DefaultOverloadSleepMinutes = 2
	MinOverloadSleepMinutes     = 1
	MaxOverloadSleepMinutes     = 1440

	DefaultOverloadSleepStatusCodes = "429,503"
)

// ErrEmptyOverloadSleepStatusCodes 状态码配置为空时的错误
var ErrEmptyOverloadSleepStatusCodes = errors.New("过载状态码配置不能为空，示例：429,503 或 429,500-599")

// OverloadSleepSetting 过载休眠池配置，注册到全局配置管理器，
// 通过 /api/option/ 以 overload_sleep_setting.* 的形式读写。
type OverloadSleepSetting struct {
	// Enabled 总开关，关闭时整套流程不生效（不写池、不跳过、不写跳过日志）
	Enabled bool `json:"enabled"`
	// SleepMinutes 休眠分钟数，1-1440，非法值回落默认值
	SleepMinutes int `json:"sleep_minutes"`
	// StatusCodes 视为过载的状态码，逗号分隔，支持区间（如 429,500-599）
	StatusCodes string `json:"status_codes"`
}

var overloadSleepSetting = OverloadSleepSetting{
	Enabled:      false,
	SleepMinutes: DefaultOverloadSleepMinutes,
	StatusCodes:  DefaultOverloadSleepStatusCodes,
}

// 状态码解析结果缓存：避免每次请求都解析字符串
var (
	overloadSleepCodesMutex sync.RWMutex
	overloadSleepCodesRaw   string
	overloadSleepCodes      []StatusCodeRange
)

func init() {
	config.GlobalConfig.Register("overload_sleep_setting", &overloadSleepSetting)
	overloadSleepCodesRaw = overloadSleepSetting.StatusCodes
	overloadSleepCodes, _ = ParseHTTPStatusCodeRanges(overloadSleepSetting.StatusCodes)
}

// GetOverloadSleepSetting 获取配置对象
func GetOverloadSleepSetting() *OverloadSleepSetting {
	return &overloadSleepSetting
}

// IsOverloadSleepEnabled 总开关是否启用
func IsOverloadSleepEnabled() bool {
	return overloadSleepSetting.Enabled
}

// GetOverloadSleepMinutes 当前生效的休眠分钟数（已归一化）
func GetOverloadSleepMinutes() int {
	return normalizeOverloadSleepMinutes(overloadSleepSetting.SleepMinutes)
}

// NormalizeOverloadSleepSetting 归一化配置：分钟数夹取到合法区间，状态码规范化为区间字符串。
// 在配置加载/更新后调用（见 model.handleConfigUpdate）。
func NormalizeOverloadSleepSetting() {
	overloadSleepSetting.SleepMinutes = normalizeOverloadSleepMinutes(overloadSleepSetting.SleepMinutes)

	raw := strings.TrimSpace(overloadSleepSetting.StatusCodes)
	ranges, err := ParseHTTPStatusCodeRanges(raw)
	if err != nil || len(ranges) == 0 {
		// 空值或非法值：回落默认状态码，避免线上配置写坏后整套逻辑失效
		raw = DefaultOverloadSleepStatusCodes
		ranges, _ = ParseHTTPStatusCodeRanges(raw)
	} else {
		raw = OverloadSleepStatusCodesToString(ranges)
	}
	overloadSleepSetting.StatusCodes = raw
	setOverloadSleepCodes(raw, ranges)
}

// ValidateOverloadSleepStatusCodes 校验状态码配置，非法时返回错误（供 /api/option/ 拒绝保存）。
func ValidateOverloadSleepStatusCodes(raw string) error {
	ranges, err := ParseHTTPStatusCodeRanges(raw)
	if err != nil {
		return err
	}
	if len(ranges) == 0 {
		return ErrEmptyOverloadSleepStatusCodes
	}
	return nil
}

// IsOverloadStatusCode 判断状态码是否属于「过载」，总开关关闭时恒为 false。
func IsOverloadStatusCode(code int) bool {
	if !overloadSleepSetting.Enabled {
		return false
	}
	ensureOverloadSleepCodesParsed()
	overloadSleepCodesMutex.RLock()
	defer overloadSleepCodesMutex.RUnlock()
	return shouldMatchStatusCodeRanges(overloadSleepCodes, code)
}

func setOverloadSleepCodes(raw string, ranges []StatusCodeRange) {
	overloadSleepCodesMutex.Lock()
	defer overloadSleepCodesMutex.Unlock()
	overloadSleepCodesRaw = raw
	overloadSleepCodes = ranges
}

// ensureOverloadSleepCodesParsed 在配置字符串发生变化时重新解析，
// 保证 IsOverloadStatusCode 总是使用最新配置。
func ensureOverloadSleepCodesParsed() {
	overloadSleepCodesMutex.RLock()
	same := overloadSleepCodesRaw == overloadSleepSetting.StatusCodes
	overloadSleepCodesMutex.RUnlock()
	if same {
		return
	}
	ranges, err := ParseHTTPStatusCodeRanges(overloadSleepSetting.StatusCodes)
	if err != nil {
		ranges = nil
	}
	setOverloadSleepCodes(overloadSleepSetting.StatusCodes, ranges)
}

func normalizeOverloadSleepMinutes(minutes int) int {
	if minutes < MinOverloadSleepMinutes || minutes > MaxOverloadSleepMinutes {
		return DefaultOverloadSleepMinutes
	}
	return minutes
}

// OverloadSleepStatusCodesToString 将区间列表格式化为 "429,500-599" 形式。
func OverloadSleepStatusCodesToString(ranges []StatusCodeRange) string {
	if len(ranges) == 0 {
		return ""
	}
	sorted := make([]StatusCodeRange, len(ranges))
	copy(sorted, ranges)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	parts := make([]string, 0, len(sorted))
	for _, r := range sorted {
		if r.Start == r.End {
			parts = append(parts, strconv.Itoa(r.Start))
			continue
		}
		parts = append(parts, strconv.Itoa(r.Start)+"-"+strconv.Itoa(r.End))
	}
	return strings.Join(parts, ",")
}
