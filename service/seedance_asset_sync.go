package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	// assetReadyPollAttempts / assetReadyPollInterval 注册后的短轮询参数：500ms × 5 次
	assetReadyPollAttempts = 5
	assetReadyPollInterval = 500 * time.Millisecond
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
		common.SysLog(fmt.Sprintf("[AssetSync] binding hit: channel_id=%d upstream_asset_id=%s source_url=%s", ch.Id, logical.UpstreamAssetID, logical.SourceURL))
		return logical.UpstreamAssetID, nil
	}

	// 当前渠道是否已有同 source_url 的绑定
	if sourceURL := strings.TrimSpace(logical.SourceURL); sourceURL != "" {
		if existing, findErr := model.GetSeedanceAssetBySourceURLInChannel(sourceURL, userID, ch.Id, logical.AssetType); findErr == nil {
			switch strings.ToLower(strings.TrimSpace(existing.Status)) {
			case "active":
				common.SysLog(fmt.Sprintf("[AssetSync] binding hit by source_url: channel_id=%d upstream_asset_id=%s source_url=%s", ch.Id, existing.UpstreamAssetID, existing.SourceURL))
				return existing.UpstreamAssetID, nil
			case "processing":
				// 上次注册还在处理中：短轮询等它收敛，避免让下游直接失败
				if polled, pollErr := pollAssetStatus(adapter, existing.UpstreamAssetID); pollErr == nil && polled == "active" {
					_ = model.UpdateSeedanceAssetStatus(existing.ID, assetStatusForStorage(polled), "")
					return existing.UpstreamAssetID, nil
				}
				return "", fmt.Errorf("%w (status=processing): %s", ErrAssetNotReady, existing.UpstreamAssetID)
			case "failed":
				return "", fmt.Errorf("%w (status=failed): %s", ErrAssetNotReady, existing.UpstreamAssetID)
			}
		}
	}

	// 换渠道行但上游是同一个平台账号时，旧素材 ID 在上游本来就可直接用：
	// 直接复用并落一条当前渠道的绑定，不重新注册（重新注册会新建素材，
	// 丢掉原素材在平台侧的类型/人像授权等属性，视频生成会被审核拒绝）。
	if reusedID, reused, reuseErr := reuseUpstreamAssetOnChannel(userID, ch, adapter, logical); reuseErr != nil {
		return "", reuseErr
	} else if reused {
		return reusedID, nil
	}

	// 当前渠道还没有绑定：只有开启自动补登记才继续（默认开启，显式 false 才关闭）
	if !ch.GetOtherSettings().AssetAutoSyncEnabled() {
		return "", fmt.Errorf("%w: %s (asset_auto_sync is disabled)", ErrAssetNotSynced, assetID)
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
	//   - Processing / 空：短轮询 500ms × 5 次，收敛到 Active 才返回；
	//   - Failed：落库失败状态并返回 ErrAssetNotReady；
	//   - 轮询后仍未终态：落库当前状态并返回 ErrAssetNotReady，让下游稍后重试
	//     （第二次请求会查到这条绑定，不再重复注册）。
	status := strings.ToLower(strings.TrimSpace(created.Status))
	common.SysLog(fmt.Sprintf("[AssetSync] CreateAsset done: channel_id=%d upstream_asset_id=%s upstream_group_id=%s status=%q source_url=%s",
		ch.Id, created.ID, created.GroupID, status, sourceURL))
	if status != "active" && status != "failed" {
		if polled, pollErr := pollAssetStatus(adapter, created.ID); pollErr == nil && polled != "" {
			status = polled
		}
	}

	// 先落库（无论可用与否），保证后续请求能命中这条绑定而不是重复注册
	if persistErr := model.CreateSeedanceAsset(&model.SeedanceAsset{
		UserID:          userID,
		ChannelID:       ch.Id,
		UpstreamAssetID: created.ID,
		UpstreamGroupID: created.GroupID,
		Name:            created.Name,
		AssetType:       created.AssetType,
		SourceURL:       sourceURL,
		Status:          assetStatusForStorage(status),
		RawData:         created.RawData,
	}); persistErr != nil {
		common.SysError(fmt.Sprintf("[AssetSync] persist asset binding failed: channel_id=%d upstream_asset_id=%s err=%v",
			ch.Id, created.ID, persistErr))
	}

	common.SysLog(fmt.Sprintf("[AssetSync] registered asset on current channel: channel_id=%d upstream_asset_id=%s upstream_group_id=%s status=%s source_url=%s",
		ch.Id, created.ID, created.GroupID, status, sourceURL))

	switch status {
	case "active":
		return created.ID, nil
	case "processing":
		return "", fmt.Errorf("%w (status=processing): %s", ErrAssetNotReady, created.ID)
	case "failed":
		return "", fmt.Errorf("%w (status=failed): %s", ErrAssetNotReady, created.ID)
	default:
		return "", fmt.Errorf("%w (status=unknown): %s", ErrAssetNotReady, created.ID)
	}
}

