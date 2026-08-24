# 渠道健康监控系统设计文档

## 一、背景与问题

### 1.1 当前问题

在实际使用中发现，某些渠道会出现以下质量问题：
- **高失败率**：频繁出现 `client_gone` 错误，导致客户端超时断开
- **无数据传输**：0 t/s，虽然建立了连接但没有流式数据
- **响应缓慢**：t/s 很低（< 5 t/s），用户体验差
- **Channel Affinity 锁死问题**：如果 Channel Affinity 绑定到有问题的渠道，且设置了 `SkipRetryOnFailure: true`，会导致后续请求一直失败

### 1.2 现有机制的不足

**现有自动禁用机制：**
- 触发条件：单次请求出现特定错误码（401、403等）或关键词（invalid_key、quota_exceeded等）
- 局限性：**只能处理明确的错误**，无法处理质量问题（慢速、无响应、频繁超时等）

**需要的新机制：**
- 基于**多次请求的统计数据**判断渠道健康度
- 自动禁用持续出现质量问题的渠道
- 支持配置化的阈值和规则

## 二、设计目标

### 2.1 核心目标

1. **自动识别问题渠道**：通过统计分析识别质量差的渠道
2. **及时禁用**：达到阈值后自动禁用，避免影响用户体验
3. **可配置**：支持灵活配置监控规则和阈值
4. **渠道级控制**：每个渠道可独立配置是否启用健康监控

### 2.2 非目标（后续版本）

- 自动恢复机制（禁用后自动重试）
- 实时仪表盘展示
- 预警通知（健康度下降但未达到禁用阈值）

## 三、技术方案

### 3.1 核心指标

| 指标 | 定义 | 计算方式 | 阈值建议 |
|------|------|----------|----------|
| **client_gone 占比** | 最近 N 次请求中出现 `client_gone` 的比例 | `client_gone 次数 / 窗口大小` | ≥ 50% |
| **连续失败次数** | 连续出现 `client_gone` 的次数 | 累计连续失败，成功后清零 | ≥ 3 次 |
| **极低速占比** | 最近 N 次请求中速度极低的比例 | `(0 t/s + 低速) 次数 / 窗口大小` | ≥ 30% |
| **平均速度** | 窗口内的平均 t/s | `总 tokens / 总流式时间` | < 5 t/s |

**说明：**
- `client_gone` 表示 `stream_status.end_reason == "client_gone"`
- **极低速**：包括完全无数据（0 t/s）和低速传输（< 2 t/s）
- **平均速度**：排除 FRT（首字节时间），只计算实际流式传输阶段的速度
- t/s 计算公式：`TotalTokens / (Duration - FRT)`

### 3.2 数据结构

#### 3.2.1 单次请求结果

```go
// relay/common/channel_health.go
type RequestResult struct {
    Timestamp       time.Time        // 请求时间
    EndReason       StreamEndReason  // 结束原因（done, client_gone, timeout等）
    TokensPerSecond float64          // 平均速度 (t/s)，排除 FRT
    Duration        float64          // 总耗时（秒）
    FRT             float64          // 首字节响应时间（秒）
    TotalTokens     int              // 总 token 数
    Success         bool             // 是否成功
    IsLowSpeed      bool             // 是否低速（< 2 t/s）
}

// t/s 计算公式
func calculateTokensPerSecond(totalTokens int, duration float64, frt float64) float64 {
    if totalTokens == 0 {
        return 0
    }
    // 使用首字节后的时间计算（排除连接建立和等待首字节的时间）
    streamingDuration := duration - frt
    if streamingDuration <= 0.1 {  // 防止除零，最小 0.1 秒
        streamingDuration = 0.1
    }
    return float64(totalTokens) / streamingDuration
}
```

#### 3.2.2 渠道健康状态

