package oss

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

// PublicURL 由对象键拼接公网可访问地址。
//
// 拼接规则：
//   - PublicBaseURL 非空 → {PublicBaseURL}/{Bucket}/{objectKey}（基础地址已带桶名时不重复添加）
//   - 否则              → https://{Bucket}.{Endpoint}/{objectKey}
//
// 为什么优先 PublicBaseURL：
//  1. MinIO 部署在内网时（如 127.0.0.1:9000）必须通过反向代理域名对外提供访问，
//     内网地址用户无法访问，故必须显式配置 PublicBaseURL。
//  2. 基础地址与桶名分别配置，公网直连 MinIO 时必须包含桶路径。
//     历史记录保存的是完整 URL，修改配置不会自动改写已保存的地址。
//
// 注意返回值会被持久化到任务记录中，因此本函数必须保持稳定。
func PublicURL(objectKey string) string {
	key := strings.TrimLeft(objectKey, "/")

	if base := strings.TrimSpace(system_setting.ObjectStoragePublicBaseURL); base != "" {
		base = strings.TrimRight(base, "/")
		bucket := strings.TrimSpace(system_setting.ObjectStorageBucket)
		if u, err := url.Parse(base); err == nil {
			segments := strings.Split(strings.Trim(u.Path, "/"), "/")
			if bucket != "" && segments[len(segments)-1] != bucket {
				base += "/" + url.PathEscape(bucket)
			}
		}
		return base + "/" + key
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
