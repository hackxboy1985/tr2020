// Package sleeppool 实现「过载休眠池」的 Redis 存储。
//
// 存储结构（仅使用 Redis，不新增数据库表）：
//
//	overload_sleep_pool:model:<模型名>:channel:<渠道ID>  ->  STRING
//	    value: JSON(Entry)，其中 trigger_status_code 记录「因哪个错误码进入休眠」
//	    TTL:   休眠时长 + 宽限
//
// 为什么用「一个 (模型, 渠道) 一个 key」而不是按模型做 HASH：
//   - Redis 的 TTL 只能作用于整个 key，无法给 HASH 的单个 field 设 TTL；
//   - 若按模型共用一个 HASH，任何一个渠道刷新 key 的 TTL 都会影响同模型下其它渠道的
//     到期时间——把休眠时长调小后，新进入的渠道会把整个 key 的 TTL 缩短，
//     导致仍在休眠期的老渠道被提前恢复（这是真实 bug，已有回归测试覆盖）；
//   - 每个 (模型, 渠道) 独立 key 后，TTL 由 Redis 原生保证相互独立。
//
// 热路径代价控制：
//   - 请求时不去 SCAN 全库，而是先拿到该 (分组, 模型) 的候选渠道（来自内存缓存，无 IO），
//     再用一次 MGET 精确查询这些候选渠道的 key，因此固定 1 次 Redis 往返。
//   - SCAN 只用在管理端「列出全部/按模型列出」这类低频路径上。
package sleeppool

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	// KeyPrefix 休眠池 key 前缀（后面拼「模型名 + channel + 渠道ID」）
	KeyPrefix = "overload_sleep_pool:model:"
	// channelKeyInfix 模型名与渠道 ID 之间的分隔符
	channelKeyInfix = ":channel:"
	// allKeysPattern 用于 SCAN 全部休眠池 key
	allKeysPattern = "overload_sleep_pool:*"
	// maxScanCount SCAN 单次数量
	maxScanCount = 500
	// maxScanRounds 防止无限 SCAN（异常情况下保护）
	maxScanRounds = 200
	// keyTTLGrace key 过期时间相对休眠时长的宽限（吸收多节点时钟偏差），
	// 真正是否休眠以条目自身的 sleep_until 判定，多留一会儿不会造成误跳过。
	keyTTLGrace = 2 * time.Minute
)

// ErrRedisUnavailable Redis 未启用或不可用
var ErrRedisUnavailable = errors.New("Redis 未启用，过载休眠池不可用")

// nowFunc 返回当前秒级时间戳。
// 抽成变量是为了让单元测试可以注入假时钟（条目到期基于 sleep_until 判断，
// 而 miniredis 的 FastForward 只影响 Redis TTL，不影响进程时钟）。
var nowFunc = common.GetTimestamp

// Entry 单条休眠记录：模型 + 渠道
type Entry struct {
	ModelName   string `json:"model_name"`
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
	Group       string `json:"group"`
	SleepUntil  int64  `json:"sleep_until"`
	// TriggerStatusCode 因哪个上游状态码进入休眠（如 429 / 503）
	TriggerStatusCode int `json:"trigger_status_code"`
	// TriggerCount 同一 (模型, 渠道) 连续进入休眠的次数
	TriggerCount int   `json:"trigger_count"`
	CreatedAt    int64 `json:"created_at"`
	UpdatedAt    int64 `json:"updated_at"`
}