```go
type ChannelHealth struct {
    mu sync.RWMutex
    
    // 基本信息
    ChannelID   int
    ChannelName string
    
    // 滑动窗口（存储最近 N 次请求结果）
    RecentResults []RequestResult
    WindowSize    int           // 请求次数窗口（默认 10）
    TimeWindow    time.Duration // 时间窗口（默认 30 分钟）
    ResultExpiry  time.Duration // 结果过期时间（默认 1 小时）
    
    // 统计数据
    TotalRequests    int       // 总请求数
    ClientGoneCount  int       // client_gone 总次数
    LowSpeedCount    int       // 低速/无数据传输总次数（< 2 t/s）
    ConsecutiveFails int       // 当前连续失败次数
    
    // 时间戳
    LastRequestTime time.Time  // 最后一次请求时间
    LastFailTime    time.Time  // 最后一次失败时间
    
    // 健康状态
    IsHealthy     bool
    DisableReason string
}

// 清理过期结果
func (h *ChannelHealth) cleanExpiredResults() {
    cutoff := time.Now().Add(-h.ResultExpiry)
    validResults := []RequestResult{}
    for _, r := range h.RecentResults {
        if r.Timestamp.After(cutoff) {
            validResults = append(validResults, r)
        }
    }
    h.RecentResults = validResults
}
```

#### 3.2.3 全局监控器

```go
type ChannelHealthMonitor struct {
    mu       sync.RWMutex
    channels map[int]*ChannelHealth  // key: channelID
    settings ChannelHealthSetting
}

var globalChannelHealthMonitor *ChannelHealthMonitor

// 定期清理不活跃渠道（避免内存泄漏）
func (m *ChannelHealthMonitor) startCleanupRoutine() {
    ticker := time.NewTicker(1 * time.Hour)
    go func() {
        for range ticker.C {
            m.cleanInactiveChannels()
        }
    }()
}

func (m *ChannelHealthMonitor) cleanInactiveChannels() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    inactiveThreshold := 24 * time.Hour
    now := time.Now()
    
    for id, ch := range m.channels {
        ch.mu.RLock()
        lastRequest := ch.LastRequestTime
        ch.mu.RUnlock()
        
        if now.Sub(lastRequest) > inactiveThreshold {
            delete(m.channels, id)
        }
    }
}
```

### 3.3 核心逻辑

#### 3.3.1 记录请求结果

```go
func (m *ChannelHealthMonitor) RecordRequest(channelID int, channelName string, result RequestResult) {
    // 1. 获取或创建渠道健康状态
    // 2. 添加结果到滑动窗口
    // 3. 更新统计数据
    // 4. 检查是否需要禁用
}
```

#### 3.3.2 健康检查规则

```go
func (h *ChannelHealth) checkHealth() bool {
    // 先清理过期结果
    h.cleanExpiredResults()
    
    // 规则1: 样本数量不足时的特殊处理
    if len(h.RecentResults) < minRequests {
        // 样本少但连续失败，也应该禁用（防止新渠道连续失败被忽略）
        if len(h.RecentResults) >= 3 && h.ConsecutiveFails >= 3 {
            h.disable(fmt.Sprintf("新渠道连续%d次请求失败", h.ConsecutiveFails))
            return false
        }
        // 样本不足，暂不判断
        return true
    }
    
    // 规则2: client_gone 占比 >= 50%
    clientGoneCount := 0
    lowSpeedCount := 0
    totalSpeed := 0.0
    
    for _, r := range h.RecentResults {
        if r.EndReason == StreamEndReasonClientGone {
            clientGoneCount++
        }
        if r.TokensPerSecond < 2.0 || r.IsLowSpeed {
            lowSpeedCount++
        }
        totalSpeed += r.TokensPerSecond
    }
    
    failRate := float64(clientGoneCount) / float64(len(h.RecentResults))
    if failRate >= 0.5 {
        h.disable(fmt.Sprintf("最近%d次请求中%d次client_gone (%.1f%%)", 
            len(h.RecentResults), clientGoneCount, failRate*100))
        return false
    }
    
    // 规则3: 连续失败 >= 3 次
    if h.ConsecutiveFails >= 3 {
        h.disable(fmt.Sprintf("连续%d次请求失败", h.ConsecutiveFails))
        return false
    }
    
    // 规则4: 低速/无数据占比 >= 30%
    lowSpeedRate := float64(lowSpeedCount) / float64(len(h.RecentResults))
    if lowSpeedRate >= 0.3 {
        h.disable(fmt.Sprintf("最近%d次请求中%d次低速传输 (%.1f%%, < 2 t/s)", 
            len(h.RecentResults), lowSpeedCount, lowSpeedRate*100))
        return false
    }
    
    // 规则5: 平均速度 < 5 t/s
    avgSpeed := totalSpeed / float64(len(h.RecentResults))
    if avgSpeed < 5.0 {
        h.disable(fmt.Sprintf("平均速度过低: %.2f t/s", avgSpeed))
        return false
    }
    
    return true
}
```

