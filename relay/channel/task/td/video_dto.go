package td

import (
	"bytes"

	"github.com/QuantumNous/new-api/common"
)

// ============================
// 视频请求结构
// ============================

// VideoGenerateRequest 视频生成上游请求体。
//
// 为什么不复用 relaycommon.TaskSubmitReq：
//  1. 公共 TaskSubmitReq 没有 videos / audios / generate_audio 字段，
//     直接复用会导致这些参数在反序列化时被静默丢弃；
//  2. 公共结构体被十余个任务渠道共用（hailuo/kling/sora/doubao/vertex/jimeng 等），
//     在其中新增视频素材字段会让不支持参考视频的渠道也接受这些参数，
//     造成跨渠道的静默误接收；
//  3. 公共 TaskSubmitReq.Duration 是非指针 int，无法区分「未传」与「显式传 0」。
//
// 因此视频分支使用独立结构体，遵守 AGENTS.md 规则 6：
// 可选标量字段使用指针 + omitempty，保证「未传 => 省略，显式传零值 => 下发」。
type VideoGenerateRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Duration    *int     `json:"duration,omitempty"`     // 缺省由 videoModelSpecs.DefaultDuration 填充
	AspectRatio string   `json:"aspect_ratio,omitempty"` // 16:9 / 9:16 / 1:1（grok）
	Resolution  string   `json:"resolution,omitempty"`   // 720p / 1080p / 2k
	Images      []string `json:"images,omitempty"`       // 参考图
	Videos      []string `json:"videos,omitempty"`       // 参考视频
	Audios      []string `json:"audios,omitempty"`       // 参考音频

	// InputReference 单张首帧图（grok 专有）。
	// 上游允许两种写法：纯 URL 字符串，或 {"url":"..."} 对象。
	// 指针类型：区分「未传」与「传了空值」。
	InputReference *InputReference `json:"input_reference,omitempty"`

	// Extra grok 要求必填的补充参数容器，至少含 aspect_ratio、resolution。
	//
	// 为什么不用具名 struct：上游对 extra 的字段集仍在扩展（文档已出现
	// reference_images），具名结构会把未登记字段再次静默丢弃，
	// 等于把「静默丢字段」的问题从一层挪到另一层。
	//
	// 首尾帧 / 多参考图通过 extra.reference_images[].role 自动透传，
	// 数量约束由 videoModelSpec.MaxExtraReferenceImages 描述。
	Extra map[string]any `json:"extra,omitempty"`

	// GenerateAudio 视频音效同步开关。
	// 指针类型：显式传 false 与未传语义不同，不能因 omitempty 丢掉 false。
	GenerateAudio *bool `json:"generate_audio,omitempty"`
}

// InputReference 单张首帧图，兼容上游的两种写法。
//
// 上游文档（grok）明确 input_reference 可为字符串 URL 或 {"url":"..."} 对象。
// 用 WasObject 记录入参形态，回传上游时还原为原写法，
// 避免改变上游实际收到的报文形态（有些上游对两种写法处理不一致）。
type InputReference struct {
	URL       string
	WasObject bool // 入参为对象写法时为 true
}

// UnmarshalJSON 兼容字符串与对象两种写法。
func (r *InputReference) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	// 字符串写法："https://example.com/first.jpg"
	if trimmed[0] == '"' {
		var s string
		if err := common.Unmarshal(data, &s); err != nil {
			return err
		}
		r.URL, r.WasObject = s, false
		return nil
	}
	// 对象写法：{"url":"https://example.com/first.jpg"}
	var obj struct {
		URL string `json:"url"`
	}
	if err := common.Unmarshal(data, &obj); err != nil {
		return err
	}
	r.URL, r.WasObject = obj.URL, true
	return nil
}

// MarshalJSON 按入参形态还原，保持与上游约定一致。
func (r InputReference) MarshalJSON() ([]byte, error) {
	if r.WasObject {
		return common.Marshal(struct {
			URL string `json:"url"`
		}{URL: r.URL})
	}
	return common.Marshal(r.URL)
}

// ============================
// 视频响应结构
// ============================

// VideoTaskResultData 视频任务查询响应的 data 部分。
//
// 上游查询接口 GET /v1/tasks/{task_id} 与图片任务共用，
// 差异仅在 result 内部字段：图片为 images[]，视频为 videos[] / video_url。
type VideoTaskResultData struct {
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	Progress   int               `json:"progress"`
	Created    int64             `json:"created,omitempty"`
	Completed  int64             `json:"completed,omitempty"`
	ActualTime int               `json:"actual_time,omitempty"`
	Result     *VideoTaskResult  `json:"result,omitempty"`
	Error      *TaskError        `json:"error,omitempty"`
}

// VideoTaskResult 视频任务结果。
// video_url 与 videos[].url[] 同时存在（上游两份文档结构一致），
// 解析时优先 video_url，回退 videos[0].url[0]，避免上游只给其中一种时取不到值。
type VideoTaskResult struct {
	VideoURL string        `json:"video_url,omitempty"`
	Videos   []TaskVideo   `json:"videos,omitempty"`
}

// TaskVideo 生成的视频
type TaskVideo struct {
	URL []string `json:"url"`
	// ExpiresAt 视频 URL 过期时间戳（Unix 秒）。
	// 对齐 TaskImage.ExpiresAt —— 图片侧本就有该字段，视频侧此前缺失。
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

// VideoQueryTaskResponse 视频任务查询响应
type VideoQueryTaskResponse struct {
	Code int                 `json:"code"`
	Data VideoTaskResultData `json:"data"`
}

// FirstVideoURL 按「video_url 优先，videos[].url[] 回退」提取视频地址。
// 返回空字符串表示上游未给出可用地址。
func (r *VideoTaskResult) FirstVideoURL() string {
	if r == nil {
		return ""
	}
	if r.VideoURL != "" {
		return r.VideoURL
	}
	for _, v := range r.Videos {
		for _, u := range v.URL {
			if u != "" {
				return u
			}
		}
	}
	return ""
}
