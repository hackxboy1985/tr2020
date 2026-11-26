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
	if _, ok := response["billing"]; !ok {
		return body, nil
	}
	delete(response, "billing")
	return common.Marshal(response)
}
