package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/tidwall/gjson"
)

// MappingConfigV2 V2版本的模型映射配置
type MappingConfigV2 struct {
	Version  int                `json:"version"`
	Rules    []MappingRule      `json:"rules"`
	V1Compat map[string]string  `json:"-"` // V1兼容字段，不序列化
}

// MappingRule 映射规则
type MappingRule struct {
	SourceModel string      `json:"source_model"`
	Conditions  []Condition `json:"conditions,omitempty"`
	TargetModel string      `json:"target_model"`
	Priority    int         `json:"priority,omitempty"`
	Description string      `json:"description,omitempty"`
}

// Condition 条件
type Condition struct {
	Param    string      `json:"param"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value,omitempty"`
}

// ParamValue 参数提取结果
type ParamValue struct {
	Value  interface{}
	Exists bool // 参数是否存在且非nil
}

// isV2Mapping 检测是否为V2配置
func isV2Mapping(mappingJSON string) bool {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(mappingJSON), &config); err != nil {
		return false
	}
	version, ok := config["version"]
	if !ok {
		return false
	}
	versionFloat, ok := version.(float64)
	return ok && versionFloat == 2
}

// parseV2ConfigWithV1Compat 解析V2配置并提取V1兼容字段
func parseV2ConfigWithV1Compat(mappingJSON string) (*MappingConfigV2, error) {
	// 先解析为通用map
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(mappingJSON), &rawConfig); err != nil {
		return nil, err
	}

	// 解析为V2结构
	var config MappingConfigV2
	if err := json.Unmarshal([]byte(mappingJSON), &config); err != nil {
		return nil, err
	}

	// 提取V1兼容映射（除了version、rules之外的顶层字符串字段）
	config.V1Compat = make(map[string]string)
	for key, value := range rawConfig {
		// 跳过V2保留字段
		if key == "version" || key == "rules" {
			continue
		}

		// 其他字符串字段视为V1格式的映射
		if strValue, ok := value.(string); ok {
			config.V1Compat[key] = strValue
		}
	}

	return &config, nil
}

// extractParam 从请求中提取参数
func extractParam(request *dto.GeneralOpenAIRequest, path string, cachedJSON []byte) ParamValue {
	parts := strings.Split(path, ".")

	// 一级字段：直接访问（性能最优）
	if len(parts) == 1 {
		return extractFirstLevelParam(request, path)
	}

	// 多级字段：使用gjson解析
	if cachedJSON == nil {
		return ParamValue{Exists: false}
	}

	result := gjson.GetBytes(cachedJSON, path)
	if !result.Exists() {
		return ParamValue{Exists: false}
	}

	return ParamValue{Value: result.Value(), Exists: true}
}

// extractFirstLevelParam 直接访问一级字段
func extractFirstLevelParam(request *dto.GeneralOpenAIRequest, field string) ParamValue {
	switch field {
	case "temperature":
		if request.Temperature != nil {
			return ParamValue{Value: *request.Temperature, Exists: true}
		}
	case "max_tokens":
		if request.MaxTokens != nil {
			return ParamValue{Value: float64(*request.MaxTokens), Exists: true}
		}
	case "max_completion_tokens":
		if request.MaxCompletionTokens != nil {
			return ParamValue{Value: float64(*request.MaxCompletionTokens), Exists: true}
		}
	case "top_p":
		if request.TopP != nil {
			return ParamValue{Value: *request.TopP, Exists: true}
		}
	case "top_k":
		if request.TopK != nil {
			return ParamValue{Value: float64(*request.TopK), Exists: true}
		}
	case "stream":
		if request.Stream != nil {
			return ParamValue{Value: *request.Stream, Exists: true}
		}
	case "frequency_penalty":
		if request.FrequencyPenalty != nil {
			return ParamValue{Value: *request.FrequencyPenalty, Exists: true}
		}
	case "presence_penalty":
		if request.PresencePenalty != nil {
			return ParamValue{Value: *request.PresencePenalty, Exists: true}
		}
	case "n":
		if request.N != nil {
			return ParamValue{Value: float64(*request.N), Exists: true}
		}
	case "reasoning_effort":
		if request.ReasoningEffort != "" {
			return ParamValue{Value: request.ReasoningEffort, Exists: true}
		}
	case "model":
		if request.Model != "" {
			return ParamValue{Value: request.Model, Exists: true}
		}
	}

	// 尝试使用反射访问其他字段
	return extractParamByReflection(request, field)
}

