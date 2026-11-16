package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// 素材跨渠道同步的哨兵错误：controller 用它映射成对下游的错误码/文案
var (
	// ErrAssetNotFound 本地查不到该上游素材 ID（从未登记或已软删）
	ErrAssetNotFound = errors.New("asset not found")
	// ErrAssetNotSynced 当前渠道没有该素材的绑定，且未开启 asset_auto_sync
	ErrAssetNotSynced = errors.New("asset is not available on current channel")
	// ErrAssetRequiresReverification LivenessFace 素材跨渠道无法自动复用
	ErrAssetRequiresReverification = errors.New("asset requires re-verification on current channel")
	// ErrAssetNotReady 当前渠道的绑定处于 processing / failed 状态
	ErrAssetNotReady = errors.New("asset not ready")
)

// NormalizeAssetRef 归一化素材引用：去掉 asset:// 前缀与空白。
func NormalizeAssetRef(ref string) string {
	trimmed := strings.TrimSpace(ref)
	trimmed = strings.TrimPrefix(trimmed, "asset://")
	return strings.TrimSpace(trimmed)
}

// EnsureAssetForChannel 确保"逻辑素材"在当前渠道有一份可用绑定，返回当前渠道的上游素材 ID。
//
// 解析顺序（惰性补登记，方案 B 保留素材组结构）：
//  1. 按 (user_id, upstream_asset_id) 跨渠道找到逻辑素材；
//  2. 它本身就在当前渠道且 Active → 直接返回；
//  3. 当前渠道已有同 source_url + asset_type 的 Active 绑定 → 直接返回；
//  4. 未命中：渠道开启 asset_auto_sync 时，按 source_url 在当前渠道重新登记
//     （组按「组名 + 组类型」复用/创建，不一律塞默认组）；未开启则返回 ErrAssetNotSynced。
//
// 重复请求天然幂等：命中第 2/3 步就零上游调用；补登记时上游本身也按 URL 去重。
func EnsureAssetForChannel(userID int, ch *model.Channel, adapter AssetAdapter, ref string) (string, error) {
	if ch == nil {
		return "", fmt.Errorf("channel is nil")
	}
	if adapter == nil {
		return "", fmt.Errorf("asset adapter is nil")
	}

	assetID := NormalizeAssetRef(ref)
	if assetID == "" {
		return "", ErrAssetNotFound
	}

	logical, err := model.GetSeedanceAssetByUpstreamIDAnyChannel(assetID, userID)
	if err != nil {
		return "", ErrAssetNotFound
	}

	// 已经是当前渠道的可用绑定
	if logical.ChannelID == ch.Id && strings.EqualFold(strings.TrimSpace(logical.Status), "Active") {
		return logical.UpstreamAssetID, nil
	}

	// 当前渠道是否已有同 source_url 的绑定
	if sourceURL := strings.TrimSpace(logical.SourceURL); sourceURL != "" {
		if existing, findErr := model.GetSeedanceAssetBySourceURLInChannel(sourceURL, userID, ch.Id, logical.AssetType); findErr == nil {
			switch strings.ToLower(strings.TrimSpace(existing.Status)) {
			case "active":
				return existing.UpstreamAssetID, nil
			case "processing":
				return "", fmt.Errorf("%w (status=processing): %s", ErrAssetNotReady, existing.UpstreamAssetID)
			case "failed":
				return "", fmt.Errorf("%w (status=failed): %s", ErrAssetNotReady, existing.UpstreamAssetID)
			}
		}
	}

	// 当前渠道还没有绑定：只有开启自动补登记才继续
	if !ch.GetOtherSettings().AssetAutoSync {
		return "", fmt.Errorf("%w: %s (enable asset_auto_sync to re-register automatically)", ErrAssetNotSynced, assetID)
	}

	sourceURL := strings.TrimSpace(logical.SourceURL)
	if sourceURL == "" {
		return "", fmt.Errorf("%w: source url is empty for asset %s", ErrAssetNotSynced, assetID)
	}

	// 组结构：优先沿用逻辑素材原组的名称与类型
	groupName := ""
	groupType := "AIGC"
	if logical.UpstreamGroupID != "" {
		if group, groupErr := model.GetSeedanceAssetGroupByUpstreamIDAnyChannel(logical.UpstreamGroupID, userID); groupErr == nil {
			groupName = strings.TrimSpace(group.Name)
			if strings.TrimSpace(group.GroupType) != "" {
				groupType = strings.TrimSpace(group.GroupType)
			}
		}
	}
	if strings.EqualFold(groupType, "LivenessFace") {
		return "", fmt.Errorf("%w: %s", ErrAssetRequiresReverification, assetID)
	}
	if groupName == "" {
		groupName = fmt.Sprintf("u%d", userID)
	}

	targetGroupID, err := ensureAssetGroupForChannel(userID, ch, adapter, groupName, groupType)
	if err != nil {
		return "", fmt.Errorf("ensure asset group failed: %w", err)
	}

	created, err := adapter.CreateAsset(CreateAssetRequest{
		GroupID:   targetGroupID,
		URL:       sourceURL,
		AssetType: logical.AssetType,
		Name:      logical.Name,
	})
	if err != nil {
		return "", fmt.Errorf("re-register asset on current channel failed: %w", err)
	}
	if created == nil || strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("re-register asset returned empty id")
	}

	// 注册后必须确认状态，不能"注册完就直接替换 ID 提交"：
	//   - Active：可用，落库并返回；
	//   - Processing：上游还在处理（如异步审核/转码），落库后返回 ErrAssetNotReady，
	//     让下游稍后重试（第二次请求会查到这条绑定，不再重复注册）；
	//   - Failed：落库失败状态并返回 ErrAssetNotReady；
	//   - 状态为空：补查一次 GetAsset 再判断。
	status := strings.ToLower(strings.TrimSpace(created.Status))
	if status == "" {
		if refreshed, getErr := adapter.GetAsset(created.ID, nil); getErr == nil && refreshed != nil {
			created.Status = refreshed.Status
			status = strings.ToLower(strings.TrimSpace(refreshed.Status))
		}
	}

	// 先落库（无论可用与否），保证后续请求能命中这条绑定而不是重复注册
	_ = model.CreateSeedanceAsset(&model.SeedanceAsset{
		UserID:          userID,
		ChannelID:       ch.Id,
		UpstreamAssetID: created.ID,
		UpstreamGroupID: created.GroupID,
		Name:            created.Name,
		AssetType:       created.AssetType,
		SourceURL:       sourceURL,
		Status:          created.Status,
		RawData:         created.RawData,
	})

	switch status {
	case "active":
		return created.ID, nil
	case "processing":
		return "", fmt.Errorf("%w (status=processing): %s", ErrAssetNotReady, created.ID)
	case "failed":
		return "", fmt.Errorf("%w (status=failed): %s", ErrAssetNotReady, created.ID)
	default:
		return "", fmt.Errorf("%w (status=%s): %s", ErrAssetNotReady, status, created.ID)
	}
}

