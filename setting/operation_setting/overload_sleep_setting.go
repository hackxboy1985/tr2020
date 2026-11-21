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
	// FallbackModels 备用模型映射，每行一条「原模型=备用模型」。
	// 仅当原模型的全部渠道都在休眠池中、且请求所用 API 密钥开启了
	// 「使用相近模型兜底」时，才把请求改走备用模型自己的渠道。
	// 示例：
	//   gemini-3.6-flash=gemini-3.7-flash
	//   gpt-4o=gpt-4o-mini
	FallbackModels string `json:"fallback_models"`
}

var overloadSleepSetting = OverloadSleepSetting{
	Enabled:        false,
	SleepMinutes:   DefaultOverloadSleepMinutes,
	StatusCodes:    DefaultOverloadSleepStatusCodes,
	FallbackModels: "",
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

// ---- 备用模型映射 ----

// ParseFallbackModelMappings 解析「原模型=备用模型」多行配置。
// 规则：
//   - 每行一条，忽略空行与 # 开头的注释行；
//   - 使用第一个 '=' 切分，两侧去空白；
//   - 原模型与备用模型都不能为空；
//   - 同一原模型重复配置时，后出现的覆盖先出现的。
func ParseFallbackModelMappings(raw string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		origin := strings.TrimSpace(line[:idx])
		fallback := strings.TrimSpace(line[idx+1:])
		if origin == "" || fallback == "" || origin == fallback {
			continue
		}
		result[origin] = fallback
	}
	return result
}

// 备用模型映射解析缓存：
// 解析发生在请求热路径上（每次命中「全部渠道休眠」都会调用），
// 因此按「原始配置字符串」缓存解析结果，配置不变时直接复用。
var (
	fallbackModelsMutex sync.RWMutex
	fallbackModelsRaw   string
	fallbackModelsCache map[string]string
)

// getFallbackModelMappings 返回当前生效的备用模型映射（带缓存）。
// 返回的 map 视为只读，调用方不得修改。
func getFallbackModelMappings() map[string]string {
	raw := overloadSleepSetting.FallbackModels
	fallbackModelsMutex.RLock()
	if fallbackModelsRaw == raw && fallbackModelsCache != nil {
		cached := fallbackModelsCache
		fallbackModelsMutex.RUnlock()
		return cached
	}
	fallbackModelsMutex.RUnlock()

	parsed := ParseFallbackModelMappings(raw)

	fallbackModelsMutex.Lock()
	fallbackModelsRaw = raw
	fallbackModelsCache = parsed
	fallbackModelsMutex.Unlock()
	return parsed
}

// GetFallbackModel 返回原模型配置的备用模型；未配置时返回空字符串。
func GetFallbackModel(originModel string) string {
	if originModel == "" {
		return ""
	}
	return getFallbackModelMappings()[originModel]
}

// FallbackModelsToString 把映射格式化为多行文本（便于配置回显/归一化）。
func FallbackModelsToString(mappings map[string]string) string {
	if len(mappings) == 0 {
		return ""
	}
	origins := make([]string, 0, len(mappings))
	for origin := range mappings {
		origins = append(origins, origin)
	}
	sort.Strings(origins)
	lines := make([]string, 0, len(origins))
	for _, origin := range origins {
		lines = append(lines, origin+"="+mappings[origin])
	}
	return strings.Join(lines, "\n")
}

// DescribeFallbackModelCycle 分析「原模型 -> 备用模型」之间是否存在
// 会让**本次请求来回打转**的死循环，返回环路描述（无环返回空字符串）。
//
// 判定标准（关键）：只有当链路**最终绕回 originModel 本身**时才算 originModel 处于环中。
// 例：
//   - a=b、b=a：从 a 出发得到 a -> b -> a，绕回起点 → 判定为环（兜底无意义）；
//   - a=b、b=c、c=a：绕回起点 → 判定为环；
//   - a=b、b=c、c=b：从 a 出发进入 b<->c 的环，但**不会绕回 a**。
//     此时把 a 兜底到 b 仍然有意义（b 的渠道可用就能成功），因此 a 不算处于环中。
//     旧实现会把这种情况误判为环并拒绝 a 的兜底，属于误杀。
//
// 注意：这仅检测「全局备用模型配置」层面的环；
// 渠道自带的模型映射是否构成环，由 relay 层（ChannelModelMappingReturnsToOrigin）检测。
func DescribeFallbackModelCycle(originModel string) string {
	if originModel == "" {
		return ""
	}
	mappings := getFallbackModelMappings()
	if len(mappings) == 0 {
		return ""
	}
	path := []string{originModel}
	// visited 只记录「除起点外」已经过的节点，用于识别链路内部的环（避免无限遍历）。
	visited := make(map[string]struct{}, len(mappings))
	current := originModel
	for {
		next, ok := mappings[current]
		if !ok || next == "" {
			return ""
		}
		path = append(path, next)
		// 只有绕回起点才算 originModel 处于环中
		if next == originModel {
			return strings.Join(path, " -> ")
		}
		if _, seen := visited[next]; seen {
			// 进入了与 originModel 无关的环：originModel 本身仍可安全兜底一次
			return ""
		}
		visited[next] = struct{}{}
		current = next
	}
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