#### 3.3.3 禁用渠道

```go
func (h *ChannelHealth) disable(reason string) {
    // 防止重复禁用
    if !h.IsHealthy {
        return
    }
    
    h.IsHealthy = false
    h.DisableReason = reason
    
    // 异步禁用，避免阻塞其他请求
    go func() {
        channelError := types.ChannelError{
            ChannelId:   h.ChannelID,
            ChannelName: h.ChannelName,
            AutoBan:     true,  // 需要检查渠道是否启用了健康监控
        }
        service.DisableChannel(channelError, reason)
    }()
}
```

### 3.4 集成点

#### 3.4.1 请求结束时记录

在 `relay/channel/openai/relay_responses.go` 中：

```go
func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
    startTime := time.Now()
    var frtTime time.Time
    
    // ... 现有逻辑
    
    defer func() {
        // 计算 t/s（排除 FRT）
        tokensPerSecond := 0.0
        if usage != nil && usage.TotalTokens > 0 {
            if frtTime.IsZero() {
                frtTime = startTime  // 如果没有记录 FRT，使用 startTime
            }
            streamingDuration := time.Since(frtTime).Seconds()
            if streamingDuration > 0.1 {  // 最小 0.1 秒，防止除零
                tokensPerSecond = float64(usage.TotalTokens) / streamingDuration
            }
        }
        
        // 记录到健康监控
        result := relaycommon.RequestResult{
            Timestamp:       time.Now(),
            EndReason:       info.StreamStatus.EndReason,
            TokensPerSecond: tokensPerSecond,
            Duration:        time.Since(startTime).Seconds(),
            FRT:             frtTime.Sub(startTime).Seconds(),
            TotalTokens:     usage.TotalTokens,
            Success:         info.StreamStatus.IsNormalEnd(),
            IsLowSpeed:      tokensPerSecond < 2.0,
        }
        
        monitor := relaycommon.GetChannelHealthMonitor()
        if monitor.IsEnabled() && monitor.IsChannelEnabled(info.ChannelId) {
            monitor.RecordRequest(info.ChannelId, info.ChannelName, result)
        }
    }()
    
    // 在首次收到数据时记录 FRT
    helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
        if frtTime.IsZero() && info.ReceivedResponseCount == 0 {
            frtTime = time.Now()
        }
        // ... 原有逻辑
    })
}
```

同样在非流式的 `OaiResponsesHandler` 中也要添加。

**注意：**
- 需要在首次收到数据时记录 `frtTime`
- t/s 计算只用流式传输阶段的时间（排除 FRT）
- 对于非流式请求，FRT 可能等于总时长

### 3.5 配置管理

#### 3.5.1 全局配置（在 operation_setting 中）

