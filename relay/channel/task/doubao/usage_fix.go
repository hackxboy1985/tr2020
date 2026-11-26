package doubao

import (
	"encoding/json"
	"github.com/QuantumNous/new-api/common"
)

// stripBillingField 删除上游内部计费明细。Ark 任务响应不定义 billing；
// SubmitRespBody/Other.response_body 仍在调用处保存未经清理的原始响应。
func stripBillingField(body []byte) ([]byte, error) {
	var response map[string]json.RawMessage
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	changed := false
	if _, ok := response["billing"]; ok {
		delete(response, "billing")
		changed = true
	}
	if rawUsage, ok := response["usage"]; ok && string(rawUsage) != "null" {
		var usage map[string]json.RawMessage
		if err := common.Unmarshal(rawUsage, &usage); err != nil {
			return nil, err
		}
		for key := range usage {
			if key != "completion_tokens" && key != "total_tokens" {
				delete(usage, key)
				changed = true
			}
		}
		cleanUsage, err := common.Marshal(usage)
		if err != nil {
			return nil, err
		}
		response["usage"] = cleanUsage
	}
	if !changed {
		return body, nil
	}
	return common.Marshal(response)
}
