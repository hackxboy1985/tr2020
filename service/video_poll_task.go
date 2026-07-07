package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	videoPollInterval  = 1 * time.Minute
	videoPollBatchSize = 50
)

var (
	videoPollOnce    sync.Once
	videoPollRunning atomic.Bool
)

// StartVideoPollTask 启动视频状态轮询定时任务
func StartVideoPollTask() {
	videoPollOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("video poll task started: tick=%s", videoPollInterval))
			ticker := time.NewTicker(videoPollInterval)
			defer ticker.Stop()
			for range ticker.C {
				runVideoPollOnce()
			}
		})
	})
}

func runVideoPollOnce() {
	if !videoPollRunning.CompareAndSwap(false, true) {
		return
	}
	defer videoPollRunning.Store(false)

	ctx := context.Background()
	offset := 0
	total := 0
	for {
		projects, err := model.GetActiveVideoProjects(offset, videoPollBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("video poll: query active projects failed: %v", err))
			return
		}
		if len(projects) == 0 {
			break
		}
		for _, p := range projects {
			_, _, localErr, upstreamErr := GetProject(ctx, p.Id, p.UserId, true)
			if localErr != nil {
				logger.LogWarn(ctx, fmt.Sprintf("video poll: project %d not found: %v", p.Id, localErr))
			} else if upstreamErr != nil {
				logger.LogWarn(ctx, fmt.Sprintf("video poll: project %d upstream error: %v", p.Id, upstreamErr))
			}
			total++
		}
		if len(projects) < videoPollBatchSize {
			break
		}
		offset += videoPollBatchSize
	}
	if total > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("video poll: polled %d projects", total))
	} else {
		logger.LogInfo(ctx, "video poll: no active projects")
	}
}
