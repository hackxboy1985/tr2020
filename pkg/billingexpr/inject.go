package billingexpr

import "github.com/tidwall/sjson"

// InjectBodyParam 向 JSON body 注入或覆盖一个字段（使用 tidwall/sjson）。
// body 为 nil 或空时从 {} 开始构建。注入失败时返回原 body，不 panic。
// 可被 TaskAdaptor.InjectBillingParams 调用，用于将转换后的上游参数
// 写入 info.BillingRequestInput.Body，供计费表达式 param() 读取。
func InjectBodyParam(body []byte, key string, value any) []byte {
	if len(body) == 0 {
		body = []byte("{}")
	}
	result, err := sjson.SetBytes(body, key, value)
	if err != nil {
		return body
	}
	return result
}
