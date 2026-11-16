package doubao

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// assetRefError 携带出错的具体 asset 引用，便于返回给下游明确的 asset id。
type assetRefError struct {
	ref string
	err error
}

func (e *assetRefError) Error() string { return e.err.Error() }
func (e *assetRefError) Unwrap() error { return e.err }

// resolveAssetRefsForChannel 把任务请求里所有 asset:// 引用翻译成当前请求渠道的素材 ID。
//
// 换渠道后下游仍可能传旧渠道的 asset://<id>；这里按 source_url 惰性补登记
// （受渠道配置 asset_auto_sync 控制），失败时返回约定的资产错误结构。
func resolveAssetRefsForChannel(c *gin.Context, info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) *dto.TaskError {
	if c == nil || info == nil || req == nil {
		return nil
	}
	// 先做纯内存判断：请求里没有 asset:// 引用时完全不碰数据库/渠道（也避免测试环境无 DB）
	if !taskRequestHasAssetRef(req) {
		return nil
	}
	ch, err := model.CacheGetChannel(info.ChannelId)
	if err != nil || ch == nil {
		return nil
	}
	adapter, err := service.NewAssetAdapterForChannel(ch)
	if err != nil {
		// 素材能力不可用时不阻塞视频任务，交给上游按原有逻辑报错
		common.SysLog(fmt.Sprintf("[Doubao Video] build asset adapter failed, skip asset ref resolution: %v", err))
		return nil
	}

	resolve := func(raw string) (string, error) {
		resolved, resolveErr := service.EnsureAssetForChannel(info.UserId, ch, adapter, raw)
		if resolveErr != nil {
			return "", &assetRefError{ref: raw, err: resolveErr}
		}
		return "asset://" + resolved, nil
	}

	changed, err := rewriteTaskRequestAssetRefs(req, resolve)
	if err != nil {
		return assetRefTaskError(err)
	}
	if changed {
		// 覆盖 context 中的请求，后续 BuildRequestBody 会取到改写后的版本
		c.Set("task_request", *req)
		common.SysLog("[Doubao Video] asset refs rewritten for current channel")
	}
	return nil
}

// assetRefTaskError 资产解析失败 → 下游约定结构：
//
//	{"code":"InvalidParameter","message":"asset not ready (status=failed): <id>","param":null,"type":"BadRequest"}
func assetRefTaskError(err error) *dto.TaskError {
	var refErr *assetRefError
	ref := ""
	if errors.As(err, &refErr) {
		ref = refErr.ref
	}
	assetID := service.NormalizeAssetRef(ref)
	status := "failed"
	if strings.Contains(err.Error(), "status=processing") {
		status = "processing"
	}
	message := fmt.Sprintf("asset not ready (status=%s): %s", status, assetID)
	switch {
	case errors.Is(err, service.ErrAssetNotFound):
		message = fmt.Sprintf("asset not found: %s", assetID)
	case errors.Is(err, service.ErrAssetRequiresReverification):
		message = fmt.Sprintf("asset requires re-verification (status=failed): %s", assetID)
	case errors.Is(err, service.ErrAssetNotSynced), errors.Is(err, service.ErrAssetNotReady):
		message = fmt.Sprintf("asset not ready (status=%s): %s", status, assetID)
	}

	common.SysError(fmt.Sprintf("[Doubao Video] asset ref resolution failed: asset=%s err=%v", assetID, err))
	return &dto.TaskError{
		Code:       "InvalidParameter",
		Message:    message,
		StatusCode: http.StatusBadRequest,
		ResponseBody: map[string]interface{}{
			"code":    "InvalidParameter",
			"message": message,
			"param":   nil,
			"type":    "BadRequest",
		},
	}
}