// EnsureAssetGroupForChannel 确保逻辑素材组在当前渠道有绑定，返回当前渠道的上游组 ID。
//
// 方案 B：按「组名 + 组类型」复用当前渠道的同名组，没有则创建同名组。
func EnsureAssetGroupForChannel(userID int, ch *model.Channel, adapter AssetAdapter, upstreamGroupID string) (string, error) {
	if ch == nil || adapter == nil {
		return "", fmt.Errorf("channel or adapter is nil")
	}
	group, err := model.GetSeedanceAssetGroupByUpstreamIDAnyChannel(upstreamGroupID, userID)
	if err != nil {
		return "", ErrAssetNotFound
	}
	if group.ChannelID == ch.Id && strings.TrimSpace(group.UpstreamGroupID) != "" {
		return group.UpstreamGroupID, nil
	}
	if strings.EqualFold(strings.TrimSpace(group.GroupType), "LivenessFace") {
		return "", ErrAssetRequiresReverification
	}
	if !ch.GetOtherSettings().AssetAutoSync {
		return "", fmt.Errorf("%w: %s (enable asset_auto_sync to re-register automatically)", ErrAssetNotSynced, upstreamGroupID)
	}
	return ensureAssetGroupForChannel(userID, ch, adapter, group.Name, group.GroupType)
}

// ensureAssetGroupForChannel 在当前渠道按「组名 + 组类型」复用同名组，没有则创建。
func ensureAssetGroupForChannel(userID int, ch *model.Channel, adapter AssetAdapter, name, groupType string) (string, error) {
	if group, err := model.GetSeedanceAssetGroupByNameAndType(userID, ch.Id, name, groupType); err == nil {
		if strings.TrimSpace(group.UpstreamGroupID) != "" {
			return group.UpstreamGroupID, nil
		}
	}

	created, err := adapter.CreateAssetGroup(CreateAssetGroupRequest{
		Name:      name,
		GroupType: groupType,
	})
	if err != nil {
		return "", err
	}
	if created == nil || strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("create asset group returned empty id")
	}

	_ = model.CreateSeedanceAssetGroup(&model.SeedanceAssetGroup{
		UserID:          userID,
		ChannelID:       ch.Id,
		UpstreamGroupID: created.ID,
		Name:            created.Name,
		Description:     created.Description,
		GroupType:       created.GroupType,
		RawData:         created.RawData,
	})

	return created.ID, nil
}