```go
type ChannelHealthSetting struct {
    Enabled             bool          `json:"enabled"`                // 全局开关（默认 false）
    WindowSize          int           `json:"window_size"`            // 滑动窗口大小（默认 10）
    TimeWindow          time.Duration `json:"time_window"`            // 时间窗口（默认 30 分钟）
    ResultExpiry        time.Duration `json:"result_expiry"`          // 结果过期时间（默认 1 小时）
    MinRequests         int           `json:"min_requests"`           // 最小样本数（默认 5）
    FailRateThreshold   float64       `json:"fail_rate_threshold"`    // client_gone 占比阈值（默认 0.5）
    ConsecutiveFailures int           `json:"consecutive_failures"`   // 连续失败次数阈值（默认 3）
    LowSpeedThreshold   float64       `json:"low_speed_threshold"`    // 低速阈值 t/s（默认 2.0）
    LowSpeedRatio       float64       `json:"low_speed_ratio"`        // 低速占比阈值（默认 0.3）
    AvgSpeedThreshold   float64       `json:"avg_speed_threshold"`    // 平均速度阈值 t/s（默认 5.0）
}
```

**配置说明：**
- `TimeWindow`：只统计这个时间范围内的请求（如 30 分钟）
- `ResultExpiry`：超过这个时间的结果会被清理（如 1 小时）
- `LowSpeedThreshold`：低于这个速度视为"低速"（如 2 t/s）
- `LowSpeedRatio`：低速请求占比超过此值触发禁用（如 30%）
- `AvgSpeedThreshold`：窗口内平均速度低于此值触发禁用（如 5 t/s）

#### 3.5.2 渠道级配置（在 channels 表中）

在 `channels` 表的 `other` 字段中添加：

```json
{
  "enable_health_monitor": true  // 默认 false，需要显式启用
}
```

或者新增一个字段 `enable_health_monitor BOOLEAN DEFAULT 0`。

**渠道级开关优先级：**
1. 如果全局 `Enabled = false` → 所有渠道都不启用
2. 如果全局 `Enabled = true` → 只有设置了 `enable_health_monitor = true` 的渠道才启用

### 3.6 数据库变更

#### 3.6.1 Option 1: 使用现有的 other 字段

不需要数据库迁移，直接在 `other` JSON 字段中存储：

```go
type ChannelOtherSettings struct {
    // ... 现有字段
    EnableHealthMonitor bool `json:"enable_health_monitor,omitempty"`
}
```

#### 3.6.2 Option 2: 新增字段（推荐）

```sql
-- 迁移脚本
ALTER TABLE channels ADD COLUMN enable_health_monitor INTEGER DEFAULT 0;
```

优点：
- 查询性能更好
- 更明确的语义
- 便于在列表中显示

## 四、实现计划

### Phase 1：基础功能（1-2天）

**必须修复的问题：**
1. ✅ 增加低速阈值指标（< 2 t/s）和低速占比
2. ✅ 增加平均速度监控（< 5 t/s）
3. ✅ 增加时间窗口（30分钟）和结果过期机制（1小时）
4. ✅ 修复样本不足时的判断逻辑（连续3次失败仍触发）
5. ✅ 明确 t/s 计算公式（排除 FRT）
6. ✅ 异步禁用操作（避免阻塞）
7. ✅ 内存管理（定期清理不活跃渠道）

**实现范围限定（简化）：**
- **只在 3 个渠道类型中实现**：
  1. **OpenAI 渠道**：`constant.ChannelTypeOpenAI`
  2. **Codex 渠道**：`constant.ChannelTypeCodex`（Claude Code 类型）
  3. **Gemini 渠道**：`constant.ChannelTypeGemini`
- 其他渠道类型在 Phase 2 或后续根据需要添加

**实现任务：**
1. **数据结构和监控器**
   - 创建 `relay/common/channel_health.go`
   - 实现 `RequestResult`、`ChannelHealth`、`ChannelHealthMonitor`
   - 实现滑动窗口、时间窗口和统计逻辑
   - 实现定期清理机制

2. **集成到现有代码**
   - 在 `relay/channel/openai/relay_responses.go` 中添加记录逻辑（OpenAI 渠道）
   - 在 `relay/channel/codex/adaptor.go` 的 `DoResponse` 中添加记录逻辑（Codex 渠道）
   - 在 `relay/channel/gemini/` 相关文件中添加记录逻辑（Gemini 渠道）
   - 正确计算 `TokensPerSecond`（排除 FRT）
   - 记录 FRT 时间

