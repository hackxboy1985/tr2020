package td

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ============================
// 视频模型能力矩阵
// ============================

// videoModelSpec 描述一个上游视频模型的能力边界。
//
// 为什么用一张表而不是散落的 if：
// 土豆（Td）的 /v1/videos/generations 是三模型共用的同一套协议，
// 模型之间的差异**只体现在参数取值边界上**（时长范围、素材上限、分辨率档位），
// 因此把差异集中为数据，新增同协议模型时只需追加一条记录。
//
// key 为**映射后的上游模型名**（info.UpstreamModelName）。
// 用户在渠道里配置的别名（model_mapping）在校验前会被解析成上游名再查表，
// 因此别名与上游名都能正确命中。
type videoModelSpec struct {
	// 时长（秒）。
	// 范围模式：MinDuration/MaxDuration 生效（存量三模型）。
	// 枚举模式：AllowedDurations 非空时优先，范围字段被忽略（grok）。
	MinDuration     int
	MaxDuration     int
	DefaultDuration int

	// AllowedDurations 时长枚举白名单；非空时优先于 Min/Max 范围校验。
	//
	// 为什么需要：grok 的时长是**离散档位**（6/10/15）而非连续区间，
	// 用 Min=6/Max=15 会放行 7~14 这些上游不支持的取值并透传，
	// 与本渠道「边界不静默透传」的原则冲突。
	AllowedDurations []int

	// RequireDuration 是否强制要求显式传 duration。
	// grok 上游文档将 duration 标为必填；存量三模型可缺省（填 DefaultDuration）。
	RequireDuration bool

	// 允许的分辨率档位（小写字面量，与上游参数值一致）
	AllowedResolutions []string

	// AllowedAspectRatios 该模型允许的画面比例；为空时回退到通用集合
	// allowedAspectRatios。grok 额外支持 1:1，故按模型区分而非全局统一。
	AllowedAspectRatios []string

	// 素材数量上限；0 表示该模型不支持此类素材，传入即拒绝
	MaxImages int
	MaxVideos int
	MaxAudios int

	// MaxExtraReferenceImages extra.reference_images 的数量上限。
	// 0 表示不启用该通道校验 —— 存量三模型即为此值，
	// 从而完全不进入 extra 参考图校验逻辑（这是隔离的关键，勿改成布尔标志）。
	MaxExtraReferenceImages int

	// 音效同步开关是否可用
	AllowGenerateAudio bool
}

// 各模型默认时长（两份上游文档均为 4 秒）
const defaultVideoDuration = 4

// 画面比例：三模型共用（grok 另在 spec 中声明含 1:1 的集合）
var allowedAspectRatios = []string{"16:9", "9:16"}

