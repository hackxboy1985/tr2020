package taskcommon

import (
	"net/url"
	"path"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ZyTaskResult 是 Zy 查询结果中与媒体有关的字段。
// Zy 图片也可能带 object=video 和 video_url，不能仅凭字段名判断媒体类型。
type ZyTaskResult struct {
	Model    string `json:"model"`
	Object   string `json:"object"`
	ImageURL string `json:"image_url,omitempty"`
	URL      string `json:"url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
	Metadata struct {
		Image    string `json:"image,omitempty"`
		ImageURL string `json:"image_url,omitempty"`
	} `json:"metadata,omitempty"`
}

// ResultURLs 图片按 image_url → url 读取；视频按 video_url → url 读取。
// 旧版 metadata 图片字段只作为图片兼容兜底，所有候选地址去重。
func (r *ZyTaskResult) ResultURLs() []string {
	candidates := []string{r.ImageURL, r.URL, r.Metadata.ImageURL, r.Metadata.Image}
	if r.IsVideo() {
		candidates = []string{r.VideoURL, r.URL}
	}
	var results []string
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		results = append(results, candidate)
	}
	return results
}

// ResultURL 单结果任务只取最高优先级的一个地址，不为同一结果的别名重复上传。
func (r *ZyTaskResult) ResultURL() string {
	if urls := r.ResultURLs(); len(urls) > 0 {
		return urls[0]
	}
	return ""
}

func (r *ZyTaskResult) IsVideo() bool {
	if strings.TrimSpace(r.ImageURL) != "" {
		return false
	}
	// url 是 Zy 的通用结果字段，先按实际资源后缀判定，不把 video_url 别名当证据。
	candidate := strings.TrimSpace(r.URL)
	if candidate == "" {
		candidate = strings.TrimSpace(r.VideoURL)
	}
	if u, err := url.Parse(candidate); err == nil {
		switch strings.ToLower(path.Ext(u.Path)) {
		case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".avif", ".bmp", ".tif", ".tiff", ".svg":
			return false
		case ".mp4", ".mov", ".avi", ".webm", ".mkv", ".flv", ".wmv", ".m4v":
			return true
		}
	}
	if r.Metadata.ImageURL != "" || r.Metadata.Image != "" {
		return false
	}
	// 无扩展名的图片模型即使附带 video_url 也必须走图片开关。
	model := strings.ToLower(r.Model)
	if strings.Contains(model, "image") || strings.Contains(model, "dall-e") || strings.Contains(model, "flux") {
		return false
	}
	// object=video 是 Zy 的通用包装，不单独作为视频证据。
	return strings.TrimSpace(r.VideoURL) != ""
}

func IsZyVideoTaskResponse(body []byte) bool {
	var result ZyTaskResult
	if err := common.Unmarshal(body, &result); err != nil {
		return false
	}
	return result.IsVideo()
}
