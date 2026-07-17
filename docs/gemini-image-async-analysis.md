# Gemini Imagen 异步化改造分析

## 背景

当前 Gemini imagen 生图链路是同步阻塞的：

```
客户端 POST /v1/images/generations
  → 网关阻塞等待
  → Gemini API（30~60秒）
  → 返回结果
```

imagen-4 生图通常需要 30~60 秒，移动端/浏览器默认超时约 30 秒，容易在上游还没返回时客户端已断开连接，导致失败。

---

## 现状分析

### Gemini Imagen API 本身

Gemini Imagen API 是**纯同步接口**，没有官方异步/轮询支持：
- 请求：`POST /v1/{project}/locations/{location}/publishers/google/models/{model}:predict`
- 响应：直接返回 base64 图片数组，无任务 ID、无轮询端点
- 耗时：imagen-4 约 30~60s，imagen-4-ultra 更长

### 网关现有任务框架

系统已有一套异步任务框架（视频生成使用），关键路由：

| 路由 | RelayMode | 状态 |
|------|-----------|------|
| `POST /v1/images/tasks` | `RelayModeImageTaskSubmit` | **常量已定义，handler 未实现** |
| `GET /v1/images/tasks/:task_id` | `RelayModeImageTaskFetchByID` | **已实现**（`imageTaskFetchByIDRespBodyBuilder`） |

`imageTaskFetchByIDRespBodyBuilder`（relay/relay_task.go:569）已支持查询任务状态、返回图片 URL 列表。

### 现有图片生成链路

- 入口：`relay/channel/gemini/adaptor.go`，`ConvertImageRequest()`
- 处理：`relay/channel/gemini/relay-gemini.go`，`GeminiImageHandler()`
- 计费：每张图 258 token，在 `GeminiImageHandler` 同步返回时计算

---

## 异步改造方案

### 核心思路

上游 Gemini 本身不支持异步，所以**网关自己实现任务队列**：

```
客户端 POST /v1/images/tasks
  ↓
网关立即返回 { task_id, status: "pending" }（202 Accepted）
  ↓
后台 goroutine 调用 Gemini（阻塞 30~60s）
  ↓
完成后写入 task 表（状态 + 图片数据）
  ↓
客户端轮询 GET /v1/images/tasks/:task_id
  ↓
状态变为 success 时返回图片
```

### 需要实现的模块

#### 1. `RelayModeImageTaskSubmit` handler（relay/relay_task.go）

```go
// POST /v1/images/tasks
func imageTaskSubmitHandler(c *gin.Context) {
    // 1. 解析请求（复用 ImageRequest）
    // 2. 扣费预检
    // 3. 创建 task 记录，status = "pending"
    // 4. 启动 goroutine 调用上游
    // 5. 返回 202 + { task_id, status: "pending" }
}
```

#### 2. Gemini adaptor 扩展（relay/channel/gemini/adaptor.go）

新增方法，或在现有 `ConvertImageRequest` 基础上增加异步执行路径：

```go
func (a *Adaptor) SubmitImageTask(ctx context.Context, info *RelayInfo, req ImageRequest) (taskID string, err error) {
    // 调用 Gemini，阻塞等待
    // 完成后将 base64 写入 task 表
    // 计算并结算费用
}
```

#### 3. 图片数据存储策略（需要决策）

Gemini 返回的是 base64 编码的图片数据，有两种存储方式：

**方案 A：存 base64 到 task 表**
- 优点：无需额外存储服务
- 缺点：task 表体积膨胀（单张图约 500KB~2MB），数据库压力大
- 适合：低并发、小规模场景

**方案 B：转存对象存储（OSS/S3），task 表存 URL**
- 优点：DB 压力小，URL 可直接返回给客户端
- 缺点：需要配置对象存储
- 适合：生产环境

**方案 C：临时内存 + 签名 URL**（复杂，暂不推荐）

#### 4. 计费时机调整

同步模式下在 handler 返回时计费，异步模式需要在 goroutine 完成后计费：
- 成功：正常扣费（258 token × 张数）
- 失败：退还预扣额度

---

## 与视频任务的对比

| 对比项 | 视频任务（现有） | 图片任务（待实现）|
|--------|-----------------|-----------------|
| 上游支持异步 | ✅ 上游返回 task_id | ❌ 需网关自己管理 |
| 轮询机制 | 调用上游查询接口 | 查本地 task 表 |
| 结果存储 | 上游持有 URL | 需自行存储 base64 或转存 |
| 实现复杂度 | 低 | 中等 |

---

## 风险点

1. **goroutine 泄漏**：网关重启时进行中的任务丢失，需要重启后扫描 pending 任务恢复
2. **并发控制**：并发生图 goroutine 数量需要限制，避免 OOM
3. **base64 存储**：如存 DB，需评估列类型（LONGBLOB/TEXT）和表容量
4. **超时处理**：goroutine 内需设置合理超时（建议 180s），超时后任务标记为 failed

---

## 近期建议

短期若只是解决客户端超时问题，设置环境变量即可：

```yaml
# docker-compose.yml
environment:
  - RELAY_TIMEOUT=180  # 给 imagen 生图留足 3 分钟
```

中期若需要彻底解决，按上述方案实现异步任务队列，优先实现**方案 A（base64 存 DB）**快速验证，再根据实际使用量决定是否迁移到方案 B。
