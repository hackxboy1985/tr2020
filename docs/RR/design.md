# RR 渠道设计文档

## 概述

RR 渠道是一个图像生成异步任务渠道。用户调用标准 `POST /v1/images/generations` 接口，系统在后端将请求转发至 RR 上游，通过异步任务轮询机制获取结果，整个过程对下游用户透明，无新增接口。

---

## 请求流程

```
POST /v1/images/generations  (OpenAI 标准格式)
        ↓
controller.Relay → relayHandler → relay.ImageHelper
        ↓  检测渠道类型 == ChannelTypeRR
        ↓  将 ImageRequest 转换为 TaskSubmitReq，写入 context
        ↓
relay.RelayTaskSubmit (复用现有异步任务框架)
        ↓
  step 1: ValidateRequestAndSetAction
  step 2: 模型名解析 + ModelMappedHelper
  step 3: 生成公开 TaskID
  step 3.5: adaptor.InjectBillingParams  ← 注入转换后的参数（见"通用计费参数注入"章节）
  step 4: ModelPriceHelperPerCall        ← 表达式在此运行，param() 读取注入后的参数
  step 5: EstimateBilling                ← OtherRatios 乘法计费（表达式计费时跳过）
        ↓
RR TaskAdaptor.BuildRequestBody
  — size → aspectRatio + resolution  (查 SizeMap)
  — quality → quality                (查 QualityMap)
  — 模型名 → 路径: {baseURL}/openapi/v2/{model}/text-to-image（或 other 中的覆盖路径）
        ↓
上游返回 { taskId, status: "RUNNING", ... }
        ↓
DoResponse 提取 taskId，框架存入 task.PrivateData.UpstreamTaskID
        ↓  立即返回任务对象给用户（含公开 taskId）
        ↓
后台轮询 (15s 间隔，复用 TaskPollingLoop)
  FetchTask: POST {baseURL}/openapi/v2/query  { taskId }
  ParseTaskResult: 读 results[0].url → taskResult.Url
        ↓
任务完成，ResultURL = results[0].url 存入 task.PrivateData.ResultURL
        ↓
用户查询结果: GET /v1/videos/{publicTaskId}/content
  VideoProxy 检查 expire_at，未过期则从 ResultURL 拉流透明代理返回
  Cache-Control: public, max-age=86400
```

---

## 文件结构

```
constant/channel.go                          — 新增渠道类型常量 ChannelTypeRR = 59
dto/channel_settings.go                      — ChannelOtherSettings 新增 RREndpoints / RRUrlTTLHours
relay/channel/task/rr/
    adaptor.go                               — 实现 TaskAdaptor 接口（含 InjectBillingParams）
    mapping.go                               — size/quality 映射表（可直接编辑）
    models.go                                — 请求/响应 DTO
relay/relay_adaptor.go                       — GetTaskAdaptor switch 追加 ChannelTypeRR
relay/relay_task.go                          — step 3.5 调用 adaptor.InjectBillingParams
relay/channel/adapter.go                     — TaskAdaptor 接口新增 InjectBillingParams
relay/channel/task/taskcommon/helpers.go     — BaseBilling 新增 InjectBillingParams no-op
relay/image_handler.go                       — ImageHelper 入口追加 RR 分叉逻辑
pkg/billingexpr/inject.go                    — 新增 InjectBodyParam 工具函数
service/task_polling.go                      — 轮询完成时写入 expire_at
controller/video_proxy.go                    — VideoProxy 检查 expire_at
web/default/src/features/channels/
    constants.ts                             — 新增 59: 'RR'
    components/rr-path-config-editor.tsx     — RR 渠道路径+TTL编辑器（复用 Poster 模式）
    components/drawers/channel-mutate-drawer.tsx  — 新增 type==59 分支
    lib/channel-type-config.ts               — 新增 RR 渠道配置
```

---

## 通用计费参数注入机制（重要）

### 问题背景

计费表达式（`tiered_expr`）中 `param(path)` 读取的是**下游用户原始请求体**的 JSON 字段。但某些渠道在转发前会对参数进行转换（如 `size "1024x1792"` → `resolution "2k"`），转换结果无法通过 `param()` 读取，导致无法按转换后的上游参数计费。

### 时序根因

```
relay_task.go 当前流程：
  step 2.5: ModelMappedHelper
  step 4:   ModelPriceHelperPerCall  ← 表达式在此运行，info.BillingRequestInput 已冻结
  step 5:   EstimateBilling          ← 太晚，表达式已执行完毕
```

在 `EstimateBilling` 里注入无效，必须在 `step 4` 之前完成注入。

### 解决方案：`InjectBillingParams` 钩子

