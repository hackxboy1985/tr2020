package doubao

import (
	"encoding/json"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"math"
)

// NormalizeTaskResponse 只修正新收到的上游响应，不修改原始字节或再次处理已落库数据。
func (a *TaskAdaptor) NormalizeTaskResponse(body []byte) ([]byte, error) {
	if a.tokenFix == nil {
		return body, nil
	}
	fix := *a.tokenFix
	common.SysLog(fmt.Sprintf("[Doubao TokenFix] normalize start: token_fix=%.4f body_bytes=%d", fix, len(body)))
	if math.IsNaN(fix) || math.IsInf(fix, 0) || fix < 0.5 {
		return nil, fmt.Errorf("tokenFix must be finite and >= 0.5")
	}
	if fix == 1 {
		return body, nil
	}
	var response map[string]json.RawMessage
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	raw, ok := response["usage"]
	if !ok || string(raw) == "null" {
		common.SysLog("[Doubao TokenFix] usage missing/null, response unchanged")
		return body, nil
	}
	var usage map[string]json.RawMessage
	if err := common.Unmarshal(raw, &usage); err != nil {
		return nil, err
	}
	for _, field := range []string{"completion_tokens", "total_tokens"} {
		value, exists := usage[field]
		if !exists {
			continue
		}
		var tokens int64
		if err := common.Unmarshal(value, &tokens); err != nil {
			return nil, fmt.Errorf("invalid usage.%s: %w", field, err)
		}
		corrected := math.Round(float64(tokens) / fix)
		if tokens < 0 || corrected >= math.Exp2(63) {
			return nil, fmt.Errorf("usage.%s out of range", field)
		}
		encoded, err := common.Marshal(int64(corrected))
		if err != nil {
			return nil, err
		}
		usage[field] = encoded
		common.SysLog(fmt.Sprintf("[Doubao TokenFix] adjusted %s: upstream=%d adjusted=%d token_fix=%.4f", field, tokens, int64(corrected), fix))
	}
	encoded, err := common.Marshal(usage)
	if err != nil {
		return nil, err
	}
	response["usage"] = encoded
	return common.Marshal(response)
}