// reuseUpstreamAssetOnChannel 探测当前渠道的上游账号是否已存在该素材。
//
// 同一平台换渠道行（channel 记录不同、上游账号相同）时，旧素材 ID 在上游
// 本来就能直接用；直接复用可以避免重新注册导致素材属性（素材组类型、人像
// 授权等）丢失。只有上游确实不存在该素材（不同账号）才返回 reused=false，
// 交给后续补登记逻辑。
func reuseUpstreamAssetOnChannel(userID int, ch *model.Channel, adapter AssetAdapter, logical *model.SeedanceAsset) (string, bool, error) {
	if logical == nil || strings.TrimSpace(logical.UpstreamAssetID) == "" {
		return "", false, nil
	}
	existing, err := adapter.GetAsset(logical.UpstreamAssetID, nil)
	if err != nil {
		if isUpstreamAssetNotFound(err) {
			common.SysLog(fmt.Sprintf("[AssetSync] upstream probe: channel_id=%d upstream_asset_id=%s not found, will re-register", ch.Id, logical.UpstreamAssetID))
			return "", false, nil
		}
		return "", false, fmt.Errorf("probe upstream asset failed: %w", err)
	}
	common.SysLog(fmt.Sprintf("[AssetSync] upstream probe: channel_id=%d upstream_asset_id=%s status=%q", ch.Id, logical.UpstreamAssetID, existing.Status))
	if existing == nil || strings.TrimSpace(existing.ID) == "" {
		return "", false, nil
	}

	status := strings.ToLower(strings.TrimSpace(existing.Status))
	if status == "processing" || status == "" {
		if polled, pollErr := pollAssetStatus(adapter, existing.ID); pollErr == nil && polled != "" {
			status = polled
		}
	}
	if status == "failed" {
		return "", false, fmt.Errorf("%w (status=failed): %s", ErrAssetNotReady, existing.ID)
	}
	if status != "active" {
		return "", false, fmt.Errorf("%w (status=%s): %s", ErrAssetNotReady, status, existing.ID)
	}

	// 落当前渠道绑定，后续请求直接命中，不再探测上游
	if persistErr := model.CreateSeedanceAsset(&model.SeedanceAsset{
		UserID:          userID,
		ChannelID:       ch.Id,
		UpstreamAssetID: existing.ID,
		UpstreamGroupID: existing.GroupID,
		Name:            existing.Name,
		AssetType:       existing.AssetType,
		SourceURL:       strings.TrimSpace(logical.SourceURL),
		Status:          assetStatusForStorage(existing.Status),
		RawData:         existing.RawData,
	}); persistErr != nil {
		common.SysError(fmt.Sprintf("[AssetSync] persist reused asset binding failed: channel_id=%d upstream_asset_id=%s err=%v",
			ch.Id, existing.ID, persistErr))
	}
	common.SysLog(fmt.Sprintf("[AssetSync] reused upstream asset on current channel: channel_id=%d upstream_asset_id=%s", ch.Id, existing.ID))
	return existing.ID, true, nil
}

// isUpstreamAssetNotFound 判断上游错误是否为"素材不存在"。
// 用于区分"不同账号需要重新登记"与"鉴权/网络错误"：后者不能盲目补登记。
func isUpstreamAssetNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, keyword := range []string{"404", "not found", "notfound", "not exist", "does not exist", "no such asset", "resource_not_found", "resourcenotfound"} {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
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
	if !ch.GetOtherSettings().AssetAutoSyncEnabled() {
		return "", fmt.Errorf("%w: %s (asset_auto_sync is disabled)", ErrAssetNotSynced, upstreamGroupID)
	}
	return ensureAssetGroupForChannel(userID, ch, adapter, group.Name, group.GroupType)
}

// pollAssetStatus 注册后的短轮询：每 500ms 查一次 GetAsset，最多 5 次。
//
// 返回最后一次拿到的状态（小写）。上游查询报错时继续轮询，全部失败且没有状态时
// 返回最后一个错误，由调用方按"未就绪"处理。
func pollAssetStatus(adapter AssetAdapter, assetID string) (string, error) {
	status := ""
	var lastErr error
	for attempt := 0; attempt < assetReadyPollAttempts; attempt++ {
		time.Sleep(assetReadyPollInterval)
		asset, err := adapter.GetAsset(assetID, nil)
		if err != nil {
			lastErr = err
			common.SysLog(fmt.Sprintf("[AssetSync] poll GetAsset failed: attempt=%d upstream_asset_id=%s err=%v", attempt+1, assetID, err))
			continue
		}
		status = strings.ToLower(strings.TrimSpace(asset.Status))
		common.SysLog(fmt.Sprintf("[AssetSync] poll GetAsset: attempt=%d upstream_asset_id=%s status=%q", attempt+1, assetID, status))
		if status == "active" || status == "failed" {
			return status, nil
		}
	}
	return status, lastErr
}

// assetStatusForStorage 统一库内状态大小写（Active / Processing / Failed / Archived）。
func assetStatusForStorage(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "Active"
	case "processing":
		return "Processing"
	case "failed":
		return "Failed"
	case "archived":
		return "Archived"
	default:
		return strings.TrimSpace(status)
	}
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

	if persistErr := model.CreateSeedanceAssetGroup(&model.SeedanceAssetGroup{
		UserID:          userID,
		ChannelID:       ch.Id,
		UpstreamGroupID: created.ID,
		Name:            created.Name,
		Description:     created.Description,
		GroupType:       created.GroupType,
		RawData:         created.RawData,
	}); persistErr != nil {
		common.SysError(fmt.Sprintf("[AssetSync] persist asset group failed: channel_id=%d upstream_group_id=%s err=%v",
			ch.Id, created.ID, persistErr))
	}

	return created.ID, nil
}