在 `TaskAdaptor` 接口上新增一个方法，由框架在价格计算前调用：

**接口定义（`relay/channel/adapter.go`）：**

```go
// InjectBillingParams 在计费表达式求值前调用，允许适配器向 BillingRequestInput
// 注入转换后的参数（如 size→resolution）。注入的字段可被表达式 param() 读取。
// 不需要注入时直接返回即可（BaseBilling 已提供 no-op 默认实现）。
InjectBillingParams(c *gin.Context, info *relaycommon.RelayInfo)
```

**默认实现（`relay/channel/task/taskcommon/helpers.go`，嵌入 BaseBilling）：**

```go
func (BaseBilling) InjectBillingParams(_ *gin.Context, _ *relaycommon.RelayInfo) {}
```

现有所有适配器嵌入了 `BaseBilling`，**零改动**，自动获得 no-op 实现。

**框架调用点（`relay/relay_task.go`，step 3.5）：**

```go
// 3.5 允许适配器在价格计算前注入转换后的参数（供计费表达式 param() 使用）
adaptor.InjectBillingParams(c, info)
```

**工具函数（`pkg/billingexpr/inject.go`）：**

```go
// InjectBodyParam 向 JSON body 注入/覆盖一个字段（使用 tidwall/sjson）。
// body 为 nil 或空时从 {} 开始。注入失败时返回原 body，不 panic。
func InjectBodyParam(body []byte, key string, value any) []byte {
    if len(body) == 0 {
        body = []byte("{}")
    }
    result, err := sjson.SetBytes(body, key, value)
    if err != nil {
        return body
    }
    return result
}
```

### 可靠性分析

| 项目 | 结论 |
|------|------|
| 时序正确 | `InjectBillingParams` 在 `ModelPriceHelperPerCall`（含表达式求值）之前调用 ✓ |
| 向后兼容 | `BaseBilling` 提供 no-op，现有适配器零改动 ✓ |
| 非表达式计费 | 注入逻辑只在 `tiered_expr` 模式下生效（`ResolveIncomingBillingExprRequestInput` 只被表达式路径调用），其他计费模式无影响 ✓ |
| 依赖 | `tidwall/sjson v1.2.5` 已在 `go.mod` ✓ |
| 注入隔离 | 只修改 `info.BillingRequestInput.Body`，不影响实际发往上游的请求体 ✓ |
| key 冲突 | 注入的 key（如 `resolution`）若与原始请求字段同名会覆盖。如需避免冲突可用带下划线前缀的 key（如 `_resolution`），表达式配置时对应即可 |

### 扩展到同步渠道（设计已完成，暂不实现）

同步渠道的价格计算发生在 `controller/relay.go:159`（`ModelPriceHelper`），而适配器实例在之后的 `ImageHelper/TextHelper` 内才创建，因此无法用"适配器钩子"的方式在计费前注入。

**可行方案：按渠道类型注册转换函数。**

在 `controller/relay.go` 中，`GenRelayInfo`（line 120）之后、`ModelPriceHelper`（line 159）之前，`relayInfo` 和原始请求都已就绪。在此处调用一个全局注册表查询：

```go
// relay/billing_params_registry.go
// 按渠道类型注册参数转换函数，不依赖适配器实例
var syncBillingParamsRegistry = map[int]func(c *gin.Context, info *relaycommon.RelayInfo){}

func RegisterSyncBillingParams(channelType int, fn func(*gin.Context, *relaycommon.RelayInfo)) {
    syncBillingParamsRegistry[channelType] = fn
}

func ApplySyncBillingParams(c *gin.Context, info *relaycommon.RelayInfo) {
    if fn, ok := syncBillingParamsRegistry[info.ChannelType]; ok {
        fn(c, info)
    }
}
```

在 `controller/relay.go` 插入调用：

```go
// line ~158（ModelPriceHelper 之前）
relay.ApplySyncBillingParams(c, relayInfo)
priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
```

各渠道在 `init()` 中注册自己的转换函数，与适配器解耦。

**当前状态：RR 渠道走 Task 路径，此方案不需要实现。** 待有同步渠道按转换参数计费的需求时再落地。

---

## size / quality 映射表

映射规则集中在 `relay/channel/task/rr/mapping.go`，**直接编辑该文件即可修改规则，无需改其他代码**。

### SizeMap（OpenAI size → RR aspectRatio + resolution）

