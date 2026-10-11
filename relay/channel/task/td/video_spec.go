package td

import (
	"fmt"
	"slices"
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
	// 时长（秒）
	MinDuration     int
	MaxDuration     int
	DefaultDuration int

	// 允许的分辨率档位（小写字面量，与上游参数值一致）
	AllowedResolutions []string

	// 素材数量上限；0 表示该模型不支持此类素材，传入即拒绝
	MaxImages int
	MaxVideos int
	MaxAudios int

	// 音效同步开关是否可用
	AllowGenerateAudio bool
}

// 各模型默认时长（两份上游文档均为 4 秒）
const defaultVideoDuration = 4

// 画面比例：三模型一致
var allowedAspectRatios = []string{"16:9", "9:16"}

// videoModelSpecs 已知视频模型能力矩阵。
//
// 依上游文档（MiniMax H3 / Seedance 2.0 / 2.5）逐项登记：
//   - minimax-h3   : 3-15s, 720p/1080p/2k, 9 图, 不支持参考视频/音频, 无 generate_audio
//   - seedance-2.0 : 3-15s, 仅 720p,      9 图, 3 视频, 3 音频, 支持 generate_audio
//   - seedance-2.5 : 4-30s, 仅 720p,     30 图, 10 视频, 10 音频, 支持 generate_audio
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

// supportsAspectRatio 判断比例是否合法（三模型共用同一取值集合）。
func supportsAspectRatio(aspectRatio string) bool {
	return slices.Contains(allowedAspectRatios, strings.TrimSpace(aspectRatio))
}

// resolutionHint 用于错误信息，列出模型允许的档位。
func (s videoModelSpec) resolutionHint() string {
	return strings.Join(s.AllowedResolutions, ", ")
}

// durationHint 用于错误信息，输出模型允许的时长范围。
func (s videoModelSpec) durationHint() string {
	return fmt.Sprintf("%d-%d", s.MinDuration, s.MaxDuration)
}