// RemainingSeconds 剩余休眠秒数（<=0 表示已到期）
func (e *Entry) RemainingSeconds(now int64) int64 {
	remaining := e.SleepUntil - now
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired 是否已到期
func (e *Entry) IsExpired(now int64) bool {
	return e.SleepUntil <= now
}

// Enabled 休眠池功能是否可用（Redis 已启用且总开关打开）
func Enabled() bool {
	return common.RedisEnabled && common.RDB != nil && operation_setting.IsOverloadSleepEnabled()
}

// Available Redis 是否可用（不考虑总开关，供管理接口展示）
func Available() bool {
	return common.RedisEnabled && common.RDB != nil
}

// ModelKey 返回 (模型, 渠道) 对应的 Redis key。
func ModelKey(modelName string, channelId int) string {
	return KeyPrefix + modelName + channelKeyInfix + strconv.Itoa(channelId)
}

// FormatKey 兼容旧调用：返回模型维度前缀（不带渠道 ID）。
func FormatKey(modelName string) string {
	return KeyPrefix + modelName + channelKeyInfix
}

// modelKeyPattern 返回用于 SCAN 指定模型全部休眠 key 的 glob 模式。
func modelKeyPattern(modelName string) string {
	return escapeGlob(KeyPrefix+modelName+channelKeyInfix) + "*"
}

// escapeGlob 转义 Redis glob 元字符，避免模型名里含 * ? [ ] \\ 时把匹配搞乱。
func escapeGlob(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '*', '?', '[', ']', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// parseKey 从 key 反解出 (模型名, 渠道ID)。
func parseKey(key string) (modelName string, channelId int, ok bool) {
	rest := strings.TrimPrefix(key, KeyPrefix)
	if rest == key {
		return "", 0, false
	}
	idx := strings.LastIndex(rest, channelKeyInfix)
	if idx < 0 {
		return "", 0, false
	}
	channelId, err := strconv.Atoi(rest[idx+len(channelKeyInfix):])
	if err != nil || channelId <= 0 {
		return "", 0, false
	}
	modelName = rest[:idx]
	if modelName == "" {
		return "", 0, false
	}
	return modelName, channelId, true
}

// Mark 将「模型 + 渠道」放入休眠池（或续期）。
// sleepMinutes <= 0 时使用配置中的休眠分钟数。
func Mark(entry Entry, sleepMinutes int) error {
	if !Available() {
		return ErrRedisUnavailable
	}
	if entry.ModelName == "" || entry.ChannelId <= 0 {
		return errors.New("invalid sleep pool entry: model_name or channel_id is empty")
	}
	if sleepMinutes <= 0 {
		sleepMinutes = operation_setting.GetOverloadSleepMinutes()
	}
	now := nowFunc()
	sleepSeconds := int64(sleepMinutes) * 60
	// 始终按「从现在起再睡 N 分钟」续期
	entry.SleepUntil = now + sleepSeconds
	entry.UpdatedAt = now
	if entry.CreatedAt == 0 {
		entry.CreatedAt = now
	}

	// 触发次数：读取旧条目累加，便于排查「同一渠道反复过载」
	if existing, err := getEntry(entry.ModelName, entry.ChannelId); err == nil && existing != nil {
		entry.TriggerCount = existing.TriggerCount
		if entry.CreatedAt == 0 {
			entry.CreatedAt = existing.CreatedAt
		}
		if entry.ChannelName == "" {
			entry.ChannelName = existing.ChannelName
		}
		if entry.ChannelType == 0 {
			entry.ChannelType = existing.ChannelType
		}
		if entry.Group == "" {
			entry.Group = existing.Group
		}
	}
	entry.TriggerCount++

	data, err := common.Marshal(entry)
	if err != nil {
		return err
	}
	// TTL 只作用于这一个 (模型, 渠道)，不会影响同模型下其它渠道
	ttl := time.Duration(sleepSeconds)*time.Second + keyTTLGrace
	return common.RDB.Set(context.Background(), ModelKey(entry.ModelName, entry.ChannelId), string(data), ttl).Err()
}

func getEntry(modelName string, channelId int) (*Entry, error) {
	if !Available() {
		return nil, ErrRedisUnavailable
	}
	raw, err := common.RDB.Get(context.Background(), ModelKey(modelName, channelId)).Result()
	if err != nil {
		return nil, nil
	}
	var entry Entry
	if unmarshalErr := common.UnmarshalJsonStr(raw, &entry); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	entry.ModelName = modelName
	entry.ChannelId = channelId
	return &entry, nil
}

// IsSleeping 判断 (模型, 渠道) 是否处于休眠中。
// 第二个返回值返回命中的条目（未命中为 nil）。
func IsSleeping(modelName string, channelId int) (bool, *Entry) {
	if !Available() || modelName == "" || channelId <= 0 {
		return false, nil
	}
	entry, err := getEntry(modelName, channelId)
	if err != nil || entry == nil {
		return false, nil
	}
	if entry.IsExpired(nowFunc()) {
		return false, nil
	}
	return true, entry
}

// GetSleepingChannels 返回给定渠道 ID 中「处于休眠」的那些条目。
//
// 这是请求热路径使用的方法：用一次 MGET 精确查询候选渠道，
// 不做 SCAN，因此不会随休眠池规模增长而变慢。
func GetSleepingChannels(modelName string, channelIds []int) map[int]*Entry {
	result := make(map[int]*Entry)
	if !Available() || modelName == "" || len(channelIds) == 0 {
		return result
	}
	keys := make([]string, 0, len(channelIds))
	validIds := make([]int, 0, len(channelIds))
	for _, id := range channelIds {
		if id <= 0 {
			continue
		}
		keys = append(keys, ModelKey(modelName, id))
		validIds = append(validIds, id)
	}
	if len(keys) == 0 {
		return result
	}
	values, err := common.RDB.MGet(context.Background(), keys...).Result()
	if err != nil {
		common.SysError("failed to MGET overload sleep pool for model " + modelName + ": " + err.Error())
		return result
	}
	now := nowFunc()
	for i, value := range values {
		if i >= len(validIds) || value == nil {
			continue
		}
		raw, ok := value.(string)
		if !ok {
			continue
		}
		var entry Entry
		if unmarshalErr := common.UnmarshalJsonStr(raw, &entry); unmarshalErr != nil {
			continue
		}
		entry.ModelName = modelName
		entry.ChannelId = validIds[i]
		if entry.IsExpired(now) {
			continue
		}
		result[entry.ChannelId] = &entry
	}
	return result
}

// SleepingChannelIDs 返回指定模型当前休眠中的渠道（按候选渠道精确查询）。
//
// Deprecated: 该接口需要调用方提供渠道候选；保留它只是为了让旧调用点编译通过，
// 新代码请使用 GetSleepingChannels。
func SleepingChannelIDs(modelName string) map[int]*Entry {
	entries, err := ListModelEntries(modelName)
	if err != nil {
		common.SysError("failed to load overload sleep pool for model " + modelName + ": " + err.Error())
		return map[int]*Entry{}
	}
	return entries
}

// ListModelEntries 列出指定模型当前未到期的休眠条目（按渠道 ID 升序）。
// 管理端使用；通过 SCAN 前缀实现「按模型搜索」。
func ListModelEntries(modelName string) (map[int]*Entry, error) {
	result := make(map[int]*Entry)
	if !Available() || modelName == "" {
		return result, nil
	}
	keys, err := scanKeys(modelKeyPattern(modelName))
	if err != nil {
		return result, err
	}
	entries := mgetEntries(keys, modelName)
	for _, entry := range entries {
		result[entry.ChannelId] = entry
	}
	return result, nil
}

// ListAllEntries 列出全部未到期的休眠条目，按模型名升序、渠道 ID 升序排序。
func ListAllEntries() ([]*Entry, error) {
	all := make([]*Entry, 0)
	if !Available() {
		return all, nil
	}
	keys, err := scanKeys(allKeysPattern)
	if err != nil {
		return all, err
	}
	all = mgetEntries(keys, "")
	sort.Slice(all, func(i, j int) bool {
		if all[i].ModelName == all[j].ModelName {
			return all[i].ChannelId < all[j].ChannelId
		}
		return all[i].ModelName < all[j].ModelName
	})
	return all, nil
}

// scanKeys 用 SCAN 收集匹配的 key（管理端低频路径）。
func scanKeys(pattern string) ([]string, error) {
	ctx := context.Background()
	keys := make([]string, 0, 16)
	var cursor uint64
	rounds := 0
	for {
		batch, next, err := common.RDB.Scan(ctx, cursor, pattern, maxScanCount).Result()
		if err != nil {
			return keys, err
		}
		keys = append(keys, batch...)
		cursor = next
		rounds++
		if cursor == 0 || rounds >= maxScanRounds {
			break
		}
	}
	return keys, nil
}

// mgetEntries 批量读取 key 对应的条目，过滤已到期/非法数据。
// modelName 非空时强制覆盖条目里的模型名字段（用于按模型查询）。
func mgetEntries(keys []string, modelName string) []*Entry {
	if len(keys) == 0 {
		return nil
	}
	values, err := common.RDB.MGet(context.Background(), keys...).Result()
	if err != nil {
		return nil
	}
	now := nowFunc()
	entries := make([]*Entry, 0, len(keys))
	for i, value := range values {
		if i >= len(keys) || value == nil {
			continue
		}
		raw, ok := value.(string)
		if !ok {
			continue
		}
		parsedModel, parsedChannelId, ok := parseKey(keys[i])
		if !ok {
			continue
		}
		var entry Entry
		if unmarshalErr := common.UnmarshalJsonStr(raw, &entry); unmarshalErr != nil {
			continue
		}
		entry.ModelName = parsedModel
		entry.ChannelId = parsedChannelId
		if modelName != "" {
			entry.ModelName = modelName
		}
		if entry.IsExpired(now) {
			continue
		}
		entries = append(entries, &entry)
	}
	return entries
}

// Remove 从休眠池移除指定 (模型, 渠道)。
// 返回被移除的条目数（0 表示原本就不存在）。
func Remove(modelName string, channelId int) (int, error) {
	if !Available() {
		return 0, ErrRedisUnavailable
	}
	if modelName == "" || channelId <= 0 {
		return 0, errors.New("model_name and channel_id are required")
	}
	return int(common.RDB.Del(context.Background(), ModelKey(modelName, channelId)).Val()), nil
}

// ClearModel 清空指定模型的休眠池，返回删除条目数
func ClearModel(modelName string) (int, error) {
	if !Available() {
		return 0, ErrRedisUnavailable
	}
	if modelName == "" {
		return 0, errors.New("model_name is required")
	}
	keys, err := scanKeys(modelKeyPattern(modelName))
	if err != nil {
		return 0, err
	}
	return delKeys(keys), nil
}

// ClearAll 清空全部休眠池，返回删除的条目数
func ClearAll() (int, error) {
	if !Available() {
		return 0, ErrRedisUnavailable
	}
	keys, err := scanKeys(allKeysPattern)
	if err != nil {
		return 0, err
	}
	return delKeys(keys), nil
}

func delKeys(keys []string) int {
	if len(keys) == 0 {
		return 0
	}
	ctx := context.Background()
	deleted := 0
	for _, key := range keys {
		deleted += int(common.RDB.Del(ctx, key).Val())
	}
	return deleted
}