// extractParamByReflection 使用反射提取参数（备用方案）
func extractParamByReflection(request *dto.GeneralOpenAIRequest, field string) ParamValue {
	v := reflect.ValueOf(request)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// 通过JSON tag查找字段
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		jsonTag := structField.Tag.Get("json")
		if jsonTag == "" {
			continue
		}

		tagName := strings.Split(jsonTag, ",")[0]
		if tagName == field {
			fieldValue := v.Field(i)

			// 处理指针类型
			if fieldValue.Kind() == reflect.Ptr {
				if fieldValue.IsNil() {
					return ParamValue{Exists: false}
				}
				fieldValue = fieldValue.Elem()
			}

			return ParamValue{Value: fieldValue.Interface(), Exists: true}
		}
	}

	return ParamValue{Exists: false}
}

// evaluateCondition 评估单个条件
func evaluateCondition(condition Condition, paramValue ParamValue) (bool, string) {
	// 参数不存在，条件失败
	if !paramValue.Exists {
		return false, "参数不存在或为nil"
	}

	switch condition.Operator {
	case ">":
		return compareGreaterThan(paramValue.Value, condition.Value)
	case ">=":
		return compareGreaterThanOrEqual(paramValue.Value, condition.Value)
	case "<":
		return compareLessThan(paramValue.Value, condition.Value)
	case "<=":
		return compareLessThanOrEqual(paramValue.Value, condition.Value)
	case "==":
		return compareEqual(paramValue.Value, condition.Value)
	case "!=":
		return compareNotEqual(paramValue.Value, condition.Value)
	case "in":
		return compareIn(paramValue.Value, condition.Value)
	case "not_in":
		return compareNotIn(paramValue.Value, condition.Value)
	default:
		return false, fmt.Sprintf("不支持的运算符: %s", condition.Operator)
	}
}

// 比较函数

func compareGreaterThan(actual, expected interface{}) (bool, string) {
	actualNum, expectedNum, err := convertToFloat64(actual, expected)
	if err != nil {
		return false, err.Error()
	}
	result := actualNum > expectedNum
	if !result {
		return false, fmt.Sprintf("%v 不满足 > %v", actual, expected)
	}
	return true, ""
}

func compareGreaterThanOrEqual(actual, expected interface{}) (bool, string) {
	actualNum, expectedNum, err := convertToFloat64(actual, expected)
	if err != nil {
		return false, err.Error()
	}
	result := actualNum >= expectedNum
	if !result {
		return false, fmt.Sprintf("%v 不满足 >= %v", actual, expected)
	}
	return true, ""
}

func compareLessThan(actual, expected interface{}) (bool, string) {
	actualNum, expectedNum, err := convertToFloat64(actual, expected)
	if err != nil {
		return false, err.Error()
	}
	result := actualNum < expectedNum
	if !result {
		return false, fmt.Sprintf("%v 不满足 < %v", actual, expected)
	}
	return true, ""
}

func compareLessThanOrEqual(actual, expected interface{}) (bool, string) {
	actualNum, expectedNum, err := convertToFloat64(actual, expected)
	if err != nil {
		return false, err.Error()
	}
	result := actualNum <= expectedNum
	if !result {
		return false, fmt.Sprintf("%v 不满足 <= %v", actual, expected)
	}
	return true, ""
}

func compareEqual(actual, expected interface{}) (bool, string) {
	// 尝试数值比较
	actualNum, expectedNum, err := convertToFloat64(actual, expected)
	if err == nil {
		result := actualNum == expectedNum
		if !result {
			return false, fmt.Sprintf("%v 不等于 %v", actual, expected)
		}
		return true, ""
	}

	// 字符串比较
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	result := actualStr == expectedStr
	if !result {
		return false, fmt.Sprintf("%v 不等于 %v", actual, expected)
	}
	return true, ""
}

func compareNotEqual(actual, expected interface{}) (bool, string) {
	result, _ := compareEqual(actual, expected)
	if result {
		return false, fmt.Sprintf("%v 等于 %v", actual, expected)
	}
	return true, ""
}

func compareIn(actual, expected interface{}) (bool, string) {
	expectedArray, ok := expected.([]interface{})
	if !ok {
		return false, "in 运算符的 value 必须是数组"
	}

	for _, item := range expectedArray {
		if equal, _ := compareEqual(actual, item); equal {
			return true, ""
		}
	}

	return false, fmt.Sprintf("%v 不在 %v 中", actual, expected)
}

