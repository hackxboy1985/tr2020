package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

func TestOssEnabledChannelsFilter(t *testing.T) {
	truncateTables(t)
	for i, tc := range []struct {
		channelType int
		settings    dto.ChannelOtherSettings
		want        bool
	}{
		{constant.ChannelTypeTudou, dto.ChannelOtherSettings{TdOssImageEnabled: true}, true},
		{constant.ChannelTypeZy, dto.ChannelOtherSettings{ZyOssVideoEnabled: true}, true},
		{constant.ChannelTypeZy, dto.ChannelOtherSettings{}, false},
		{constant.ChannelTypeZy, dto.ChannelOtherSettings{TdOssImageEnabled: true}, false},
		{constant.ChannelTypeRR, dto.ChannelOtherSettings{ZyOssImageEnabled: true}, false},
	} {
		ch := &Channel{Id: 8800 + i, Type: tc.channelType}
		ch.SetOtherSettings(tc.settings)
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	channels, err := GetChannelsWithOssTransferEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 {
		t.Fatalf("enabled channels = %d, want 2", len(channels))
	}
	for _, ch := range channels {
		if ch.Id != 8800 && ch.Id != 8801 {
			t.Fatalf("unexpected channel: %d", ch.Id)
		}
	}
}

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
