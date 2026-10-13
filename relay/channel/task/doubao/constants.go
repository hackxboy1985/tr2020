package doubao

import "strings"

var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
}

var ChannelName = "doubao-video"

// ============================================================================
// 视频计费价格表：模型 → 分辨率 → 是否带视频输入
// ============================================================================
//
// 价格单位与官方定价一致（元 / 百万 tokens），仅用于计算"带视频/不带视频"的
// 相对折扣比率；最终扣费仍以管理员在「模型倍率/价格」中配置的费率为基准。
//
// 管理员约定：基础费率（ModelRatio / ModelPrice）配置为「480p/720p + 不含视频」
// 的标准价，系统按下列价格表自动折算其它组合。
const (
	// Resolution480P 输出视频分辨率 480p
	Resolution480P = "480p"
	// Resolution720P 输出视频分辨率 720p
	Resolution720P = "720p"
	// Resolution1080P 输出视频分辨率 1080p
	Resolution1080P = "1080p"
	// Resolution4K 输出视频分辨率 4k
	Resolution4K = "4k"

	// ResolutionBaseline 基准分辨率（价格表中作为 1.0 基准的档位）
	ResolutionBaseline = Resolution720P
)

// videoResolutionPrices 单个模型的「分辨率 → (不含视频价, 含视频价)」价格表。
type videoResolutionPrices map[string][2]float64

// videoPriceMap 模型 → 分辨率 → (不含视频价格, 含视频价格)。
//
// 严格按模型 + 分辨率查表，未列出的组合视为未配置（不参与计价）。
var videoPriceMap = map[string]videoResolutionPrices{
	// doubao-seedance-2.5
	//   480p/720p：不含视频 70.00，含视频 42.00
	//   1080p    ：不含视频 77.00，含视频 46.00
	"doubao-seedance-2-5-260628": {
		Resolution480P:  {70.0, 42.0},
		Resolution720P:  {70.0, 42.0},
		Resolution1080P: {77.0, 46.0},
	},

	// doubao-seedance-2.0
	//   480p/720p：不含视频 46.00，含视频 28.00
	//   1080p    ：不含视频 51.00，含视频 31.00
	//   4k       ：不含视频 26.00，含视频 16.00
	"doubao-seedance-2-0-260128": {
		Resolution480P:  {46.0, 28.0},
		Resolution720P:  {46.0, 28.0},
		Resolution1080P: {51.0, 31.0},
		Resolution4K:    {26.0, 16.0},
	},

	// doubao-seedance-2.0-fast（仅按是否含视频输入区分，480p/720p 同价）
	"doubao-seedance-2-0-fast-260128": {
		Resolution480P: {37.0, 22.0},
		Resolution720P: {37.0, 22.0},
	},

	// doubao-seedance-2.0-mini（仅按是否含视频输入区分，480p/720p 同价）
	"doubao-seedance-2-0-mini-260615": {
		Resolution480P: {23.0, 14.0},
		Resolution720P: {23.0, 14.0},
	},
}

// resolutionAliases 归一化分辨率别名 → 标准档位。
// 注意：不收录 "2k" —— 2K 是 2048×1080 的 DCI 规格，与 1080p（1920×1080）
// 不是同一档，混用会导致计价档位错误。
var resolutionAliases = map[string]string{
	"480p":   Resolution480P,
	"480":    Resolution480P,
	"sd":     Resolution480P,
	"720p":   Resolution720P,
	"720":    Resolution720P,
	"hd":     Resolution720P,
	"1080p":  Resolution1080P,
	"1080":   Resolution1080P,
	"fhd":    Resolution1080P,
	"fullhd": Resolution1080P,
	"4k":     Resolution4K,
	"2160p":  Resolution4K,
	"uhd":    Resolution4K,
}

// NormalizeResolution 归一化分辨率标识，返回标准档位（480p/720p/1080p/4k）；
// 无法识别时返回空字符串。
//
// 支持：480p / 720P / 1080p / 4K 等写法（大小写不敏感）。
// 仅接受分辨率档位标识；尺寸（WxH）不是 Doubao 的分辨率入参，不做转换。
func NormalizeResolution(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	// 去除常见修饰：如 "720p_16:9"、"1080p(默认)"
	if idx := strings.IndexAny(s, "(_/ \t"); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if r, ok := resolutionAliases[s]; ok {
		return r
	}
	// 纯数字：480 / 720 / 1080 / 2160 / 3840
	if isDigits(s) {
		if r, ok := resolutionAliases[s+"p"]; ok {
			return r
		}
		switch {
		case len(s) <= 3:
			return Resolution480P
		case strings.HasPrefix(s, "1"):
			return Resolution1080P
		default:
			return Resolution4K
		}
	}
	return ""
}

// isDigits 判断字符串是否全为数字。
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// GetVideoResolutionPrice 返回指定模型 + 分辨率下的（不含视频价格, 含视频价格）。
//
// 严格按「模型 + 分辨率」查表，不做任何回退：
// 模型或该分辨率档位未配置时返回 ok=false，由调用方决定行为。
func GetVideoResolutionPrice(modelName, resolution string) (priceWithoutVideo, priceWithVideo float64, ok bool) {
	prices, ok := videoPriceMap[modelName][NormalizeResolution(resolution)]
	if !ok {
		return 0, 0, false
	}
	return prices[0], prices[1], true
}