3. **配置管理**
   - 添加 `ChannelHealthSetting` 到 `operation_setting`
   - 添加渠道级开关（使用 `other` 字段：`enable_health_monitor`）
   - 实现 `IsEnabled()` 和 `IsChannelEnabled()` 方法

4. **测试**
   - 单元测试：滑动窗口、统计计算、健康检查规则、时间过期
   - 集成测试：模拟 OpenAI、Codex 和 Gemini 渠道的多次请求触发禁用

### Phase 2：扩展支持和前端（1-2天）

1. **支持更多渠道类型**
   - Azure OpenAI
   - Claude 
   - 其他主要渠道类型

2. **支持更多端点（重要）**
   - `/v1/chat/completions`（OpenAI 标准聊天 API）
   - `/v1/messages`（Claude API）
   - `/v1/embeddings`（如果需要）
   - 其他流式 API

3. **前端支持（重要）**
   - **系统设置页面**：
     - 添加"渠道健康监控"配置区域
     - 配置全局开关、窗口大小、各项阈值
     - 实时预览配置效果
   
   - **渠道管理页面**：
     - 添加"启用健康监控"开关（每个渠道独立配置）
     - 显示渠道健康状态（健康/已禁用/禁用原因）
     - 显示关键指标（失败率、平均速度、最近状态）

4. **Channel Affinity 清理（重要）**
   - 渠道被禁用时清除所有指向该渠道的 Affinity 绑定
   - 避免请求被锁在已禁用的渠道上
   - 实现 `service.ClearChannelAffinity(channelID)` 方法
   
   ```go
   func (h *ChannelHealth) disable(reason string) {
       // ... 现有禁用逻辑
       
       // 清理 Channel Affinity 绑定
       go func() {
           service.ClearChannelAffinity(h.ChannelID)
       }()
   }
   ```

5. **验证和优化**
   - 验证重复禁用检查在并发场景下的正确性
   - 优化内存清理策略（可选）
   - 测试 Channel Affinity 清理的有效性

### Phase 3：监控和可视化（后续版本）

1. **健康度仪表盘**
   - 实时显示各渠道的健康指标
   - 历史趋势图

2. **预警和通知**
   - 健康度下降但未达到禁用阈值时发送预警
   - 渠道被禁用时发送通知

## 五、测试方案

### 5.1 单元测试

```go
// relay/common/channel_health_test.go

func TestChannelHealth_FailRateThreshold(t *testing.T) {
    // 模拟 10 次请求，其中 6 次 client_gone
    // 预期：应该触发禁用
}

func TestChannelHealth_ConsecutiveFailures(t *testing.T) {
    // 模拟连续 3 次 client_gone
    // 预期：应该触发禁用
}

func TestChannelHealth_ZeroSpeedThreshold(t *testing.T) {
    // 模拟 10 次请求，其中 4 次 0 t/s
    // 预期：应该触发禁用
}

func TestChannelHealth_InsufficientSamples(t *testing.T) {
    // 模拟只有 3 次请求（< minRequests）
    // 预期：不应该触发禁用
}
```

### 5.2 集成测试

```go
// 在实际环境中测试
func TestChannelHealthIntegration(t *testing.T) {
    // 1. 创建测试渠道
    // 2. 配置健康监控
    // 3. 发送多次请求（模拟失败）
    // 4. 验证渠道被禁用
    // 5. 清理
}
```

## 六、注意事项

### 6.1 性能考虑

1. **内存使用**：每个渠道存储最近 10 次请求结果，假设有 100 个渠道，每个结果约 100 字节，总内存占用约 100KB，可忽略
2. **并发安全**：使用 `sync.RWMutex` 保护并发访问
3. **异步禁用**：禁用操作使用 goroutine 异步执行，避免阻塞请求响应
4. **定期清理**：每小时清理一次不活跃渠道（24小时无请求），防止内存泄漏

