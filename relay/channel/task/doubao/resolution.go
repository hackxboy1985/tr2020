package doubao

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// 分辨率解析（EstimateBilling 的必要输入之一）：
//
// Doubao/Seedance 的分辨率入参统一是 resolution（档位标识），不存在 size 字段，
// 因此不做 WxH 尺寸解析。两种格式都要解析：
//
//	格式一（标准格式）：metadata.resolution
//	  {"model":"...","prompt":"...","images":[...],
//	   "metadata":{"resolution":"720p","ratio":"16:9"}}
//	格式二（Ark 原生格式）：顶层 resolution
//	  {"model":"...","content":[...],"resolution":"720p","ratio":"16:9"}
//
// 唯一取值入口：ResolveVideoResolution(req)。
// 两种受支持格式的 resolution 入站解析后都落在 req.Metadata["resolution"]：
//   - 格式一（OpenAI 风格）：客户端写在 metadata.resolution
//   - 格式二（Ark 原生）：客户端写在顶层 resolution，被 ValidateBasicTaskRequest 搬进 metadata
//
// 校验与计费共用同一结果（校验阶段解析后缓存，计费复用）。
//
// 相关文档：docs/咪咕/现有下游用户seedanceOpenAI格式api.md

// ResolutionKey 承载输出视频分辨率的字段名。
const ResolutionKey = "resolution"

// resolutionNestedKeys metadata 内可能再嵌一层分辨率的容器字段名。
var resolutionNestedKeys = []string{"video_generation", "videoGeneration", "parameters", "params", "input"}

// ResolveVideoResolution 从已解析的任务请求中解析分辨率档位。
// 无法识别时返回空字符串。
func ResolveVideoResolution(req *relaycommon.TaskSubmitReq) string {
	if req == nil {
		return ""
	}

	// 格式一：metadata.resolution
	// （Ark 原生顶层的 resolution 经 ValidateBasicTaskRequest 后也落在 metadata）
	if r := resolveResolutionFromMetadata(req.Metadata); r != "" {
		return r
	}

	// 格式二：Ark 原生 content[] 内的 resolution
	if r := resolveResolutionFromContent(req.Content); r != "" {
		return r
	}

	return ""
}

// resolveResolutionFromMetadata 在 metadata 中查找分辨率，含一层嵌套兜底。
func resolveResolutionFromMetadata(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	if r := NormalizeResolution(stringifyValue(metadata[ResolutionKey])); r != "" {
		return r
	}
	// 嵌套形态：metadata.video_generation.resolution 等
	for _, key := range resolutionNestedKeys {
		if nested, ok := metadata[key].(map[string]interface{}); ok {
			if r := NormalizeResolution(stringifyValue(nested[ResolutionKey])); r != "" {
				return r
			}
		}
	}
	return ""
}

// resolveResolutionFromContent 在 Ark 原生 content[] 数组中查找分辨率。
func resolveResolutionFromContent(content []map[string]interface{}) string {
	for _, item := range content {
		if item == nil {
			continue
		}
		if r := NormalizeResolution(stringifyValue(item[ResolutionKey])); r != "" {
			return r
		}
	}
	return ""
}

// stringifyValue 把 JSON 反序列化后的任意值安全转成字符串。
// json.Number / float64（如 "resolution": 1080）也能正确转换。
func stringifyValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case float64:
		return formatNumber(val)
	case float32:
		return formatNumber(float64(val))
	case int:
		return itoa(val)
	case int64:
		return itoa64(val)
	case interface{ String() string }:
		return val.String()
	default:
		return ""
	}
}

// formatNumber 将 1080 / 1080.0 统一格式化为 "1080"。
func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return itoa64(int64(f))
	}
	return ""
}

func itoa(v int) string {
	return itoa64(int64(v))
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
