package doubao

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestTaskResultToolUsageCompatibility(t *testing.T) {
	// Keep this test sequential while overriding the global cache flag.
	// The converter only needs optional channel settings, not a database.
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCacheEnabled })

	cases := []struct{ name, toolUsage string }{
		{"missing", ""},
		{"empty_object", "{}"},
		{"null", "null"},
		{"integer", `{"web_search":0}`},
		{"positive_integer", `{"web_search":3}`},
		{"decimal", `{"web_search":1.5}`},
		{"large_number", `{"web_search":999999999999999999999999999999}`},
		{"numeric_string", `{"web_search":"0"}`},
		{"empty_string", `{"web_search":""}`},
		{"text_string", `{"web_search":"unavailable"}`},
		{"boolean", `{"web_search":true}`},
		{"false", `{"web_search":false}`},
		{"null_search", `{"web_search":null}`},
		{"array", `{"web_search":[0,"1"]}`},
		{"object", `{"web_search":{"count":1}}`},
		{"tool_usage_string", `"unavailable"`},
		{"tool_usage_array", "[]"},
		{"tool_usage_number", "0"},
		{"tool_usage_boolean", "false"},
	}
	statuses := []struct {
		upstream string
		internal model.TaskStatus
	}{
		{"succeeded", model.TaskStatusSuccess},
		{"failed", model.TaskStatusFailure},
		{"cancelled", model.TaskStatusCancelled},
		{"running", model.TaskStatusInProgress},
	}
	for _, tc := range cases {
		for _, status := range statuses {
			t.Run(tc.name+"/"+status.upstream, func(t *testing.T) {
				toolUsage := ""
				if tc.toolUsage != "" {
					toolUsage = `,"tool_usage":` + tc.toolUsage
				}
				body := []byte(fmt.Sprintf(
					`{"id":"upstream_1","status":%q,"content":{"video_url":"https://cdn.example.com/video.mp4"},"error":{"code":"failed","message":"upstream failure"},"usage":{"completion_tokens":123,"total_tokens":456%s}}`,
					status.upstream, toolUsage))
				adaptor := &TaskAdaptor{}
				result, err := adaptor.ParseTaskResult(body)
				if err != nil {
					t.Fatalf("ParseTaskResult: %v", err)
				}
				if result.Status != string(status.internal) {
					t.Fatalf("status = %s, want %s", result.Status, status.internal)
				}
				if status.internal == model.TaskStatusSuccess {
					if result.Url != "https://cdn.example.com/video.mp4" || result.CompletionTokens != 123 || result.TotalTokens != 456 {
						t.Fatalf("lost result or billing usage: %+v", result)
					}
				}
				if status.internal == model.TaskStatusFailure && result.Reason != "upstream failure" {
					t.Fatalf("lost failure reason: %+v", result)
				}
				task := &model.Task{TaskID: "task_1", Status: status.internal, Data: body}
				converted, err := adaptor.ConvertToOpenAIVideo(task)
				if err != nil {
					t.Fatalf("ConvertToOpenAIVideo: %v", err)
				}
				var video dto.OpenAIVideo
				if err := common.Unmarshal(converted, &video); err != nil {
					t.Fatal(err)
				}
				if video.ID != task.TaskID || video.Status != task.Status.ToVideoStatus() {
					t.Fatalf("unexpected video: %+v", video)
				}
				if video.Usage == nil || video.Usage.CompletionTokens != 123 || video.Usage.TotalTokens != 456 {
					t.Fatalf("lost video usage: %+v", video.Usage)
				}
			})
		}
	}
}

func TestTaskResultRejectsMalformedJSONAndInvalidBillingUsage(t *testing.T) {
	for _, body := range []string{
		`{"status":"succeeded","usage":{"tool_usage":{"web_search":}}}`,
		`{"status":"succeeded","usage":{"completion_tokens":"invalid"}}`,
		`{"status":"succeeded","usage":{"total_tokens":{}}}`,
	} {
		adaptor := &TaskAdaptor{}
		if _, err := adaptor.ParseTaskResult([]byte(body)); err == nil {
			t.Fatalf("accepted invalid response: %s", body)
		}
		if _, err := adaptor.ConvertToOpenAIVideo(&model.Task{Data: []byte(body)}); err == nil {
			t.Fatalf("converted invalid response: %s", body)
		}
	}
}
