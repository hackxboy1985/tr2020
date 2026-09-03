package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// needsStatusPullthrough 判断该状态是否需要透传查询上游
// 非终态都需要查上游同步最新状态
func needsStatusPullthrough(status string) bool {
	switch status {
	case model.VideoProjectStatusOneClickGenerated, // 终态
		model.VideoProjectStatusSuccess,            // 终态（上游新名称）
		model.VideoProjectStatusFailed,             // 终态
		model.VideoProjectStatusVideoPreparing:     // 本地拼接失败，无需查上游
		return false
	}
	return true
}

// CreateProject 创建视频项目
// isAdmin=true 时 req.ChannelId 生效，否则忽略
// 返回值额外包含上游原始请求/响应体，以及渠道是否开启了保存开关
func CreateProject(ctx context.Context, userId int, isAdmin bool, req *dto.CreateVideoProjectRequest) (*model.VideoProject, []byte, []byte, bool, error) {
	// 获取用户名
	username, err := model.GetUsernameById(userId, false)
	if err != nil {
		username = fmt.Sprintf("user_%d", userId)
	}

	// 获取用户分组
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to get user: %w", err)
	}
	// Token 鉴权时优先用 token 的分组
	userGroup := req.TokenGroup
	if userGroup == "" {
		userGroup = user.Group
	}
	if userGroup == "" {
		userGroup = "default"
	}

	// 选择渠道
	var ch *model.VideoChannel
	if isAdmin && req.ChannelId > 0 {
		// 管理员指定渠道
		ch, err = model.GetVideoChannelById(req.ChannelId)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("channel not found: %w", err)
		}
		if ch.Enabled == 0 {
			return nil, nil, nil, false, fmt.Errorf("channel %d is disabled", req.ChannelId)
		}
	} else {
		// 按用户组 + 可选渠道类型，按权重随机选
		ch, err = model.SelectVideoChannel(userGroup, req.ChannelType)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("no available video channel for group '%s': %w", userGroup, err)
		}
	}

	// 构建适配器
	adapter, err := NewAdapterFromChannel(ch)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to build adapter: %w", err)
	}

	// 生成项目名称
	projectName := fmt.Sprintf("%s_%s_%d", username, time.Now().Format("20060102"), time.Now().Unix())

	// 序列化 MediaList（如果有）
	mediaListJSON := ""
	if len(req.MediaList) > 0 {
		if b, err := common.Marshal(req.MediaList); err == nil {
			mediaListJSON = string(b)
		}
	}

	// 创建本地记录
	project := &model.VideoProject{
		UserId:        userId,
		Username:      username,
		ProjectName:   projectName,
		ChannelId:     ch.Id,
		ChannelType:   ch.ChannelType, // 快照，后续不更新
		ProductImgUrl: req.ProductImgUrl,
		Brand:         req.Brand,
		ProductName:   req.ProductName,
		Tagline:       req.Tagline,
		SellingPoints: req.SellingPoints,
		Prompt:        req.Prompt,
		Vtype:         req.Vtype,
		VtypeAdd:      req.VtypeAdd,
		Language:      req.Language,
		Platform:      req.Platform,
		Region:        req.Region,
		Roles:         req.Roles,
		SelectAudios:  req.SelectAudios,
		MediaList:     mediaListJSON,
		Duration:      req.Duration,
		Resolution:    req.Resolution,
		VideoModel:    req.VideoModel,
		Whstr:         req.Whstr,
		Status:        model.VideoProjectStatusCreated,
	}

	if err := model.CreateVideoProject(project); err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to create project: %w", err)
	}

	saveBody := ch.SaveRequestResponse == 1

	// 调用上游
	resp, err := adapter.CreateProject(ctx, req)
	if err != nil {
		_ = model.UpdateVideoProjectStatus(project.Id, model.VideoProjectStatusFailed, err.Error())
		var rawReqOnErr []byte
		if resp != nil {
			rawReqOnErr = resp.RawRequest
		}
		return nil, rawReqOnErr, nil, false, fmt.Errorf("upstream channel error: %w", err)
	}

	// 更新 remote_project_id 和状态
	updates := map[string]interface{}{
		"remote_project_id": resp.RemoteProjectId,
		"status":            resp.Status,
	}
	_ = model.UpdateVideoProjectFields(project.Id, updates)
	project.RemoteProjectId = resp.RemoteProjectId
	project.Status = resp.Status

	return project, resp.RawRequest, resp.RawResponse, saveBody, nil
}