// rewriteTaskRequestAssetRefs 遍历任务请求常见位置，把 asset:// 引用替换成 resolver 的结果。
func rewriteTaskRequestAssetRefs(req *relaycommon.TaskSubmitReq, resolve func(string) (string, error)) (bool, error) {
	if req == nil || resolve == nil {
		return false, nil
	}
	changed := false
	var err error
	var c bool

	if req.Image, c, err = rewriteAssetRefString(req.Image, resolve); err != nil {
		return false, err
	}
	changed = changed || c

	for i := range req.Images {
		if req.Images[i], c, err = rewriteAssetRefString(req.Images[i], resolve); err != nil {
			return false, err
		}
		changed = changed || c
	}

	if req.InputReference, c, err = rewriteAssetRefString(req.InputReference, resolve); err != nil {
		return false, err
	}
	changed = changed || c

	for i := range req.Content {
		var item interface{}
		if item, c, err = rewriteAssetRefAny(req.Content[i], resolve); err != nil {
			return false, err
		}
		if c {
			if m, ok := item.(map[string]interface{}); ok {
				req.Content[i] = m
				changed = true
			}
		}
	}

	if req.Metadata != nil {
		var metadata interface{}
		if metadata, c, err = rewriteAssetRefAny(req.Metadata, resolve); err != nil {
			return false, err
		}
		if c {
			if m, ok := metadata.(map[string]interface{}); ok {
				req.Metadata = m
				changed = true
			}
		}
	}

	return changed, nil
}

func isAssetRef(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "asset://")
}

// taskRequestHasAssetRef 纯内存扫描任务请求里是否包含 asset:// 引用。
func taskRequestHasAssetRef(req *relaycommon.TaskSubmitReq) bool {
	if req == nil {
		return false
	}
	if isAssetRef(req.Image) || isAssetRef(req.InputReference) {
		return true
	}
	for _, image := range req.Images {
		if isAssetRef(image) {
			return true
		}
	}
	return valueHasAssetRef(req.Content) || valueHasAssetRef(req.Metadata)
}

func valueHasAssetRef(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		return isAssetRef(typed)
	case []string:
		for _, item := range typed {
			if isAssetRef(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range typed {
			if valueHasAssetRef(item) {
				return true
			}
		}
	case []map[string]interface{}:
		for _, item := range typed {
			if valueHasAssetRef(item) {
				return true
			}
		}
	case map[string]interface{}:
		for _, item := range typed {
			if valueHasAssetRef(item) {
				return true
			}
		}
	}
	return false
}

func rewriteAssetRefString(raw string, resolve func(string) (string, error)) (string, bool, error) {
	if !isAssetRef(raw) {
		return raw, false, nil
	}
	resolved, err := resolve(strings.TrimSpace(raw))
	if err != nil {
		return "", false, err
	}
	return resolved, true, nil
}

func rewriteAssetRefAny(value interface{}, resolve func(string) (string, error)) (interface{}, bool, error) {
	switch typed := value.(type) {
	case string:
		return rewriteAssetRefString(typed, resolve)
	case []string:
		changed := false
		for i := range typed {
			resolved, c, err := rewriteAssetRefString(typed[i], resolve)
			if err != nil {
				return nil, false, err
			}
			if c {
				typed[i] = resolved
				changed = true
			}
		}
		return typed, changed, nil
	case []interface{}:
		changed := false
		for i := range typed {
			item, c, err := rewriteAssetRefAny(typed[i], resolve)
			if err != nil {
				return nil, false, err
			}
			if c {
				typed[i] = item
				changed = true
			}
		}
		return typed, changed, nil
	case []map[string]interface{}:
		changed := false
		for i := range typed {
			item, c, err := rewriteAssetRefAny(typed[i], resolve)
			if err != nil {
				return nil, false, err
			}
			if c {
				if m, ok := item.(map[string]interface{}); ok {
					typed[i] = m
					changed = true
				}
			}
		}
		return typed, changed, nil
	case map[string]interface{}:
		changed := false
		for key, item := range typed {
			resolved, c, err := rewriteAssetRefAny(item, resolve)
			if err != nil {
				return nil, false, err
			}
			if c {
				typed[key] = resolved
				changed = true
			}
		}
		return typed, changed, nil
	default:
		return value, false, nil
	}
}