### 6.2 与现有机制的关系

**现有自动禁用：**
- 触发条件：单次错误
- 错误类型：明确的错误（401、invalid_key等）
- 立即生效

**健康监控：**
- 触发条件：多次统计
- 错误类型：质量问题（慢速、超时、无响应）
- 累计达到阈值后生效

**两者互补，不冲突：**
- 现有机制处理**明确错误**，立即禁用
- 健康监控处理**质量问题**，累计判断

### 6.3 边界情况

1. **新渠道**：
   - 样本数 < `MinRequests` 时不进行全面判断
   - 但如果连续 3 次失败，仍会触发禁用（防止有问题的新渠道继续使用）

2. **偶发失败**：只有持续或高频失败才触发，偶发失败不影响

3. **渠道恢复**：被健康监控禁用的渠道需要手动启用（自动恢复在后续版本实现）

4. **时间窗口**：
   - 过期的记录会被自动清理，避免历史数据影响当前判断
   - 低频渠道不会因为旧的失败记录被误判
   - 高频渠道的统计具有时效性

5. **FRT 计算**：
   - 流式请求：在首次收到数据时记录 FRT
   - 非流式请求：FRT 可能等于总时长
   - t/s 计算排除 FRT，只计算实际传输阶段的速度

### 6.5 前后端字段一致性规范（重要）

**为避免前后端字段不一致导致的读写错误，必须遵守以下规范：**

#### 1. **配置字段命名规范**

**后端 Go 结构体：**
```go
type ChannelHealthSetting struct {
    Enabled             bool          `json:"enabled"`
    WindowSize          int           `json:"window_size"`
    TimeWindow          time.Duration `json:"time_window"`           // 前端传 "30m"
    ResultExpiry        time.Duration `json:"result_expiry"`         // 前端传 "1h"
    MinRequests         int           `json:"min_requests"`
    FailRateThreshold   float64       `json:"fail_rate_threshold"`
    ConsecutiveFailures int           `json:"consecutive_failures"`
    LowSpeedThreshold   float64       `json:"low_speed_threshold"`
    LowSpeedRatio       float64       `json:"low_speed_ratio"`
    AvgSpeedThreshold   float64       `json:"avg_speed_threshold"`
}
```

**前端 TypeScript 接口（必须完全对应）：**
```typescript
interface ChannelHealthSetting {
  enabled: boolean;
  window_size: number;
  time_window: string;              // "30m", "1h" 等
  result_expiry: string;            // "30m", "1h" 等
  min_requests: number;
  fail_rate_threshold: number;      // 0.5 表示 50%
  consecutive_failures: number;
  low_speed_threshold: number;      // 2.0 表示 2 t/s
  low_speed_ratio: number;          // 0.3 表示 30%
  avg_speed_threshold: number;      // 5.0 表示 5 t/s
}
```

#### 2. **渠道配置字段**

**后端（在 `other` JSON 字段中）：**
```go
type ChannelOtherSettings struct {
    // ... 现有字段
    EnableHealthMonitor bool `json:"enable_health_monitor,omitempty"`
}
```

**前端（必须使用相同的 key）：**
```typescript
interface ChannelOtherSettings {
  // ... 现有字段
  enable_health_monitor?: boolean;  // 注意：snake_case，不是 camelCase
}
```

#### 3. **字段类型转换规则**

| Go 类型 | JSON 类型 | TypeScript 类型 | 注意事项 |
|---------|----------|----------------|---------|
| `bool` | `boolean` | `boolean` | 直接对应 |
| `int` | `number` | `number` | 直接对应 |
| `float64` | `number` | `number` | 直接对应，注意精度 |
| `time.Duration` | `string` | `string` | Go 序列化为 `"30m"`, `"1h"` 等 |
| `json.RawMessage` | `any` | `any` | 需要二次解析 |

#### 4. **实现检查清单**

**在实现时必须确认：**

