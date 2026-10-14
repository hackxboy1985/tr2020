package oss

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadResult 转存结果。
type UploadResult struct {
	// PublicURL 对象对外访问地址（按当时的 PublicBaseURL 生成，会被持久化）
	PublicURL string
	// ObjectKey 对象键，用于后续清理或人工核对
	ObjectKey string
	// Size 实际转存的字节数
	Size int64
	// ContentType 源内容类型
	ContentType string
}

// UploadFromURL 将 srcURL 指向的文件流式转存到对象存储。
//
// 行为约定：
//   - 幂等：相同 objectKey 重复调用会覆盖同一对象，不会产生垃圾文件。
//   - 流式：全程使用 io.Reader 管道，不将文件读入内存（视频可达数百 MB）。
//   - SSRF 防护：下载源地址前校验，复用系统既有的 fetch_setting 配置。
//
// 调用方应在失败时降级返回上游原始 URL，不得因转存失败而中断主流程。
func UploadFromURL(ctx context.Context, srcURL string, objectKey string) (*UploadResult, error) {
	if !IsConfigured() {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(srcURL) == "" {
		return nil, fmt.Errorf("source url is empty")
	}
	if strings.TrimSpace(objectKey) == "" {
		return nil, fmt.Errorf("object key is empty")
	}

	client, err := Client()
	if err != nil {
		return nil, err
	}

	// 下载源文件（流式，不落盘、不入内存）
	resp, err := openSourceStream(ctx, srcURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	contentType := normalizeContentType(resp.Header.Get("Content-Type"))
	contentLength := resp.ContentLength

	// 体积预检：Content-Length 已知且超限时直接拒绝，避免浪费带宽
	maxSize := system_setting.ObjectStorageMaxFileSize
	if maxSize > 0 && contentLength > 0 && contentLength > maxSize {
		return nil, fmt.Errorf("%w: content-length=%d max=%d", ErrFileTooLarge, contentLength, maxSize)
	}

	// 内容类型预检：防止把 HTML 错误页当作结果上传
	if contentType != "" && !isMediaContentType(contentType) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
	}

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(system_setting.ObjectStorageBucket),
		Key:         aws.String(objectKey),
		Body:        resp.Body,
		ContentType: aws.String(contentTypeOrDefault(contentType, objectKey)),
	}

	// ContentLength 必须是确定值才能设置；未知时交由 SDK 流式处理。
	// 注意：不能为了获取长度而缓冲整个文件。
	if contentLength > 0 {
		putInput.ContentLength = aws.Int64(contentLength)
	}

	if _, err := client.PutObject(ctx, putInput); err != nil {
		return nil, fmt.Errorf("put object failed: %w", err)
	}

	size := contentLength
	if size < 0 {
		size = 0
	}

	return &UploadResult{
		PublicURL:   PublicURL(objectKey),
		ObjectKey:   objectKey,
		Size:        size,
		ContentType: contentType,
	}, nil
}

// openSourceStream 打开源文件的下载流，并执行 SSRF 校验。
//
// 为什么不直接复用 service.DoDownloadRequest：
// pkg/oss 是被 service 依赖的下层包，反向导入 service 会形成循环依赖，
// 且破坏分层。此处按相同规则自行校验后发起请求，保持行为一致。
func openSourceStream(ctx context.Context, srcURL string) (*http.Response, error) {
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(
		srcURL,
		fetchSetting.EnableSSRFProtection,
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	); err != nil {
		return nil, fmt.Errorf("request reject: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request failed: %w", err)
	}

	resp, err := getHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download source failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download source failed: status=%d", resp.StatusCode)
	}

	return resp, nil
}

// httpClient 转存专用客户端。
// 不设置整体 Timeout（大文件传输耗时较长），由 context 控制总时长；
// 但设置响应头超时，避免连接建立后对端不返回数据导致长时间挂起。
var transferHTTPClient = &http.Client{
	Transport: &http.Transport{
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

func getHTTPClient() *http.Client {
	return transferHTTPClient
}

// normalizeContentType 归一化 Content-Type：转小写、去参数、去空白。
func normalizeContentType(ct string) string {
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = ct[:idx]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// isMediaContentType 判断是否为图片或视频类型。
func isMediaContentType(ct string) bool {
	return strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "image/")
}

// contentTypeOrDefault 返回用于对象存储的 Content-Type。
// 源未提供或非媒体类型时，按对象键扩展名兜底，保证浏览器能正确识别。
func contentTypeOrDefault(ct string, objectKey string) string {
	if ct != "" && isMediaContentType(ct) {
		return ct
	}
	if ext := extFromURL(objectKey); ext != "" {
		for mime, known := range contentTypeExtMap {
			if known == ext {
				return mime
			}
		}
	}
	return "application/octet-stream"
}
