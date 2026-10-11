package td

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
	AspectRatio string   `json:"aspect_ratio,omitempty"` // 16:9 / 9:16
	Resolution  string   `json:"resolution,omitempty"`   // 720p / 1080p / 2k
	Images      []string `json:"images,omitempty"`       // 参考图
	Videos      []string `json:"videos,omitempty"`       // 参考视频
	Audios      []string `json:"audios,omitempty"`       // 参考音频

	// GenerateAudio 视频音效同步开关。
	// 指针类型：显式传 false 与未传语义不同，不能因 omitempty 丢掉 false。
	GenerateAudio *bool `json:"generate_audio,omitempty"`
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