| OpenAI `size`  | `aspectRatio` | `resolution` | 备注 |
|----------------|---------------|--------------|------|
| `256x256`      | `1:1`         | `1k`         | |
| `512x512`      | `1:1`         | `1k`         | |
| `1024x1024`    | `1:1`         | `1k`         | DALL-E 标准正方形 |
| `2048x2048`    | `1:1`         | `2k`         | |
| `4096x4096`    | `1:1`         | `4k`         | |
| `1280x720`     | `16:9`        | `1k`         | |
| `1792x1024`    | `16:9`        | `2k`         | DALL-E 3 标准横屏 |
| `1920x1080`    | `16:9`        | `2k`         | |
| `3840x2160`    | `16:9`        | `4k`         | 标准 4K 横屏 |
| `4096x2304`    | `16:9`        | `4k`         | |
| `720x1280`     | `9:16`        | `1k`         | |
| `1024x1792`    | `9:16`        | `2k`         | DALL-E 3 标准竖屏 |
| `1080x1920`    | `9:16`        | `2k`         | |
| `2160x3840`    | `9:16`        | `4k`         | 标准 4K 竖屏 |
| `2304x4096`    | `9:16`        | `4k`         | |
| `1024x768`     | `4:3`         | `1k`         | |
| `1360x1024`    | `4:3`         | `2k`         | |
| `2880x2160`    | `4:3`         | `4k`         | |
| `768x1024`     | `3:4`         | `1k`         | |
| `1024x1360`    | `3:4`         | `2k`         | |
| `2160x2880`    | `3:4`         | `4k`         | |
| `2048x1024`    | `2:1`         | `2k`         | |
| `4096x2048`    | `2:1`         | `4k`         | |
| `1024x2048`    | `1:2`         | `2k`         | |
| `2048x4096`    | `1:2`         | `4k`         | |
| **默认**（未命中）| `1:1`        | `1k`         | |

### QualityMap（OpenAI quality → RR quality）

| OpenAI `quality` | RR `quality` |
|------------------|--------------|
| `standard`       | `medium`     |
| `hd`             | `high`       |
| `""`（未填）      | `medium`     |
| **默认**（未命中）| `medium`     |

---

## 计费设计

### 管理员配置方式

将 RR 模型计费模式设为 `tiered_expr`（v2: 按次），表达式通过 `param("resolution")` 读取**转换后**的分辨率档位：

```
v2: tier("result",
  param("resolution") == "4k" ? 0.08 :
  param("resolution") == "2k" ? 0.04 :
  0.02
)
```

`size` 到 `resolution` 的转换由代码处理，管理员无需关心 `size` 字段。

### RR 适配器实现（`InjectBillingParams`）

```go
func (a *TaskAdaptor) InjectBillingParams(c *gin.Context, info *relaycommon.RelayInfo) {
    v, ok := c.Get("task_request")
    if !ok {
        return
    }
    req, ok := v.(relaycommon.TaskSubmitReq)
    if !ok {
        return
    }

    cfg := mapSize(req.Size)

    // 确保 BillingRequestInput 存在
    if info.BillingRequestInput == nil {
        storage, err := common.GetBodyStorage(c)
        if err != nil {
            return
        }
        body, _ := storage.Bytes()
        info.BillingRequestInput = &billingexpr.RequestInput{Body: body}
    }

    // 注入转换后的 resolution，供 param("resolution") 使用
    info.BillingRequestInput.Body = billingexpr.InjectBodyParam(
        info.BillingRequestInput.Body, "resolution", cfg.Resolution,
    )
}
```

---

## 图片 URL 处理与有效期

### 上游 URL

RR 任务成功后，上游返回：

```json
"results": [{"url": "https://xxx.cos.ap-beijing.myqcloud.com/...", "outputType": "png"}]
```

URL 为 Tencent COS 对象链接，存在有效期（默认约 1 天），到期后访问返回 403/404。

### 存储与代理

- `ParseTaskResult` 取 `results[0].url` 赋值给 `taskResult.Url`
- 轮询框架存入 `task.PrivateData.ResultURL`
- 同时计算并存入 `task.PrivateData.ExpireAt`：`完成时间 + TTL（秒）`
- TTL 从渠道配置读取：`channel.GetOtherSettings().RRUrlTTLHours`，默认 24 小时
- 用户访问 `/v1/videos/{taskId}/content` → `VideoProxy` 先检查 `ExpireAt`：
  - 未过期：从 `ResultURL` 实时拉流，附带 `Cache-Control: public, max-age=86400`
  - 已过期：返回 410 Gone + 错误信息，不发出无效请求

### 有效期配置

存在 `channel.other` 字段（JSON），由前端 RR 渠道编辑器管理：

```json
{
  "rr_endpoints": {
    "rhart-image-g-2-official": "/openapi/v2/rhart-image-g-2-official/text-to-image"
  },
  "rr_url_ttl_hours": 24
}
```

后端 `ChannelOtherSettings` 新增字段：

