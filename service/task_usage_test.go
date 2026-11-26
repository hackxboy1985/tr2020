package service

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
	"testing"
)

type usageResultAdaptor struct{ resultBillingAdaptor }

func (a *usageResultAdaptor) NormalizeTaskResponse(raw []byte) ([]byte, error) {
	return []byte(`{"status":"succeeded","usage":{"completion_tokens":40594,"total_tokens":40594}}`), nil
}
func (a *usageResultAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var reply struct {
		Usage struct {
			Completion int `json:"completion_tokens"`
			Total      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := common.Unmarshal(body, &reply); err != nil {
		return nil, err
	}
	return &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Url: "https://example.com/a.mp4", CompletionTokens: reply.Usage.Completion, TotalTokens: reply.Usage.Total}, nil
}

// 验证真实落库、钱包/令牌补扣、日志 token、原文保留和终态重复查询。
func TestNormalizedVideoUsageSettlement(t *testing.T) {
	truncate(t)
	old := ratio_setting.ModelRatio2JSONString()
	defer ratio_setting.UpdateModelRatioByJSONString(old)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"test-model":1}`))
	seedUser(t, 450, 100000)
	seedToken(t, 450, 450, "sk-usage-test", 100000)
	seedChannel(t, 450)
	task := makeTask(450, 450, 5000, 450, BillingSourceWallet, 0)
	require.NoError(t, model.DB.Create(task).Error)
	raw := []byte(`{"status":"succeeded","usage":{"completion_tokens":26386,"total_tokens":26386}}`)
	adaptor := &usageResultAdaptor{}
	require.NoError(t, ApplyVideoTaskResult(context.Background(), adaptor, task, raw))
	require.Equal(t, 40594, task.Quota)
	require.Equal(t, 64406, getUserQuota(t, 450))
	require.Equal(t, 64406, getTokenRemainQuota(t, 450))
	stored, exists, err := model.GetByTaskId(450, task.TaskID)
	require.NoError(t, err)
	require.True(t, exists)
	require.Contains(t, string(stored.Data), "40594")
	require.NotContains(t, string(stored.Data), "26386")
	log := getLastLog(t)
	require.Equal(t, 40594, log.CompletionTokens)
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	require.Equal(t, float64(40594), other["total_tokens"])
	require.Equal(t, string(raw), other["response_body"])
	require.NoError(t, ApplyVideoTaskResult(context.Background(), adaptor, task, raw))
	require.Equal(t, 40594, task.Quota)
	require.Equal(t, int64(1), countLogs(t))
}
