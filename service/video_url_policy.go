package service

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// officialVideoURLHostSuffixes 火山官方成片存储域名（TOS 直链）。
//
// 示例（官链）：
//
//	https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-5/.../cgt-xxx.mp4?X-Tos-Signature=...
//
// 判定只看 host（带 X-Tos-* 签名参数仍算官链，签名只是防盗链）。
var officialVideoURLHostSuffixes = []string{".volces.com"}

// IsOfficialVideoURL 判断成片地址是否是火山官方域名（host 以 .volces.com 结尾）。
// 非绝对 http(s) 地址一律返回 false。
func IsOfficialVideoURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}
	for _, suffix := range officialVideoURLHostSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// ShouldProxyVideoURL 判断某个成片地址是否需要由本服务代理后再交给下游。
//
//   - sdos2 上游：地址天然是受 Bearer 保护的接口路径 → 总是代理；
//   - 官链（.volces.com）：下游可直连 → 不代理（省带宽）；
//   - 非官链：仅当渠道开启 allow_non_official_video_url 时代理。
func ShouldProxyVideoURL(ch *model.Channel, raw string) bool {
	if ch == nil {
		return false
	}
	if IsSdos2Upstream(ch) {
		return true
	}
	if IsOfficialVideoURL(raw) {
		return false
	}
	return AllowNonOfficialVideoURL(ch)
}
