package system_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// 对象存储（S3 协议）配置。
//
// 设计说明：
//   - 采用标准 S3 协议，同时兼容自建 MinIO 与阿里云 OSS 等任意 S3 兼容服务；
//     切换后端只需改配置，无需改代码。
//   - 当前部署形态为宝塔面板下的自建 MinIO，故 ForcePathStyle 默认为 true。
//   - 未配置完整时（IsObjectStorageConfigured 为 false），所有转存逻辑短路，
//     系统行为与未引入该功能时完全一致。
//
// 环境变量前缀统一使用 S3_（而非 OSS_）：语义准确，不与阿里云强绑定。
var (
	// ObjectStorageEndpoint 服务端点。
	//   MinIO:  http://127.0.0.1:9000
	//   阿里云: oss-cn-hangzhou.aliyuncs.com
	ObjectStorageEndpoint = common.GetEnvOrDefaultString("S3_ENDPOINT", "")

	// ObjectStorageBucket 存储桶名称，需具备公网读权限（本方案采用直出 URL）。
	ObjectStorageBucket = common.GetEnvOrDefaultString("S3_BUCKET", "")

	// ObjectStorageAccessKeyId 访问密钥 ID。
	ObjectStorageAccessKeyId = common.GetEnvOrDefaultString("S3_ACCESS_KEY_ID", "")

	// ObjectStorageAccessKeySecret 访问密钥 Secret。
	// key 名含 "Secret"，命中 common/constants.go 中的既有约定——
	// 含 Secret/Token 的 option key 不会通过 GetOptions 返回前端，天然避免泄露。
	ObjectStorageAccessKeySecret = common.GetEnvOrDefaultString("S3_ACCESS_KEY_SECRET", "")

	// ObjectStorageRegion 区域。MinIO 会忽略该值，但 S3 SDK 要求非空。
	ObjectStorageRegion = common.GetEnvOrDefaultString("S3_REGION", "us-east-1")

	// ObjectStorageForcePathStyle 是否强制 path-style 寻址。
	//
	// MinIO 必须为 true：S3 SDK 默认使用 virtual-hosted 风格
	// （{bucket}.{endpoint}），该风格依赖 DNS 泛解析；MinIO 部署在裸 IP 或
	// 内网时无法解析，必须改用 path-style（{endpoint}/{bucket}）。
	// 阿里云 OSS 等公有云服务可设为 false。
	ObjectStorageForcePathStyle = common.GetEnvOrDefaultBool("S3_FORCE_PATH_STYLE", true)

	// ObjectStoragePublicBaseURL 对象对外访问的基础地址。
	//
	// MinIO 部署必须设置：内网地址（如 127.0.0.1:9000）用户无法访问，需在
	// 宝塔配置反向代理域名（如 https://oss.example.com）并填入此处。
	// 必须启用 HTTPS，否则 HTTPS 页面加载 HTTP 资源会被浏览器混合内容拦截。
	//
	// 留空时回退为 https://{bucket}.{endpoint}/{key} 的虚拟主机风格。
	ObjectStoragePublicBaseURL = common.GetEnvOrDefaultString("S3_PUBLIC_BASE_URL", "")

	// ObjectStoragePathPrefix 对象键前缀，便于按目录配置生命周期规则。
	ObjectStoragePathPrefix = common.GetEnvOrDefaultString("S3_PATH_PREFIX", "td/")

	// ObjectStorageUploadTimeout 单次转存超时时间（秒）。
	// 视频文件较大，默认给 300 秒；不得使用无限期 context。
	ObjectStorageUploadTimeout = common.GetEnvOrDefault("S3_UPLOAD_TIMEOUT", 300)

	// ObjectStorageMaxFileSize 允许转存的最大文件体积（字节），默认 2GB。
	// 超过则直接拒绝，防止异常大文件打爆带宽与内存。
	ObjectStorageMaxFileSize = int64(common.GetEnvOrDefault("S3_MAX_FILE_SIZE", 2*1024*1024*1024))
)

// IsObjectStorageConfigured 判断对象存储配置是否完整。
//
// 四要素（Endpoint / Bucket / AccessKeyId / AccessKeySecret）全部非空才视为
// 已配置。任一缺失即返回 false，所有转存逻辑据此短路。
func IsObjectStorageConfigured() bool {
	return strings.TrimSpace(ObjectStorageEndpoint) != "" &&
		strings.TrimSpace(ObjectStorageBucket) != "" &&
		strings.TrimSpace(ObjectStorageAccessKeyId) != "" &&
		strings.TrimSpace(ObjectStorageAccessKeySecret) != ""
}
