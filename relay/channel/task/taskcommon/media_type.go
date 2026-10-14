package taskcommon

import "github.com/QuantumNous/new-api/common"

// IsVideoTaskResponse 按查询响应体结构判定是否为视频任务结果。
//
// 为什么按结构判定而不是按 adaptor 状态判定：
// 轮询（service/task_polling.go）会通过 GetTaskAdaptorFunc 新建适配器实例并重新
// Init，提交阶段缓存的 videoTask 标记不会带到查询阶段，用标记会导致视频结果被
// 按图片解析而取不到 URL（该教训已在 relay/channel/task/td/adaptor.go 的注释中记录）。
//
// 判定依据：result 中出现 video_url 或 videos 字段。
// 图片结果的 result 只有 images，两者不会同时出现，故判定无歧义。
//
// 单一数据源说明：
// 本函数被两处调用，必须保持同一份实现，避免将来判据变更时出现不同步：
//  1. relay/channel/task/td 的 ParseTaskResult —— 决定按图片还是视频解析结果；
//  2. service 的 OSS 转存判定 —— 决定走「图片转存开关」还是「视频转存开关」。
//
// 之所以下沉到 taskcommon 而不是放在 td 包：td 包已导入 service 包，
// service 若反向导入 td 会形成循环依赖，故把共用判据放在两者都可依赖的中间层。
func IsVideoTaskResponse(respBody []byte) bool {
	var probe struct {
		Data struct {
			Result map[string]any `json:"result"`
		} `json:"data"`
	}
	if err := common.Unmarshal(respBody, &probe); err != nil {
		return false
	}
	if probe.Data.Result == nil {
		return false
	}
	if _, ok := probe.Data.Result["video_url"]; ok {
		return true
	}
	if _, ok := probe.Data.Result["videos"]; ok {
		return true
	}
	return false
}