```go
// RR 渠道：模型→接口路径映射
RREndpoints   map[string]string `json:"rr_endpoints,omitempty"`
// RR 渠道：上游图片 URL 有效期（小时），默认 24
RRUrlTTLHours int               `json:"rr_url_ttl_hours,omitempty"`
```

`VideoProxy` 中针对 RR 渠道新增过期检查（在 default 分支之前）：

```go
case constant.ChannelTypeRR:
    videoURL = task.GetResultURL()
    if expireAt := task.PrivateData.ExpireAt; expireAt > 0 && time.Now().Unix() > expireAt {
        videoProxyError(c, http.StatusGone, "url_expired",
            "Image URL has expired. Please regenerate the image.")
        return
    }
```

---

## 渠道配置（前端编辑器）

### RR 渠道 `other` 字段编辑器

新建 `rr-path-config-editor.tsx`，**复用 `PosterPathConfigEditor` 结构**，区别：

- 去掉 "API Version" 输入框（RR 路径中模型名即区分版本，无统一版本号）
- 保留 "Endpoint Overrides" 表格（模型 → 完整路径）
- 新增 "URL TTL（小时）" 数字输入框，默认 `24`，最小值 `1`

JSON 结构（写入 `other` 字段）：

```json
{
  "rr_endpoints": { "model-name": "/openapi/v2/model-name/text-to-image" },
  "rr_url_ttl_hours": 24
}
```

### `channel-mutate-drawer.tsx` 新增分支

```tsx
{/* RR (type 59) - upstream path + TTL config */}
{currentType === 59 && (
  <FormField
    control={form.control}
    name='other'
    render={({ field }) => (
      <FormItem>
        <FormLabel>{t('Upstream Path Config')}</FormLabel>
        <FormControl>
          <RRPathConfigEditor
            value={field.value ?? ''}
            onChange={field.onChange}
          />
        </FormControl>
        <FormMessage />
      </FormItem>
    )}
  />
)}
```

---

## 渠道注册

### `constant/channel.go`

```go
ChannelTypeRR = 59  // 紧接 ChannelTypePoster(58) 之后

// ChannelBaseURLs[59]:
"https://www.runninghub.ai"

// ChannelTypeNames[ChannelTypeRR]:
"RR"
```

> ⚠️ `ChannelTypeDummy` 需同步 +1（当前值为 59，新增后变 60）。

### `relay/relay_adaptor.go` — `GetTaskAdaptor`

```go
case constant.ChannelTypeRR:
    return &taskrr.TaskAdaptor{}
```

### `relay/image_handler.go` — `ImageHelper` 分叉

```go
if info.ChannelType == constant.ChannelTypeRR {
    taskReq := convertImageRequestToTaskSubmitReq(imageReq)
    c.Set("task_request", taskReq)
    result, taskErr := RelayTaskSubmit(c, info)
    // ... 处理结果
    return
}
```

---

## TaskAdaptor 接口实现清单

| 方法 | 实现说明 |
|------|----------|
| `Init` | 存 apiKey、baseURL、channelType，读取 `RREndpoints` |
| `ValidateRequestAndSetAction` | 调 `ValidateBasicTaskRequest(c, info, TaskActionGenerate)` |
| `InjectBillingParams` | 查 SizeMap 得到 resolution，注入 `info.BillingRequestInput.Body` |
| `EstimateBilling` | 返回 nil（表达式计费时不使用 OtherRatios） |
| `AdjustBillingOnSubmit` | 返回 nil |
| `AdjustBillingOnComplete` | 返回 0 |
| `BuildRequestURL` | 优先查 `RREndpoints[model]`，否则 `{baseURL}/openapi/v2/{model}/text-to-image` |
| `BuildRequestHeader` | `Content-Type: application/json`，`Authorization: Bearer {key}` |
| `BuildRequestBody` | 查 SizeMap/QualityMap，构造 `{prompt, aspectRatio, resolution, quality}` |
| `DoRequest` | 调 `channel.DoTaskApiRequest` |
| `DoResponse` | 解析响应，提取 `taskId`，返回给框架 |
| `FetchTask` | `POST {baseURL}/openapi/v2/query` `{"taskId": "..."}` |
| `ParseTaskResult` | 状态映射 + 提取 `results[0].url` |
| `GetModelList` | 返回支持的模型列表 |
| `GetChannelName` | 返回 `"RR"` |

---

## FetchTask 状态映射

| RR status | 框架 TaskStatus | Progress |
|-----------|----------------|----------|
| `RUNNING` | `TaskStatusInProgress` | `50%` |
| `PENDING` / 其他未知 | `TaskStatusInProgress` | `20%` |
| `SUCCESS` | `TaskStatusSuccess` | `100%` |
| `FAILED` | `TaskStatusFailure` | `100%` |
