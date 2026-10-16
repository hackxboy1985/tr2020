package model

import "testing"

func TestRefreshOssTransferUsesPersistedPrivateData(t *testing.T) {
	truncateTables(t)
	task := &Task{TaskID: "task_oss_refresh", Status: TaskStatusSuccess,
		PrivateData: TaskPrivateData{ResultURL: "https://upstream.example/image.png", OssTransferAttempts: 2}}
	insertTask(t, task)
	stale := *task
	stale.PrivateData.ResultURL = "https://stale.example/image.png"
	stale.PrivateData.OssTransferAttempts = 1
	if err := stale.RefreshPrivateData(); err != nil {
		t.Fatal(err)
	}
	if stale.PrivateData.OssTransferAttempts != 2 || stale.PrivateData.ResultURL != task.PrivateData.ResultURL {
		t.Fatalf("stale private data retained: %+v", stale.PrivateData)
	}
	stale.PrivateData.OssTransferAttempts = 3
	if stale.CanRetryOssTransfer(3) {
		t.Fatal("must stop after three total attempts")
	}
}