// videoModelSpecs 已知视频模型能力矩阵。
//
// 依上游文档逐项登记：
//   - minimax-h3   : 3-15s, 720p/1080p/2k, 9 图, 不支持参考视频/音频, 无 generate_audio
//   - seedance-2.0 : 3-15s, 仅 720p,      9 图, 3 视频, 3 音频, 支持 generate_audio
//   - seedance-2.5 : 4-30s, 仅 720p,     30 图, 10 视频, 10 音频, 支持 generate_audio
//   - grok-imagine-video-1.5 : 时长枚举 6/10/15（必填）, 仅 720p,
//     16:9/9:16/1:1, 参考图走 input_reference + extra.reference_images,
//     无 images 字段, 不支持参考视频/音频/generate_audio, extra 必填
//
// 未登记的模型一律拒绝（不静默透传），避免用户以为参数生效、实际被上游忽略。
var videoModelSpecs = map[string]videoModelSpec{
	VideoModelMiniMaxH3: {
		MinDuration:        3,
		MaxDuration:        15,
		DefaultDuration:    defaultVideoDuration,
		AllowedResolutions: []string{Resolution720P, Resolution1080P, Resolution2KVideo},
		MaxImages:          9,
		MaxVideos:          0, // 文档明确：提交 videos 返回 400
		MaxAudios:          0,
		AllowGenerateAudio: false, // 文档未列该参数，即不支持
	},
	VideoModelSeedance20: {
		MinDuration:        3,
		MaxDuration:        15,
		DefaultDuration:    defaultVideoDuration,
		AllowedResolutions: []string{Resolution720P},
		MaxImages:          9,
		MaxVideos:          3,
		MaxAudios:          3,
		AllowGenerateAudio: true,
	},
	VideoModelSeedance25: {
		MinDuration:        4,
		MaxDuration:        30,
		DefaultDuration:    defaultVideoDuration,
		AllowedResolutions: []string{Resolution720P},
		MaxImages:          30,
		MaxVideos:          10,
		MaxAudios:          10,
		AllowGenerateAudio: true,
	},
	// grok：路径与前三者相同，但参数协议不同（见 docs/channel/td-grok-video-design.md）
	VideoModelGrokImagineVideo15: {
		// 时长是离散档位而非区间：上游文档为 6 / 10 / 15，且标为必填
		AllowedDurations: []int{6, 10, 15},
		RequireDuration:  true,
		// RequireDuration 为真时 DefaultDuration 仅作防御性兜底（正常路径不会用到）
		DefaultDuration:    defaultVideoDuration,
		AllowedResolutions: []string{Resolution720P},
		// 本模型独有 1:1；显式声明后不再回退全局集合
		AllowedAspectRatios: []string{AspectRatio16x9, AspectRatio9x16, AspectRatio1x1},
		// 上游文档全文无 images 字段：参考图走 input_reference 与
		// extra.reference_images 两条通道，故 images 配 0（传入即拒绝），
		// 避免该参数被静默透传给不认它的上游。
		MaxImages: 0,
		MaxVideos: 0, // 上游文档未列，不支持
		MaxAudios: 0,
		// 首尾帧 2 张、多参考图 2~7 张，取上限 7。
		// 非 0 才会启用 extra 参考图校验（存量三模型为 0，天然跳过）。
		MaxExtraReferenceImages: 7,
		AllowGenerateAudio:      false,
	},
}

// lookupVideoModelSpec 按上游模型名查能力矩阵。
// 大小写与首尾空白不敏感（上游模型名统一小写，但用户可能写错大小写）。
func lookupVideoModelSpec(model string) (videoModelSpec, bool) {
	spec, ok := videoModelSpecs[normalizeModelName(model)]
	return spec, ok
}

// normalizeModelName 归一化模型名用于查表，保留原始大小写下发上游。
func normalizeModelName(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

// supportsResolution 判断模型是否允许该分辨率档位（大小写不敏感）。
func (s videoModelSpec) supportsResolution(resolution string) bool {
	return slices.Contains(s.AllowedResolutions, strings.ToLower(strings.TrimSpace(resolution)))
}

// supportsAspectRatio 判断比例是否合法。
//
// 优先用模型自己的 AllowedAspectRatios；未声明时回退到通用集合
// allowedAspectRatios —— 存量三模型不声明该字段，行为与改造前完全一致。
func (s videoModelSpec) supportsAspectRatio(aspectRatio string) bool {
	allowed := s.AllowedAspectRatios
	if len(allowed) == 0 {
		allowed = allowedAspectRatios
	}
	return slices.Contains(allowed, strings.TrimSpace(aspectRatio))
}

// aspectRatioHint 用于错误信息，列出该模型允许的比例。
// 与 supportsAspectRatio 使用同一套回退逻辑，避免提示与实际校验不一致。
func (s videoModelSpec) aspectRatioHint() string {
	allowed := s.AllowedAspectRatios
	if len(allowed) == 0 {
		allowed = allowedAspectRatios
	}
	return strings.Join(allowed, ", ")
}

// supportsDuration 判断时长是否合法。
// 枚举模式（AllowedDurations 非空）优先；否则走 Min/Max 范围。
func (s videoModelSpec) supportsDuration(duration int) bool {
	if len(s.AllowedDurations) > 0 {
		return slices.Contains(s.AllowedDurations, duration)
	}
	return duration >= s.MinDuration && duration <= s.MaxDuration
}

// resolutionHint 用于错误信息，列出模型允许的档位。
func (s videoModelSpec) resolutionHint() string {
	return strings.Join(s.AllowedResolutions, ", ")
}

// durationHint 用于错误信息。
// 枚举模式输出 "6, 10, 15"，范围模式输出 "3-15"。
func (s videoModelSpec) durationHint() string {
	if len(s.AllowedDurations) > 0 {
		parts := make([]string, 0, len(s.AllowedDurations))
		for _, d := range s.AllowedDurations {
			parts = append(parts, strconv.Itoa(d))
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprintf("%d-%d", s.MinDuration, s.MaxDuration)
}