- [ ] 后端 Go struct 的 `json` tag 与前端 TypeScript interface 的字段名完全一致
- [ ] 使用 **snake_case**（不是 camelCase），因为 Go 默认序列化不会转换
- [ ] 前端读取配置时，字段名拼写完全正确（复制粘贴，不要手打）
- [ ] 前端提交配置时，字段名拼写完全正确
- [ ] 测试时验证配置保存后能正确读回
- [ ] 注意 `time.Duration` 类型的特殊处理（前端传字符串 `"30m"`）

#### 5. **常见错误示例（禁止）**

❌ **错误示例 1：前端使用 camelCase**
```typescript
// 错误！
interface ChannelHealthSetting {
  windowSize: number;        // 应该是 window_size
  failRateThreshold: number; // 应该是 fail_rate_threshold
}
```

❌ **错误示例 2：拼写错误**
```typescript
// 错误！
const config = {
  fail_rate_threshhold: 0.5  // 拼错了：threshhold -> threshold
}
```

❌ **错误示例 3：类型不匹配**
```typescript
// 错误！
const config = {
  enabled: "true"            // 应该是 boolean，不是 string
}
```

#### 6. **调试方法**

如果出现字段读写问题：
1. 后端打印序列化后的 JSON：`log.Printf("JSON: %s", jsonBytes)`
2. 前端打印接收到的对象：`console.log("config:", config)`
3. 对比字段名是否完全一致（大小写、下划线）
4. 检查类型是否匹配

1. **集成范围**：
   - Phase 1 只集成到 **OpenAI 渠道**、**Codex 渠道** 和 **Gemini 渠道**
   - 只支持 `/v1/responses` API（流式和非流式）
   - Phase 2 会扩展到 `/v1/chat/completions`、`/v1/messages` 等端点
   - 其他渠道类型（Azure、Claude 等）在 Phase 2 添加

2. **配置热更新**：修改配置需要重启服务生效

3. **可观测性**：Phase 1 暂无 Prometheus 指标和详细事件记录

4. **前端界面**：Phase 1 只有后端实现，Phase 2 会添加前端配置界面

## 七、配置示例

### 7.1 全局配置（operation_setting.json）

```json
{
  "channel_health_setting": {
    "enabled": true,
    "window_size": 10,
    "time_window": "30m",
    "result_expiry": "1h",
    "min_requests": 5,
    "fail_rate_threshold": 0.5,
    "consecutive_failures": 3,
    "low_speed_threshold": 2.0,
    "low_speed_ratio": 0.3,
    "avg_speed_threshold": 5.0
  }
}
```

**配置说明：**
- `window_size: 10`：统计最近 10 次请求
- `time_window: "30m"`：只统计 30 分钟内的请求
- `result_expiry: "1h"`：1 小时前的记录会被清理
- `fail_rate_threshold: 0.5`：client_gone 占比 ≥ 50% 触发禁用
- `low_speed_threshold: 2.0`：速度 < 2 t/s 视为低速
- `low_speed_ratio: 0.3`：低速请求占比 ≥ 30% 触发禁用
- `avg_speed_threshold: 5.0`：平均速度 < 5 t/s 触发禁用

### 7.2 渠道配置（channels 表）

```json
{
  "enable_health_monitor": true
}
```

或者：

```sql
UPDATE channels SET enable_health_monitor = 1 WHERE id = 15;
```

## 八、后续优化方向

1. **自动恢复机制**：禁用一段时间后自动重新启用并测试
2. **分级处理**：健康度 < 80% 降低权重，< 50% 发预警，< 30% 禁用
3. **渠道评分系统**：用于负载均衡时优先选择健康渠道
4. **历史数据分析**：长期统计各渠道的稳定性和质量

## 九、总结

该设计方案通过**滑动窗口统计**和**多维度指标**判断渠道健康度，能够有效识别和禁用质量差的渠道，避免影响用户体验。方案支持灵活配置，与现有自动禁用机制互补，可以逐步迭代实现。