// GetProject 获取项目详情，进行中状态会透传查询上游
// 返回值：project, rawResponse, localErr(本地找不到), upstreamErr(上游查询失败)
func GetProject(ctx context.Context, projectId int64, userId int, isAdmin bool) (*model.VideoProject, []byte, error, error) {
	var project *model.VideoProject
	var err error

	if isAdmin {
		project, err = model.GetVideoProjectByIdAdmin(projectId)
	} else {
		project, err = model.GetVideoProjectById(projectId, userId)
	}
	if err != nil {
		return nil, nil, err, nil
	}

	// 终态或无 remote_project_id：直接返回本地数据
	if !needsStatusPullthrough(project.Status) || project.RemoteProjectId == "" {
		return project, nil, nil, nil
	}

	// 透传查询上游
	ch, err := model.GetVideoChannelById(project.ChannelId)
	if err != nil {
		// 渠道已删除，返回本地数据
		common.SysLog(fmt.Sprintf("video project %d channel %d not found: %v", project.Id, project.ChannelId, err))
		return project, nil, nil, nil
	}

	adapter, err := NewAdapterFromChannel(ch)
	if err != nil {
		return project, nil, nil, nil
	}

	statusResp, err := adapter.GetProjectStatus(ctx, project.RemoteProjectId)
	if err != nil {
		// 上游查询失败，返回本地数据，同时返回上游错误供调用方记录日志
		common.SysLog(fmt.Sprintf("video project %d status query failed: %v", project.Id, err))
		return project, nil, nil, err
	}

	rawResponse := statusResp.RawResponse

	// 更新本地
	updates := map[string]interface{}{
		"status":   statusResp.Status,
		"progress": statusResp.Progress,
	}
	if statusResp.ErrorMsg != "" {
		updates["error_msg"] = statusResp.ErrorMsg
	}
	if statusResp.MainImageUrl != "" {
		updates["main_image_url"] = statusResp.MainImageUrl
	}
	if statusResp.MainImageAssetId != "" {
		updates["main_image_asset_id"] = statusResp.MainImageAssetId
	}
	if statusResp.GeneratedResult != "" {
		updates["generated_result"] = statusResp.GeneratedResult
	}
	if statusResp.FirstVideoUrl != "" {
		updates["first_video_url"] = statusResp.FirstVideoUrl
	}
	// 更新上游积分和金额字段
	if statusResp.CreditAmount > 0 || statusResp.CreditRefund > 0 || statusResp.CreditNet > 0 {
		updates["upstream_credit_amount"] = statusResp.CreditAmount
		updates["upstream_credit_refund"] = statusResp.CreditRefund
		updates["upstream_credit_net"] = statusResp.CreditNet
	}
	if statusResp.MoneyAmount > 0 || statusResp.MoneyRefund > 0 || statusResp.MoneyNet > 0 {
		updates["upstream_money_amount"] = statusResp.MoneyAmount
		updates["upstream_money_refund"] = statusResp.MoneyRefund
		updates["upstream_money_net"] = statusResp.MoneyNet
	}
	isTerminalStatus := statusResp.Status == model.VideoProjectStatusOneClickGenerated ||
		statusResp.Status == model.VideoProjectStatusSuccess ||
		statusResp.Status == model.VideoProjectStatusFailed
	if isTerminalStatus && statusResp.ActualDuration > 0 {
		updates["actual_duration"] = statusResp.ActualDuration
	}

	_ = model.UpdateVideoProjectFields(project.Id, updates)

	project.Status = statusResp.Status
	project.Progress = statusResp.Progress
	if statusResp.ErrorMsg != "" {
		project.ErrorMsg = statusResp.ErrorMsg
	}
	if statusResp.MainImageUrl != "" {
		project.MainImageUrl = statusResp.MainImageUrl
	}
	if statusResp.MainImageAssetId != "" {
		project.MainImageAssetId = statusResp.MainImageAssetId
	}
	if statusResp.GeneratedResult != "" {
		project.GeneratedResult = statusResp.GeneratedResult
	}
	if statusResp.FirstVideoUrl != "" {
		project.FirstVideoUrl = statusResp.FirstVideoUrl
	}
	project.UpstreamCreditAmount = statusResp.CreditAmount
	project.UpstreamCreditRefund = statusResp.CreditRefund
	project.UpstreamCreditNet = statusResp.CreditNet
	project.UpstreamMoneyAmount = statusResp.MoneyAmount
	project.UpstreamMoneyRefund = statusResp.MoneyRefund
	project.UpstreamMoneyNet = statusResp.MoneyNet
	if isTerminalStatus && statusResp.ActualDuration > 0 {
		project.ActualDuration = statusResp.ActualDuration
	}

	// 首次到终态时结算（Settled=0 防止重复结算）
	if project.Settled == 0 {
		switch statusResp.Status {
		case model.VideoProjectStatusOneClickGenerated, model.VideoProjectStatusSuccess, model.VideoProjectStatusFailed:
			// 上游1元=本系统 QuotaPerUnit 积分（1$ = 500,000 quota，1元=1$）
			realQuota := 0
			moneyAmount := statusResp.MoneyAmount
			if moneyAmount == 0 {
				moneyAmount = project.UpstreamMoneyAmount
			}
			moneyNet := statusResp.MoneyNet
			moneyCalc := moneyAmount - statusResp.MoneyRefund // moneyNet 应等于此值
			if moneyNet == 0 && statusResp.MoneyRefund == 0 && moneyAmount > 0 {
				// 上游未返回 moneyNet，直接用计算值
				if statusResp.Status == model.VideoProjectStatusFailed {
					moneyNet = 0
				} else {
					moneyNet = moneyCalc
				}
			} else if math.Abs(moneyNet-moneyCalc) > 0.001 {
				// 上游返回的 moneyNet 与 moneyAmount - moneyRefund 不一致，取较大值（对用户最保守）
				common.SysError(fmt.Sprintf("video project %d moneyNet mismatch: moneyNet=%.2f calc=%.2f, use max",
					project.Id, moneyNet, moneyCalc))
				if moneyCalc > moneyNet {
					moneyNet = moneyCalc
				}
			}

			if moneyNet > 0 {
				realQuota = common.YuanToQuota(moneyNet)
			}
			delta := realQuota - project.PreDeductedQuota // 正=补扣，负=退款，0=无差

			if delta > 0 {
				// 补扣：上游实际消耗 > 预扣
				if err := model.DecreaseUserQuota(project.UserId, delta, false); err != nil {
					common.SysLog(fmt.Sprintf("video project %d extra charge failed: %v", project.Id, err))
					// 补扣失败：记录但不阻断，realQuota 仍按实际值记录
				} else {
					model.RecordLogWithQuota(project.UserId, model.LogTypeConsume, delta, project.VideoModel, project.ChannelId, model.ChannelTypeVideo, project.TokenId, project.TokenName,
						fmt.Sprintf("广告任务 %d 结算补扣积分 %d（预扣 %.2f元，实扣 %.2f元，上游 moneyNet=%.2f）",
							project.Id, delta, common.QuotaToYuan(project.PreDeductedQuota), common.QuotaToYuan(realQuota), moneyNet))
					model.UpdateUserUsedQuota(project.UserId, delta)
					model.UpdateChannelUsedQuota(project.ChannelId, delta)
				}
			} else if delta < 0 {
				// 退款：上游实际消耗 < 预扣
				refundQuota := -delta
				if err := model.IncreaseUserQuota(project.UserId, refundQuota, false); err != nil {
					common.SysLog(fmt.Sprintf("video project %d refund failed: %v", project.Id, err))
				} else {
					model.RecordLogWithQuota(project.UserId, model.LogTypeRefund, -refundQuota, project.VideoModel, project.ChannelId, 2, project.TokenId, project.TokenName,
						fmt.Sprintf("广告任务 %d 结算退还积分 %d（预扣 %.2f元，实扣 %.2f元，上游 moneyNet=%.2f）",
							project.Id, refundQuota, common.QuotaToYuan(project.PreDeductedQuota), common.QuotaToYuan(realQuota), moneyNet))
					model.UpdateUserUsedQuota(project.UserId, -refundQuota)
					model.UpdateChannelUsedQuota(project.ChannelId, -refundQuota)
				}
			} else {
				// 零差额：实际消耗 = 预扣，无需调整积分，仅记录结算日志
				model.RecordLogWithQuota(project.UserId, model.LogTypeConsume, 0, project.VideoModel, project.ChannelId, model.ChannelTypeVideo, project.TokenId, project.TokenName,
					fmt.Sprintf("广告任务 %d 结算完成，预扣与实扣一致（预扣 %.2f元，上游 moneyNet=%.2f）",
						project.Id, common.QuotaToYuan(project.PreDeductedQuota), moneyNet))
			}

			settleUpdates := map[string]interface{}{
				"settled":    1,
				"real_quota": realQuota,
			}
			_ = model.UpdateVideoProjectFields(project.Id, settleUpdates)
			project.Settled = 1
			project.RealQuota = realQuota
			common.SysLog(fmt.Sprintf("video project %d settled: pre=%d delta=%d real=%d upstream_money_net=%.2f",
				project.Id, project.PreDeductedQuota, delta, realQuota, statusResp.MoneyNet))
		}
	}

	return project, rawResponse, nil, nil
}

