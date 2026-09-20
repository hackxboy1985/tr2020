package zy

import (
	"fmt"
	"strconv"
	"strings"
)

// 分辨率档位（上游要求大写 K）
const (
	Resolution1K = "1K"
	Resolution2K = "2K"
	Resolution4K = "4K"
)

// AutoAspectRatio 表示由上游根据提示词自动推断比例
const AutoAspectRatio = "auto"

// aspectRatioTiers 比例 → 各档位像素尺寸，直接对应上游文档的「比例与 size 对照」表。
// 需要调整像素对照时只改这里即可。
var aspectRatioTiers = map[string]map[string]string{
	"1:1":  {Resolution1K: "1024x1024", Resolution2K: "2048x2048", Resolution4K: "2880x2880"},
	"16:9": {Resolution1K: "1280x720", Resolution2K: "2560x1440", Resolution4K: "3840x2160"},
	"9:16": {Resolution1K: "720x1280", Resolution2K: "1440x2560", Resolution4K: "2160x3840"},
	"3:2":  {Resolution1K: "1248x832", Resolution2K: "2496x1664", Resolution4K: "3504x2336"},
	"2:3":  {Resolution1K: "832x1248", Resolution2K: "1664x2496", Resolution4K: "2336x3504"},
	"4:3":  {Resolution1K: "1152x864", Resolution2K: "2304x1728", Resolution4K: "3264x2448"},
	"3:4":  {Resolution1K: "864x1152", Resolution2K: "1728x2304", Resolution4K: "2448x3264"},
	"5:4":  {Resolution1K: "1120x896", Resolution2K: "2240x1792", Resolution4K: "3200x2560"},
	"4:5":  {Resolution1K: "896x1120", Resolution2K: "1792x2240", Resolution4K: "2560x3200"},
	"21:9": {Resolution1K: "1456x624", Resolution2K: "3024x1296", Resolution4K: "3696x1584"},
}

// sizeToTier 由 aspectRatioTiers 反向生成：像素尺寸 → {比例, 档位}。
var sizeToTier = func() map[string]SizeConfig {
	m := make(map[string]SizeConfig)
	for ratio, tiers := range aspectRatioTiers {
		for resolution, size := range tiers {
			m[size] = SizeConfig{AspectRatio: ratio, Resolution: resolution}
		}
	}
	return m
}()

// SizeConfig 归一化后的画面参数
type SizeConfig struct {
	AspectRatio string
	Resolution  string
}

// supportedAspectRatios 上游支持的固定比例（含 auto）
var supportedAspectRatios = map[string]bool{
	AutoAspectRatio: true,
	"1:1":           true,
	"16:9":          true,
	"9:16":          true,
	"4:3":           true,
	"3:4":           true,
	"3:2":           true,
	"2:3":           true,
	"5:4":           true,
	"4:5":           true,
	"21:9":          true,
}

// IsSupportedAspectRatio 校验比例是否为上游支持的值（大小写不敏感，auto 亦合法）
func IsSupportedAspectRatio(ratio string) bool {
	return supportedAspectRatios[strings.ToLower(strings.TrimSpace(ratio))]
}

// IsPixelSize 判断 size 是否为 "宽x高" 像素格式
func IsPixelSize(size string) bool {
	s := strings.ToLower(strings.TrimSpace(size))
	if !strings.Contains(s, "x") {
		return false
	}
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return false
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	return errW == nil && errH == nil && w > 0 && h > 0
}

// IsRatioSize 判断 size 是否为 "a:b" 比例格式（不是像素）
func IsRatioSize(size string) bool {
	s := strings.TrimSpace(size)
	return strings.Contains(s, ":") && !strings.Contains(s, "x")
}

// NormalizeResolution 归一化分辨率档位为上游要求的 1K/2K/4K。
// 接受 1k/1K/2k/4k 等写法，空值返回 1K（上游默认档）。
func NormalizeResolution(resolution string) string {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "1K":
		return Resolution1K
	case "2K":
		return Resolution2K
	case "4K":
		return Resolution4K
	default:
		return Resolution1K
	}
}

// IsValidResolution 校验分辨率档位是否合法（空值视为合法，取默认 1K）
func IsValidResolution(resolution string) bool {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "", "1K", "2K", "4K":
		return true
	default:
		return false
	}
}

// normalizeAspectRatio 归一化比例字符串：auto 统一为小写 auto，其余原样返回
func normalizeAspectRatio(ratio string) string {
	ratio = strings.TrimSpace(ratio)
	if strings.EqualFold(ratio, AutoAspectRatio) {
		return AutoAspectRatio
	}
	return ratio
}

