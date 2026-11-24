package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type resultBillingAdaptor struct {
	result      *relaycommon.TaskInfo
	actualQuota int
	parseErr    error
}

func (a *resultBillingAdaptor) Init(*relaycommon.RelayInfo) {}
func (a *resultBillingAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	panic("unexpected upstream fetch")
}
func (a *resultBillingAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) {
	return a.result, a.parseErr
}
func (a *resultBillingAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int {
	return a.actualQuota
}

func TestApplyVideoTaskResultBillingOnce(t *testing.T) {
	cases := []struct {
		name, status           string
		actual, delta, logType int
		perCall                bool
	}{
		{"refund_difference", model.TaskStatusSuccess, 3000, -2000, model.LogTypeRefund, false},
		{"extra_charge", model.TaskStatusSuccess, 7000, 2000, model.LogTypeConsume, false},
		{"zero_difference", model.TaskStatusSuccess, 5000, 0, model.LogTypeConsume, false},
		{"per_call", model.TaskStatusSuccess, 7000, 0, model.LogTypeConsume, true},
		{"failure", model.TaskStatusFailure, 0, -5000, model.LogTypeRefund, false},
		{"cancelled", model.TaskStatusCancelled, 0, -5000, model.LogTypeRefund, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			seedUser(t, 401, 10000)
			seedToken(t, 401, 401, "sk-result-test", 10000)
			seedChannel(t, 401)
			task := makeTask(401, 401, 5000, 401, BillingSourceWallet, 0)
			task.PrivateData.BillingContext.PerCallBilling = tc.perCall
			require.NoError(t, model.DB.Create(task).Error)
			stale := *task
			adaptor := &resultBillingAdaptor{result: &relaycommon.TaskInfo{Status: tc.status, Progress: "100%", Url: "https://example.com/video.mp4", Reason: "failed", OriTaskID: "cgt-result"}, actualQuota: tc.actual}
			body := []byte(`{"status":"finished"}`)
			require.NoError(t, ApplyVideoTaskResult(context.Background(), adaptor, task, body))
			require.EqualValues(t, tc.status, task.Status)
			require.Positive(t, task.FinishTime)
			require.Equal(t, 10000-tc.delta, getUserQuota(t, 401))
			require.Equal(t, 10000-tc.delta, getTokenRemainQuota(t, 401))
			require.Equal(t, int64(1), countLogs(t))
			log := getLastLog(t)
			require.Equal(t, tc.logType, log.Type)
			require.Equal(t, tc.delta, log.Quota)
			if tc.delta == 0 {
				require.Equal(t, "查询结果", log.Content)
			}
			if tc.status == model.TaskStatusSuccess {
				require.Equal(t, adaptor.result.Url, task.GetResultURL())
			}
			// Another fetch repeats the result; a poller uses an older snapshot.
			require.NoError(t, ApplyVideoTaskResult(context.Background(), adaptor, task, body))
			require.NoError(t, ApplyVideoTaskResult(context.Background(), adaptor, &stale, body))
			require.EqualValues(t, tc.status, stale.Status)
			require.Equal(t, 10000-tc.delta, getUserQuota(t, 401))
			require.Equal(t, int64(1), countLogs(t))
			// A late running response cannot roll back a terminal state.
			adaptor.result = &relaycommon.TaskInfo{Status: model.TaskStatusInProgress, Progress: "50%"}
			require.NoError(t, ApplyVideoTaskResult(context.Background(), adaptor, task, []byte("{}")))
			require.EqualValues(t, tc.status, task.Status)
		})
	}
}

func TestApplyVideoTaskResultErrorDoesNotExposeTerminalState(t *testing.T) {
	for _, name := range []string{"parse_error", "nil_result", "write_error"} {
		t.Run(name, func(t *testing.T) {
			truncate(t)
			task := makeTask(402, 402, 5000, 0, BillingSourceWallet, 0)
			require.NoError(t, model.DB.Create(task).Error)
			before := task.Snapshot()
			adaptor := &resultBillingAdaptor{result: &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, Progress: "100%"}}
			if name == "parse_error" {
				adaptor.parseErr = fmt.Errorf("invalid usage")
			}
			if name == "nil_result" {
				adaptor.result = nil
			}
			if name == "write_error" {
				callback := "test:reject_result_update"
				require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) { tx.AddError(fmt.Errorf("write rejected")) }))
				defer model.DB.Callback().Update().Remove(callback)
			}
			require.Error(t, ApplyVideoTaskResult(context.Background(), adaptor, task, []byte("{}")))
			require.True(t, before.Equal(task.Snapshot()))
			require.Equal(t, int64(0), countLogs(t))
		})
	}
}
