package doubao

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestTokenFixNormalize(t *testing.T) {
	raw := []byte(`{"status":"succeeded","content":{"video_url":"https://example.com/a.mp4"},"usage":{"SR":"480p","audio":true,"completion_tokens":26386,"total_tokens":26386,"duration":4,"ratio":"9:16"}}`)
	for _, tc := range []struct {
		name    string
		fix     *float64
		want    int
		invalid bool
	}{
		{"absent", nil, 26386, false}, {"example", floatPtr(.65), 40594, false},
		{"boundary", floatPtr(.5), 52772, false}, {"one", floatPtr(1), 26386, false},
		{"above_one", floatPtr(2), 13193, false}, {"zero", floatPtr(0), 0, true},
		{"negative", floatPtr(-1), 0, true}, {"too_small", floatPtr(.49), 0, true},
		{"nan", floatPtr(math.NaN()), 0, true}, {"infinite", floatPtr(math.Inf(1)), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &TaskAdaptor{tokenFix: tc.fix}
			normalized, err := a.NormalizeTaskResponse(raw)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, status := range []string{"succeeded", "completed"} {
				var data map[string]interface{}
				require.NoError(t, common.Unmarshal(normalized, &data))
				data["status"] = status
				body, err := common.Marshal(data)
				require.NoError(t, err)
				result, err := a.ParseTaskResult(body)
				require.NoError(t, err)
				require.Equal(t, tc.want, result.TotalTokens)
				require.Equal(t, tc.want, result.CompletionTokens)
				// 查询转换直接读修正后的落库数据，多次调用不会重复除系数。
				delete(data, "content") // 本测试只测用量，不访问渠道数据库。
				queryBody, err := common.Marshal(data)
				require.NoError(t, err)
				task := &model.Task{Data: queryBody}
				task.Status = model.TaskStatusSuccess
				for i := 0; i < 2; i++ {
					out, err := a.ConvertToOpenAIVideo(task)
					require.NoError(t, err)
					var reply struct {
						Usage struct {
							Total int `json:"total_tokens"`
						} `json:"usage"`
					}
					require.NoError(t, common.Unmarshal(out, &reply))
					require.Equal(t, tc.want, reply.Usage.Total)
				}
				usage := data["usage"].(map[string]interface{})
				require.Equal(t, "480p", usage["SR"])
				require.Equal(t, true, usage["audio"])
			}
			require.Contains(t, string(raw), "26386")
		})
	}
}
func floatPtr(v float64) *float64 { return &v }

func TestTokenFixInitAndMissingUsage(t *testing.T) {
	a := &TaskAdaptor{}
	a.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.Nil(t, a.tokenFix)
	a.tokenFix = floatPtr(.65)
	for _, body := range []string{`{"id":"a"}`, `{"usage":null}`, `{"usage":{"total_tokens":0}}`} {
		out, err := a.NormalizeTaskResponse([]byte(body))
		require.NoError(t, err)
		require.JSONEq(t, body, string(out))
	}
	_, err := a.NormalizeTaskResponse([]byte(`{"usage":{"total_tokens":9223372036854775807}}`))
	require.Error(t, err)
}