// resolveSizeConfig 按「aspect_ratio 优先、size 兜底」的规则归一化画面参数。
//
// 优先级（与产品约定一致）：
//  1. aspect_ratio 非空 → 校验比例，档位取 image_size/resolution（默认 1K）
//  2. size 为比例字符串 → 作为 aspect_ratio，档位取 image_size/resolution
//  3. size 为像素尺寸 → 命中对照表则反查比例+档位；未命中则原样透传 size
//  4. 均为空 → 不指定，交由上游按提示词自动推断
//
// 返回的 SizeConfig.Resolution 仅在「需要且可以下发档位」时有意义；
// 未命中的像素尺寸通过 passthroughSize 原样回传。
func resolveSizeConfig(aspectRatio, size, imageSize, resolution string) (cfg SizeConfig, passthroughSize string, err error) {
	requested := imageSize
	if strings.TrimSpace(requested) == "" {
		requested = resolution
	}
	if !IsValidResolution(requested) {
		return cfg, "", fmt.Errorf("invalid image_size/resolution: %s, must be one of: 1K, 2K, 4K", requested)
	}
	// image_size 与 resolution 等价，同时传不同值属于请求错误
	if strings.TrimSpace(imageSize) != "" && strings.TrimSpace(resolution) != "" &&
		NormalizeResolution(imageSize) != NormalizeResolution(resolution) {
		return cfg, "", fmt.Errorf("image_size and resolution must not be provided with different values")
	}
	cfg.Resolution = NormalizeResolution(requested)

	// 1. aspect_ratio 优先
	if ratio := strings.TrimSpace(aspectRatio); ratio != "" {
		if !IsSupportedAspectRatio(ratio) {
			return cfg, "", fmt.Errorf("invalid aspect_ratio: %s", ratio)
		}
		cfg.AspectRatio = normalizeAspectRatio(ratio)
		return cfg, "", nil
	}

	// 2/3. 回退到 size
	size = strings.TrimSpace(size)
	if size == "" {
		// 4. 不指定比例，上游自动推断；此时档位无比例可依附，一并忽略
		cfg.AspectRatio = ""
		return cfg, "", nil
	}
	if IsRatioSize(size) {
		if !IsSupportedAspectRatio(size) {
			return cfg, "", fmt.Errorf("invalid size: %s", size)
		}
		cfg.AspectRatio = normalizeAspectRatio(size)
		return cfg, "", nil
	}
	if IsPixelSize(size) {
		if hit, ok := sizeToTier[size]; ok {
			cfg.AspectRatio = hit.AspectRatio
			cfg.Resolution = hit.Resolution
			return cfg, "", nil
		}
		// 未命中对照表：像素本身已决定分辨率，原样透传，不再下发档位
		cfg.AspectRatio = ""
		return cfg, size, nil
	}
	return cfg, "", fmt.Errorf("invalid size: %s", size)
}

// modelTierInName 判断该模型是否「档位写在 model 名里」（gpt-image-2 / -2K / -4K）。
func modelTierInName(model string) bool {
	switch strings.TrimSpace(model) {
	case "gpt-image-2", "gpt-image-2-2K", "gpt-image-2-4K":
		return true
	default:
		return false
	}
}

// modelSupportsParamTier 判断该模型是否支持用参数选 1K/2K/4K（flare / sunburst）。
func modelSupportsParamTier(model string) bool {
	switch strings.TrimSpace(model) {
	case "gpt-image-2.5-flare", "gpt-image-2.5-sunburst":
		return true
	default:
		return false
	}
}

// modelOnly1K 判断该模型是否仅支持 1K（gpt-image-2.5）。
func modelOnly1K(model string) bool {
	return strings.TrimSpace(model) == "gpt-image-2.5"
}

// resolvedModelForTier 选择用于判断模型档位能力的模型名。
// 优先取已解析的上游模型名（模型映射之后），否则回退到下游请求的模型名。
func resolvedModelForTier(requestModel, upstreamModel string) string {
	if strings.TrimSpace(upstreamModel) != "" {
		return upstreamModel
	}
	return requestModel
}

// requestedTier 返回用户显式请求的分辨率档位（image_size 优先，其次 resolution）。
func requestedTier(imageSize, resolution string) string {
	if strings.TrimSpace(imageSize) != "" {
		return imageSize
	}
	return resolution
}

// validateTierSupport 校验模型是否支持用户请求的分辨率档位。
// 目前仅限制 gpt-image-2.5（只有 1K）。
func validateTierSupport(model, imageSize, resolution string) error {
	tier := strings.TrimSpace(requestedTier(imageSize, resolution))
	if tier == "" {
		return nil
	}
	if modelOnly1K(model) && NormalizeResolution(tier) != Resolution1K {
		return fmt.Errorf("model %s only supports 1K", model)
	}
	return nil
}

// buildSubmitRequest 依据归一化结果与模型能力组装上游请求体。
//
// 档位下发规则（对应上游文档「分辨率控制」）：
//   - flare / sunburst：用 aspect_ratio + image_size 选档；有像素尺寸时优先原样传 size
//   - gpt-image-2.5：仅 1K，不下发档位
//   - gpt-image-2 / -2K / -4K：档位在模型名中，不下发档位
func buildSubmitRequest(model, prompt string, cfg SizeConfig, passthroughSize string, images []string) (*submitRequest, error) {
	req := &submitRequest{
		Model:  model,
		Prompt: prompt,
		Images: images,
	}

	switch {
	case modelSupportsParamTier(model):
		if passthroughSize != "" {
			// 用户给的是对照表外的精确像素，直接按像素出图
			req.Size = passthroughSize
		} else if cfg.AspectRatio != "" {
			req.AspectRatio = cfg.AspectRatio
			req.ImageSize = cfg.Resolution
		}
	case modelOnly1K(model):
		// 仅 1K：不传档位（传 1K 亦可，但保持最小请求）
		if cfg.AspectRatio != "" {
			req.AspectRatio = cfg.AspectRatio
		}
	case modelTierInName(model):
		// 档位由模型名决定
		if cfg.AspectRatio != "" {
			req.AspectRatio = cfg.AspectRatio
		}
	default:
		// 未知/自定义模型：按通用规则下发，尽量保留用户意图
		if passthroughSize != "" {
			req.Size = passthroughSize
		} else if cfg.AspectRatio != "" {
			req.AspectRatio = cfg.AspectRatio
			req.ImageSize = cfg.Resolution
		}
	}
	return req, nil
}
