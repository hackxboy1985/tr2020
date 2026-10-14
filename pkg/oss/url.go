package oss

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

// PublicURL 由对象键拼接公网可访问地址。
//
// 拼接规则：
//   - PublicBaseURL 非空 → {PublicBaseURL}/{objectKey}
//   - 否则              → https://{Bucket}.{Endpoint}/{objectKey}
//
// 为什么优先 PublicBaseURL：
//  1. MinIO 部署在内网时（如 127.0.0.1:9000）必须通过反向代理域名对外提供访问，
//     内网地址用户无法访问，故必须显式配置 PublicBaseURL。
//  2. 将来切换 CDN 或自定义域名时，历史数据无需回改——地址在写入时即已按
//     当时的 PublicBaseURL 生成（见 UploadFromURL 的返回值语义）。
//
// 注意返回值会被持久化到任务记录中，因此本函数必须保持稳定。
func PublicURL(objectKey string) string {
	key := strings.TrimLeft(objectKey, "/")

	if base := strings.TrimSpace(system_setting.ObjectStoragePublicBaseURL); base != "" {
		return strings.TrimRight(base, "/") + "/" + key
	}

	// 回退：虚拟主机风格 https://{bucket}.{endpoint}/{key}
	endpoint := strings.TrimSpace(system_setting.ObjectStorageEndpoint)
	// 去掉 scheme，虚拟主机风格需要纯 host
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimRight(endpoint, "/")

	bucket := strings.TrimSpace(system_setting.ObjectStorageBucket)
	return "https://" + bucket + "." + endpoint + "/" + key
}
