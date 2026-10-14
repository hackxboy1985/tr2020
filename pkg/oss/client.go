// Package oss 提供对象存储（S3 协议）转存能力。
//
// 设计目标：
//   - 采用标准 S3 协议，同时兼容自建 MinIO 与阿里云 OSS 等任意 S3 兼容服务，
//     切换后端只需改配置，无需改代码。
//   - 未配置时（IsConfigured 返回 false）所有操作短路，不影响主流程。
//   - 强制流式上传，避免大视频文件打爆内存。
package oss

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	clientMu     sync.RWMutex
	cachedClient *s3.Client
	// cachedConfigKey 记录构造当前客户端时的配置指纹。
	// 配置发生变化（如管理员修改了 endpoint）时据此重建，避免使用陈旧连接。
	cachedConfigKey string
)

// IsConfigured 判断对象存储配置是否完整。
func IsConfigured() bool {
	return system_setting.IsObjectStorageConfigured()
}

// configKey 生成配置指纹，用于检测配置变更。
func configKey() string {
	return fmt.Sprintf("%s|%s|%s|%t|%s",
		system_setting.ObjectStorageEndpoint,
		system_setting.ObjectStorageRegion,
		system_setting.ObjectStorageBucket,
		system_setting.ObjectStorageForcePathStyle,
		system_setting.ObjectStorageAccessKeyId,
	)
}

// Client 返回 S3 客户端单例。
//
// 采用懒加载 + 配置指纹检测：配置变更时自动重建客户端，无需重启服务。
// 未配置完整时返回错误，调用方应据此短路。
func Client() (*s3.Client, error) {
	if !IsConfigured() {
		return nil, ErrNotConfigured
	}

	key := configKey()

	clientMu.RLock()
	if cachedClient != nil && cachedConfigKey == key {
		c := cachedClient
		clientMu.RUnlock()
		return c, nil
	}
	clientMu.RUnlock()

	clientMu.Lock()
	defer clientMu.Unlock()

	// 双重检查：可能已被其他 goroutine 重建
	if cachedClient != nil && cachedConfigKey == key {
		return cachedClient, nil
	}

	cfg := aws.Config{
		Region: system_setting.ObjectStorageRegion,
		Credentials: credentials.NewStaticCredentialsProvider(
			system_setting.ObjectStorageAccessKeyId,
			system_setting.ObjectStorageAccessKeySecret,
			"",
		),
	}

	c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(system_setting.ObjectStorageEndpoint)
		// MinIO 必须为 true，原因见 system_setting.ObjectStorageForcePathStyle 注释
		o.UsePathStyle = system_setting.ObjectStorageForcePathStyle
	})

	cachedClient = c
	cachedConfigKey = key
	return c, nil
}

// UploadTimeout 返回单次转存允许的最长耗时。
func UploadTimeout() time.Duration {
	timeout := system_setting.ObjectStorageUploadTimeout
	if timeout <= 0 {
		timeout = 300
	}
	return time.Duration(timeout) * time.Second
}

// WithUploadTimeout 基于父 context 派生带超时的 context。
// 始终设置超时，避免因网络异常导致 goroutine 永久挂起。
func WithUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, UploadTimeout())
}