// ListProjects 获取项目列表
func ListProjects(ctx context.Context, userId int, page, pageSize int, isAdmin bool, statusFilter string) (*dto.VideoProjectListResponse, error) {
	var projects []*model.VideoProject
	var total int64
	var err error

	if isAdmin {
		projects, total, err = model.GetAllVideoProjects((page-1)*pageSize, pageSize, statusFilter)
	} else {
		projects, total, err = model.GetUserVideoProjects(userId, (page-1)*pageSize, pageSize)
	}
	if err != nil {
		return nil, err
	}

	items := make([]dto.VideoProjectItemResponse, len(projects))
	for i, p := range projects {
		status := p.Status
		if status == model.VideoProjectStatusOneClickGenerated || status == model.VideoProjectStatusSuccess {
			status = "SUCCESS"
		}
		items[i] = dto.VideoProjectItemResponse{
			ProjectId:   p.Id,
			ProjectName: p.ProjectName,
			Status:      status,
			Brand:       p.Brand,
			ProductName: p.ProductName,
			CreatedAt:   p.CreatedAt.Unix(),
			UpdatedAt:   p.UpdatedAt.Unix(),
		}
	}

	return &dto.VideoProjectListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// DeleteProject 软删除项目
func DeleteProject(ctx context.Context, projectId int64, userId int, isAdmin bool) error {
	if isAdmin {
		return model.DeleteVideoProjectAdmin(projectId)
	}
	return model.DeleteVideoProject(projectId, userId)
}

// UpdateProjectStatus 管理员更新项目状态
func UpdateProjectStatus(ctx context.Context, projectId int64, req *dto.UpdateVideoProjectStatusRequest) error {
	updates := map[string]interface{}{
		"status": req.Status,
	}
	if req.ErrorMsg != "" {
		updates["error_msg"] = req.ErrorMsg
	}
	if req.MainImageUrl != "" {
		updates["main_image_url"] = req.MainImageUrl
	}
	if req.MainImageAssetId != "" {
		updates["main_image_asset_id"] = req.MainImageAssetId
	}
	if req.GeneratedResult != "" {
		updates["generated_result"] = req.GeneratedResult
	}
	return model.UpdateVideoProjectFields(projectId, updates)
}

// HandleWebhook 处理 webhook 回调
func HandleWebhook(ctx context.Context, channelId int, signature string, body []byte) error {
	ch, err := model.GetVideoChannelById(channelId)
	if err != nil {
		// 渠道不存在：返回 200（防止上游重试），记录日志
		common.SysLog(fmt.Sprintf("webhook: channel %d not found", channelId))
		return nil
	}

	adapter, err := NewAdapterFromChannel(ch)
	if err != nil {
		common.SysLog(fmt.Sprintf("webhook: failed to build adapter for channel %d: %v", channelId, err))
		return nil
	}

	// 验证签名（失败返回 error，调用方返回 401）
	if err := adapter.ValidateWebhook(ctx, signature, body); err != nil {
		return fmt.Errorf("webhook signature validation failed: %w", err)
	}

	payload, err := adapter.ParseWebhookPayload(body)
	if err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// 通过 channel_id + remote_project_id 查找本地项目
	project, err := model.GetVideoProjectByChannelAndRemoteId(channelId, payload.RemoteProjectId)
	if err != nil {
		common.SysLog(fmt.Sprintf("webhook: project not found for channel=%d remote_id=%s", channelId, payload.RemoteProjectId))
		return nil
	}

	updates := map[string]interface{}{
		"status": payload.Status,
	}
	if payload.ErrorMsg != "" {
		updates["error_msg"] = payload.ErrorMsg
	}
	if payload.Progress != "" {
		updates["progress"] = payload.Progress
	}
	if payload.MainImageUrl != "" {
		updates["main_image_url"] = payload.MainImageUrl
	}
	if payload.MainImageAssetId != "" {
		updates["main_image_asset_id"] = payload.MainImageAssetId
	}
	if payload.GeneratedResult != "" {
		updates["generated_result"] = payload.GeneratedResult
	}
	if payload.FirstVideoUrl != "" {
		updates["first_video_url"] = payload.FirstVideoUrl
	}

	if err := model.UpdateVideoProjectFields(project.Id, updates); err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	if payload.Status == model.VideoProjectStatusOneClickGenerated || payload.Status == model.VideoProjectStatusSuccess {
		common.SysLog(fmt.Sprintf("video project %d completed", project.Id))
	} else if payload.Status == model.VideoProjectStatusFailed {
		common.SysLog(fmt.Sprintf("video project %d failed: %s", project.Id, payload.ErrorMsg))
	}

	return nil
}
