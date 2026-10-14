package oss

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

var (
	// ErrNotConfigured 对象存储未配置或配置不完整
	ErrNotConfigured = errors.New("object storage is not configured")
	// ErrFileTooLarge 源文件超过允许的最大体积
	ErrFileTooLarge = errors.New("source file exceeds max allowed size")
	// ErrUnsupportedContentType 源内容类型不是预期的图片/视频，可能是错误页
	ErrUnsupportedContentType = errors.New("unsupported content type")
)

// 默认扩展名：无法从 URL 或 Content-Type 推断时使用
const (
	defaultVideoExt = ".mp4"
	defaultImageExt = ".png"
)

// 常见内容类型到扩展名的映射。
// 仅覆盖视频/图片场景，避免引入庞大的 mime 表。
var contentTypeExtMap = map[string]string{
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/x-msvideo": ".avi",
	"video/webm":      ".webm",
	"video/x-matroska": ".mkv",
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/jpg":       ".jpg",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"image/bmp":       ".bmp",
}

// extFromURL 从源 URL 的路径部分提取扩展名。
//
// 使用 url.Parse 后取 Path，可自动忽略 query 与 fragment
//（例如 .../a.mp4?auth=xxx 应得到 .mp4）。
func extFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(path.Ext(u.Path))
	return normalizeExt(ext)
}

// extFromContentType 从 Content-Type 推断扩展名。
// 会剥离形如 "video/mp4; charset=utf-8" 的参数部分。
func extFromContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return contentTypeExtMap[contentType]
}

// normalizeExt 校验并归一化扩展名，非法值返回空串。
func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" || ext == "." {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	// 扩展名只允许字母数字，最长 8 位（如 .webm / .quicktime 简化处理）
	body := strings.TrimPrefix(ext, ".")
	if len(body) == 0 || len(body) > 8 {
		return ""
	}
	for _, r := range body {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return ""
		}
	}
	return ext
}

// BuildObjectKey 生成对象键。
//
// 规则：{PathPrefix}{yyyy}/{MM}/{dd}/{taskID}{ext}
// 例：  td/2026/10/14/task_abc123.mp4
//
// 设计要点：
//   - 以 taskID 为文件名而非随机串，保证**幂等**：重复上传覆盖同一对象，
//     不会因重试产生垃圾文件。
//   - 按日期分目录，便于配置生命周期规则与人工排查。
//   - 扩展名推断顺序：源 URL 路径 → Content-Type → 兜底（视频 .mp4 / 图片 .png）。
func BuildObjectKey(taskID string, srcURL string, contentType string, isVideo bool, now time.Time) string {
	ext := extFromURL(srcURL)
	if ext == "" {
		ext = extFromContentType(contentType)
	}
	if ext == "" {
		if isVideo {
			ext = defaultVideoExt
		} else {
			ext = defaultImageExt
		}
	}

	prefix := strings.Trim(system_setting.ObjectStoragePathPrefix, "/")
	var sb strings.Builder
	if prefix != "" {
		sb.WriteString(prefix)
		sb.WriteString("/")
	}
	fmt.Fprintf(&sb, "%04d/%02d/%02d/%s%s",
		now.Year(), int(now.Month()), now.Day(), sanitizeKeySegment(taskID), ext)
	return sb.String()
}

// sanitizeKeySegment 清理对象键中的文件名片段，防止路径穿越与非法字符。
func sanitizeKeySegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	// 只保留字母、数字、下划线、短横线、点
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-', r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	out := sb.String()
	// 防止 ".." 造成路径穿越
	out = strings.ReplaceAll(out, "..", "_")
	if out == "" {
		return "unknown"
	}
	return out
}
