package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// TestModelMappingV1Fix 测试 V1 映射修复
func TestModelMappingV1Fix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	tests := []struct {
		name          string
		modelMapping  string
		originModel   string
		expectedModel string
		expectMapped  bool
	}{
		{
			name:          "V1映射：有匹配",
			modelMapping:  `{"deepseek-flash":"deepseek/deepseek-v4.1-flash"}`,
			originModel:   "deepseek-flash",
			expectedModel: "deepseek/deepseek-v4.1-flash",
			expectMapped:  true,
		},
		{
			name:          "V1映射：无匹配",
			modelMapping:  `{"deepseek-flash":"deepseek/deepseek-v4.1-flash"}`,
			originModel:   "gpt-4",
			expectedModel: "gpt-4",
			expectMapped:  false,
		},
		{
			name:          "无映射配置",
			modelMapping:  "",
			originModel:   "gpt-4",
			expectedModel: "gpt-4",
			expectMapped:  false,
		},
		{
			name:          "空映射配置",
			modelMapping:  "{}",
			originModel:   "gpt-4",
			expectedModel: "gpt-4",
			expectMapped:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Set("model_mapping", tt.modelMapping)

			info := &common.RelayInfo{
				OriginModelName: tt.originModel,
			}

			request := &dto.GeneralOpenAIRequest{
				Model: tt.originModel,
			}

			err := ModelMappedHelper(c, info, request)
			if err != nil {
				t.Errorf("ModelMappedHelper() error = %v", err)
				return
			}

			// 验证 UpstreamModelName 被正确设置
			if info.UpstreamModelName == "" {
				t.Errorf("UpstreamModelName 为空！期望 %s", tt.expectedModel)
			}

			if info.UpstreamModelName != tt.expectedModel {
				t.Errorf("UpstreamModelName = %s, 期望 %s", info.UpstreamModelName, tt.expectedModel)
			}

			if info.IsModelMapped != tt.expectMapped {
				t.Errorf("IsModelMapped = %v, 期望 %v", info.IsModelMapped, tt.expectMapped)
			}

			// 验证 request 的模型名被正确设置
			if request.Model != tt.expectedModel {
				t.Errorf("request.Model = %s, 期望 %s", request.Model, tt.expectedModel)
			}
		})
	}
}