func compareNotIn(actual, expected interface{}) (bool, string) {
	result, _ := compareIn(actual, expected)
	if result {
		return false, fmt.Sprintf("%v 在 %v 中", actual, expected)
	}
	return true, ""
}

// convertToFloat64 将值转换为float64用于数值比较
func convertToFloat64(actual, expected interface{}) (float64, float64, error) {
	actualNum, err := toFloat64(actual)
	if err != nil {
		return 0, 0, fmt.Errorf("无法将 %v 转换为数值", actual)
	}

	expectedNum, err := toFloat64(expected)
	if err != nil {
		return 0, 0, fmt.Errorf("无法将 %v 转换为数值", expected)
	}

	return actualNum, expectedNum, nil
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, err
		}
		return f, nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// applyModelMappingV2 应用V2映射规则
func applyModelMappingV2(mappingJSON string, request *dto.GeneralOpenAIRequest, originModelName string) (mappedModel string, isModelMapped bool, err error) {
	ctx := context.Background()

	// 解析配置（包含V1兼容字段）
	config, parseErr := parseV2ConfigWithV1Compat(mappingJSON)
	if parseErr != nil {
		logger.LogError(ctx, fmt.Sprintf("模型映射V2配置解析失败: %v", parseErr))
		return originModelName, false, fmt.Errorf("model_mapping_v2_parse_failed")
	}

	// 筛选匹配源模型的规则
	var matchedRules []MappingRule
	for _, rule := range config.Rules {
		if rule.SourceModel == originModelName {
			matchedRules = append(matchedRules, rule)
		}
	}

	// 有匹配规则，进行条件评估
	if len(matchedRules) > 0 {
		// 按优先级排序
		sort.Slice(matchedRules, func(i, j int) bool {
			pi := matchedRules[i].Priority
			pj := matchedRules[j].Priority
			if pi == 0 {
				pi = 999
			}
			if pj == 0 {
				pj = 999
			}
			return pi < pj
		})

		// 懒序列化：只在需要嵌套字段时才序列化
		var cachedJSON []byte
		needsJSON := false
		for _, rule := range matchedRules {
			for _, condition := range rule.Conditions {
				if strings.Contains(condition.Param, ".") {
					needsJSON = true
					break
				}
			}
			if needsJSON {
				break
			}
		}

		if needsJSON {
			cachedJSON, _ = json.Marshal(request)
		}

		// 评估规则
		for ruleIndex, rule := range matchedRules {
			allConditionsMet := true
			var failedReason string

			// 无条件规则，直接匹配
			if len(rule.Conditions) == 0 {
				logger.LogInfo(ctx, fmt.Sprintf("模型映射V2: 规则[%d]匹配成功（无条件） %s -> %s",
					ruleIndex, originModelName, rule.TargetModel))
				return rule.TargetModel, true, nil
			}

			// 评估所有条件（AND逻辑）
			for _, condition := range rule.Conditions {
				paramValue := extractParam(request, condition.Param, cachedJSON)

				matched, reason := evaluateCondition(condition, paramValue)
				if !matched {
					allConditionsMet = false
					failedReason = reason
					logger.LogDebug(ctx, fmt.Sprintf("模型映射V2: 规则[%d]条件不匹配 [%s %s %v]: %s",
						ruleIndex, condition.Param, condition.Operator, condition.Value, reason))
					break
				}
			}

			if allConditionsMet {
				logger.LogInfo(ctx, fmt.Sprintf("模型映射V2: 规则[%d]匹配成功 %s -> %s (优先级=%d, 描述=%s)",
					ruleIndex, originModelName, rule.TargetModel, rule.Priority, rule.Description))
				return rule.TargetModel, true, nil
			}

			logger.LogDebug(ctx, fmt.Sprintf("模型映射V2: 规则[%d]不匹配，原因: %s", ruleIndex, failedReason))
		}
	}

	// 所有规则都不匹配，尝试V1兼容映射
	if len(config.V1Compat) > 0 {
		if targetModel, exists := config.V1Compat[originModelName]; exists && targetModel != "" {
			logger.LogInfo(ctx, fmt.Sprintf("模型映射V2: 使用V1兼容映射 %s -> %s", originModelName, targetModel))
			return targetModel, true, nil
		}
	}

	// 无映射
	logger.LogDebug(ctx, fmt.Sprintf("模型映射V2: 无匹配规则，使用原模型 %s", originModelName))
	return originModelName, false, nil
}
