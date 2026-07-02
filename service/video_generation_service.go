package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// VideoGenerationService 视频生成服务
type VideoGenerationService struct {
	adapter     VideoGenerationAdapter
	channelType string
}

// NewVideoGenerationService 创建视频生成服务
func NewVideoGenerationService(channelType string) (*VideoGenerationService, error) {
	var adapter VideoGenerationAdapter

	switch channelType {
	case "coze":
		adapter = NewCozeAdapter()
	case "platform":
		adapter = NewPlatformAdapter()
	default:
		return nil, fmt.Errorf("unsupported video generation channel: %s", channelType)
	}

	return &VideoGenerationService{
		adapter:     adapter,
		channelType: channelType,
	}, nil
}

// CreateProject 创建视频项目
func (s *VideoGenerationService) CreateProject(ctx context.Context, userId int, req *dto.CreateVideoProjectRequest) (*model.VideoProject, error) {
	// 获取用户名
	username, err := model.GetUsernameById(userId, false)
	if err != nil {
		username = fmt.Sprintf("user_%d", userId)
	}

	// 生成项目名称
	projectName := fmt.Sprintf("%s_%s_%d", username, time.Now().Format("20060102"), time.Now().Unix())

	// 1. 创建本地记录
	project := &model.VideoProject{
		UserId:        userId,
		Username:      username,
		ProjectName:   projectName,
		ChannelType:   s.channelType,
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
		Duration:      req.Duration,
		Resolution:    req.Resolution,
		VideoModel:    req.VideoModel,
		Whstr:         req.Whstr,
		Status:        model.VideoProjectStatusCreated,
	}

	if err := model.CreateVideoProject(project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// 2. 调用渠道适配器
	resp, err := s.adapter.CreateProject(ctx, req)
	if err != nil {
		// 更新本地状态为失败
		_ = model.UpdateVideoProjectStatus(project.Id, model.VideoProjectStatusFailed, err.Error())
		return nil, fmt.Errorf("failed to call adapter: %w", err)
	}

	// 3. 更新远程项目ID和状态
	project.RemoteProjectId = resp.RemoteProjectId
	project.Status = resp.Status

	updates := map[string]interface{}{
		"remote_project_id": resp.RemoteProjectId,
		"status":            resp.Status,
	}

	if err := model.UpdateVideoProjectFields(project.Id, updates); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	return project, nil
}

// GetProject 获取项目详情
func (s *VideoGenerationService) GetProject(ctx context.Context, projectId int64, userId int, isAdmin bool) (*model.VideoProject, error) {
	var project *model.VideoProject
	var err error

	if isAdmin {
		project, err = model.GetVideoProjectByIdAdmin(projectId)
	} else {
		project, err = model.GetVideoProjectById(projectId, userId)
	}

	if err != nil {
		return nil, err
	}

	// 如果项目处于运行中状态，尝试同步最新状态
	if project.Status == model.VideoProjectStatusCozeRunning || project.Status == model.VideoProjectStatusVideoProcessing {
		if statusResp, err := s.adapter.GetProjectStatus(ctx, project.RemoteProjectId); err == nil {
			// 更新状态
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

			_ = model.UpdateVideoProjectFields(project.Id, updates)

			// 更新本地对象
			project.Status = statusResp.Status
			project.Progress = statusResp.Progress
			project.ErrorMsg = statusResp.ErrorMsg
		}
	}

	return project, nil
}

// ListProjects 获取项目列表
func (s *VideoGenerationService) ListProjects(ctx context.Context, userId int, page, pageSize int, isAdmin bool, statusFilter string) (*dto.VideoProjectListResponse, error) {
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

	// 转换为响应格式
	items := make([]dto.VideoProjectItemResponse, len(projects))
	for i, p := range projects {
		items[i] = dto.VideoProjectItemResponse{
			ProjectId:   p.Id,
			ProjectName: p.ProjectName,
			Status:      p.Status,
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

// DeleteProject 删除项目
func (s *VideoGenerationService) DeleteProject(ctx context.Context, projectId int64, userId int, isAdmin bool) error {
	if isAdmin {
		return model.DeleteVideoProjectAdmin(projectId)
	}
	return model.DeleteVideoProject(projectId, userId)
}

// UpdateProjectStatus 管理员更新项目状态
func (s *VideoGenerationService) UpdateProjectStatus(ctx context.Context, projectId int64, req *dto.UpdateVideoProjectStatusRequest) error {
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
func (s *VideoGenerationService) HandleWebhook(ctx context.Context, signature string, body []byte) error {
	// 1. 验证签名
	if err := s.adapter.ValidateWebhook(ctx, signature, body); err != nil {
		return fmt.Errorf("webhook signature validation failed: %w", err)
	}

	// 2. 解析载荷
	payload, err := s.adapter.ParseWebhookPayload(body)
	if err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// 3. 根据 remote_project_id 查找本地项目
	project, err := model.GetVideoProjectByRemoteId(s.channelType, payload.RemoteProjectId)
	if err != nil {
		return fmt.Errorf("project not found: channel=%s, remote_id=%s, err=%w",
			s.channelType, payload.RemoteProjectId, err)
	}

	// 4. 更新项目状态
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

	// 5. 触发后续逻辑（计费、通知等）
	if payload.Status == model.VideoProjectStatusOneClickGenerated {
		// TODO: 计费逻辑
		// TODO: 用户通知
		common.SysLog(fmt.Sprintf("video project %d completed successfully", project.Id))
	} else if payload.Status == model.VideoProjectStatusFailed {
		common.SysLog(fmt.Sprintf("video project %d failed: %s", project.Id, payload.ErrorMsg))
	}

	return nil
}

// GetDefaultChannelType 获取默认渠道类型
func GetDefaultChannelType() string {
	if common.VideoGenerationChannel != "" {
		return common.VideoGenerationChannel
	}
	return "platform"
}
