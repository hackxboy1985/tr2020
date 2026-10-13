# Td 渠道 grok-imagine-video-1.5 接入设计

> **状态**：设计待实现
> **最后更新**：2026-10-13
> **适用渠道**：Td（土豆，ChannelType 60）
> **上游文档**：`docs/td/grok.md`（本文档的**唯一需求依据**）
> **关联文档**：[td-video-models.md](./td-video-models.md)（前三个视频模型的接入设计）

---

## 一、结论先说

**grok-imagine-video-1.5 不能靠「在 `videoModelSpecs` 追加一条记录」接入。**

上游路径与现有三个模型**完全一致**（`POST /v1/videos/generations` 提交 + `GET /v1/tasks/{task_id}` 轮询，见上游文档「实际请求土豆的路径」一段），路由与轮询链路无需改动。但**参数层与解析层有 8 处差异**，其中 3 处必须改数据结构，无法用配置表达：

| # | 差异 | 现有实现 | grok 要求 | 是否需改数据结构 |
|---|---|---|---|---|
| 1 | `extra` 参数容器 | **字段不存在** | **必填**，至少含 `aspect_ratio` / `resolution` | ✅ 是 |
| 2 | `aspect_ratio` 取值 | 全局白名单 `16:9` / `9:16` | 另有 `1:1` | ✅ 是（改 per-model） |
| 3 | `duration` 语义 | **范围** Min–Max，可缺省 | **枚举** 6 / 10 / 15，**必填** | ✅ 是 |
| 4 | 单图首帧 | 无对应字段 | `input_reference`，string 或 `{"url":...}` | ✅ 是（联合类型） |
| 5 | 首尾帧 / 多参考图 | 无 | `extra.reference_images[].role` | 随 #1 解决 |
| 6 | 提交响应任务 ID | 只读 `data.id` | `data.id` / `data.task_id` / 顶层 `id` | 否（加兜底） |
| 7 | 任务状态 | 不认 `queued` | 含 `queued` | 否（加分支） |
| 8 | 视频过期时间 | `TaskVideo` 无该字段 | `videos[].expires_at` | 否（加字段） |

**归类**：按 [td-video-models.md](./td-video-models.md) 第五节的分法，grok 属于**「情况二：新协议新模型」**——路径虽同，参数协议不同。原文「同协议新模型只需两步」的前提是**参数取值边界一致**，grok 不满足该前提。

---

## 二、能力矩阵（grok 行）

依据上游文档「可用模型」一段：*时长 6 / 10 / 15 秒，分辨率仅 720p*。

| 能力 | grok-imagine-video-1.5 |
|---|---|
| `duration` | **枚举**：6 / 10 / 15，**必填** |
| `resolution` | 仅 `720p` |
| `aspect_ratio` | `16:9` / `9:16` / **`1:1`** |
| `input_reference` | 单张首帧，string 或 object |
| `extra.reference_images` | 首尾帧 2 张（`first_frame` / `last_frame`）；多参考图 2~7 张（`reference_image`） |
| `images` | **上游文档未出现该字段**，故不支持（传入即 400） |
| `videos` / `audios` / `generate_audio` | 上游文档均未列出，**不支持** |
| `extra` | **必填**，至少含 `aspect_ratio`、`resolution` |

> `duration` 的枚举集合与 `aspect_ratio` 的 `1:1` 是本模型区别于另外三个模型的核心点，校验必须按模型区分。
>
> 参考图有**两条独立通道**：单图走 `input_reference`，多图走 `extra.reference_images`。二者上限不同（前者 1 张，后者 2~7 张），且**互斥**——上游明确拒绝把单图放进 `reference_images`。

---

## 三、逐项改动设计

### 3.1 `extra` 参数容器（差异 #1、#5）

**问题**：`VideoGenerateRequest` 无 `extra` 字段，上游文档将其标为**必填**。用户传入后会被 `common.DecodeJson`（标准 `json.Decoder`）**静默丢弃**——不报错、不留痕，属于最难排查的失败模式。

**设计**：在 `video_dto.go` 的 `VideoGenerateRequest` 增加字段，遵守 AGENTS.md 规则 6（可选字段用 `omitempty`）：

```go
// Extra 上游文档要求必填的补充参数容器，至少含 aspect_ratio、resolution。
// 用 map 而非具名 struct：上游对 extra 的字段集仍在扩展
// （文档已出现 reference_images），具名结构会把未登记字段再次静默丢弃，
// 等于把「静默丢字段」的问题从一层挪到另一层。
Extra map[string]any `json:"extra,omitempty"`
```

**为什么用 `map[string]any`**：`extra` 内部结构由上游定义且已在演进。具名 struct 需要为每个新字段改代码；`map` 天然透传，配合下方的「单图误用」校验即可覆盖已知约束。

**首尾帧 / 多参考图**：`extra.reference_images[].role` 随 `map` 自动透传，无需单独结构体。

**建体透传**：`buildVideoRequestBody` 现有实现是 `upstreamReq := *req` 后 `common.Marshal`，字段在结构体中即自动下发，**无需改动建体逻辑**。

### 3.2 `aspect_ratio` 改为 per-model（差异 #2）

**问题**：`video_spec.go` 中 `allowedAspectRatios` 是**包级全局变量**，`supportsAspectRatio` 是**全局函数**，三个模型共用。grok 需要额外的 `1:1`。

**设计**：`videoModelSpec` 增加字段，函数改为方法，**保留全局集合作为兜底**以保证存量三模型行为零变化：

```go
// AllowedAspectRatios 该模型允许的画面比例；为空时回退到通用集合。
// grok 支持 1:1，与另外三个模型不同，故按模型区分而非全局统一。
AllowedAspectRatios []string
```

```go
func (s videoModelSpec) supportsAspectRatio(aspectRatio string) bool {
    allowed := s.AllowedAspectRatios
    if len(allowed) == 0 {
        allowed = allowedAspectRatios
    }
    return slices.Contains(allowed, strings.TrimSpace(aspectRatio))
}

// aspectRatioHint 用于错误信息，列出该模型允许的比例。
func (s videoModelSpec) aspectRatioHint() string {
    allowed := s.AllowedAspectRatios
    if len(allowed) == 0 {
        allowed = allowedAspectRatios
    }
    return strings.Join(allowed, ", ")
}
```

**调用点改动**：`video.go` 中 `supportsAspectRatio(req.AspectRatio)` → `spec.supportsAspectRatio(...)`，错误信息中的集合同步改用 `spec.aspectRatioHint()`（否则 grok 报错时会提示不含 `1:1` 的集合，自相矛盾）。

### 3.3 `duration` 枚举与必填（差异 #3）

**问题**：`videoModelSpec` 只有 `MinDuration` / `MaxDuration` / `DefaultDuration` 三个**范围**字段。若把 grok 配成 `Min=6, Max=15`，会**放行 7、8、9、11~14 秒**等上游不支持的取值并透传——与 [td-video-models.md](./td-video-models.md) 4.3 节「未知边界拒绝而非放行」的既定原则冲突。同时上游文档标 `duration` **必填**，而现有逻辑缺省填 `DefaultDuration`。

**设计**：增加枚举与必填两个字段：

```go
// AllowedDurations 时长枚举白名单；非空时优先于 Min/Max 范围校验。
// grok 的时长是离散档位（6/10/15）而非连续区间，
// 用 Min=6/Max=15 会放行 7~14 这些上游不支持的取值。
AllowedDurations []int

// RequireDuration 是否强制要求显式传 duration。
// grok 上游文档将 duration 标为必填；另外三个模型可缺省（填 DefaultDuration）。
RequireDuration bool
```

**校验逻辑**（`validateVideoRequest` 中替换现有时长段）：

```go
// 时长：枚举优先，其次范围；RequireDuration 为真时不允许缺省
switch {
case len(spec.AllowedDurations) > 0:
    if req.Duration == nil {
        if spec.RequireDuration {
            return badRequest("duration is required, model %s supports: %s",
                model, spec.durationHint())
        }
        d := spec.DefaultDuration
        req.Duration = &d
    } else if !slices.Contains(spec.AllowedDurations, *req.Duration) {
        return badRequest("invalid duration: %d, model %s supports: %s",
            *req.Duration, model, spec.durationHint())
    }
default:
    // 原有范围逻辑保持不变
}
```

`durationHint()` 需扩展：枚举非空时输出 `6, 10, 15`，否则维持 `3-15` 形式。

> **注意**：现有注释「显式传 0 会因低于下限被拒」仅适用于范围模式；枚举模式下 `0` 同样因不在集合内被拒，行为一致，注释需相应更新。

### 3.4 `input_reference` 联合类型（差异 #4）

**问题**：上游文档明确 `input_reference` 支持**两种写法**——纯 URL 字符串，或 `{"url":"..."}` 对象。现有 DTO 无此字段。

**设计**：新增类型并实现双向自定义 JSON：

```go
// InputReference 单张首帧图。
// 上游文档允许两种写法：纯 URL 字符串，或 {"url":"..."} 对象，故需自定义解析。
// 回传上游时保持原写法（记录 WasObject），避免改变上游收到的报文形态。
type InputReference struct {
    URL       string
    WasObject bool // 入参为对象写法时置 true，Marshal 时还原
}

func (r *InputReference) UnmarshalJSON(data []byte) error {
    trimmed := bytes.TrimSpace(data)
    if len(trimmed) > 0 && trimmed[0] == '"' {
        // 字符串写法
        var s string
        if err := common.Unmarshal(data, &s); err != nil {
            return err
        }
        r.URL, r.WasObject = s, false
        return nil
    }
    // 对象写法
    var obj struct {
        URL string `json:"url"`
    }
    if err := common.Unmarshal(data, &obj); err != nil {
        return err
    }
    r.URL, r.WasObject = obj.URL, true
    return nil
}

func (r InputReference) MarshalJSON() ([]byte, error) {
    if r.WasObject {
        return common.Marshal(struct {
            URL string `json:"url"`
        }{r.URL})
    }
    return common.Marshal(r.URL)
}
```

字段声明为**指针**，区分「未传」与「传了空对象」：

```go
InputReference *InputReference `json:"input_reference,omitempty"`
```

> 必须用 `common.*` 包装函数（AGENTS.md 规则 1），不得直接 `encoding/json`。

### 3.5 `extra.reference_images` 单图误用校验（差异 #5）

**问题**：上游文档反复强调「**单图不要走 `extra.reference_images`**」，并将其列入 400 错误场景。这是**上游的硬约束**，不是本项目的选择。

> **关键差异**：上游文档**全文从未出现 `images` 字段**。grok 的参考图完全走 `input_reference` + `extra.reference_images` 两条通道。因此本模型 `MaxImages` 必须配 **0**（即「不支持 `images` 字段」），不能配 7 —— 否则用户传 `images` 数组会被**静默透传**给一个不认该字段的上游，与本项目「边界不静默透传」的原则相悖。

**设计**：新增 spec 字段 `MaxExtraReferenceImages`，由能力矩阵驱动（与 `MaxImages` / `MaxVideos` 同一范式）：

```go
// MaxExtraReferenceImages extra.reference_images 的数量上限。
// 0 表示不启用该通道的校验（存量三模型即为此值，从而完全不进入本段逻辑）。
// grok 上游文档为「首尾帧 2 张」「多参考图 2~7 张」，故取上限 7。
MaxExtraReferenceImages int
```

校验段（仅当该字段 > 0 时进入，**存量三模型天然跳过**）：

```go
if spec.MaxExtraReferenceImages > 0 {
    refs := extractReferenceImages(req.Extra)
    // 上游约束：单图必须走 input_reference，误用 extra.reference_images 会 400。
    if len(refs) == 1 {
        return badRequest("single reference image must use input_reference, " +
            "not extra.reference_images (upstream rejects this)")
    }
    if len(refs) > spec.MaxExtraReferenceImages {
        return badRequest("too many reference images: %d, model %s allows at most %d",
            len(refs), model, spec.MaxExtraReferenceImages)
    }
}
```

**为什么用 spec 字段而不是布尔标志**：布尔标志（如 `UsesExtraProtocol`）需要在 `validateVideoRequest` 里单独判断，实现时容易写漏条件而误伤其它模型；spec 字段与既有 `MaxImages` / `MaxVideos` 同构，三模型不填即为 `0`，**跳过逻辑是数据结构自带的**，不依赖人工判断。

`extractReferenceImages` 从 `Extra["reference_images"]` 安全取出切片（元素可能是 `map[string]any` 或字符串），元素个数即为参考图数。

> 注意区分三个上限：
> - `MaxImages: 0` 管 `images` 字段（grok 不支持，传入即 400）；
> - `MaxExtraReferenceImages: 7` 管 `extra.reference_images`（上游上限）；
> - `input_reference` 单图**不占**任何配额。
>
> 三者是不同通道，不可混用同一个 spec 字段表达。

### 3.6 提交响应任务 ID 兜底（差异 #6）

**问题**：上游文档明确「任务 ID 取 `data.id` 或 `data.task_id`。部分中继也会把 ID 放在顶层 `id`」。现有 `DoResponse` **只读 `sResp.Data.ID`**，取不到即报 `task_id not found in response`（HTTP 500）。

**设计**：`SubmitResponse` 增加字段，`DoResponse` 按文档顺序兜底：

```go
type SubmitResponse struct {
    ID   string `json:"id"` // 部分中继把任务 ID 放在顶层
    Code int    `json:"code"`
    Data struct {
        ID            string `json:"id"`
        TaskID        string `json:"task_id"` // 与 id 等价，见上游文档
        Status        string `json:"status"`
        Progress      int    `json:"progress"`
        Created       int64  `json:"created"`
        EstimatedTime int    `json:"estimated_time"`
    } `json:"data"`
}
```

```go
// 任务 ID 位置按上游文档顺序兜底：data.id → data.task_id → 顶层 id
// firstNonEmpty 为本次新增的小工具函数（返回首个非空字符串）
taskID = firstNonEmpty(sResp.Data.ID, sResp.Data.TaskID, sResp.ID)
if taskID == "" {
    return ..., "task_id not found in response"  // 原有报错保留
}
```

### 3.7 `queued` 状态（差异 #7）

**问题**：上游文档状态表含 `queued`（「已提交，等待处理」）。`convertTudouStatus` 不认该值，落 `default` → `TaskStatusUnknown`，任务会卡在 unknown 直到超时退款。

**设计**：加分支。**注意 `convertTudouStatus` 是图片与视频分支共用的全局函数**：

```go
case StatusQueued: // "queued"
    // 上游 grok 文档状态表含 queued，与 submitted 同义（等待处理）。
    // 注意：本函数图片/视频分支共用；图片上游不发该状态，
    // 新增此分支对图片链路无可观测影响。
    return string(model.TaskStatusQueued)
```

需在 `constants.go` 增加 `StatusQueued = "queued"` 常量。

### 3.8 视频过期时间（差异 #8）

**问题**：上游文档在 `result.videos[].expires_at` 返回过期时间戳，并说明「视频 URL 约 24 小时有效」。现有 `TaskVideo` 只有 `URL []string`，**没有该字段**——而图片侧的 `TaskImage` 反而有 `ExpiresAt`。视频分支在此处是既有缺口。

**设计**：`TaskVideo` 增加字段，与 `TaskImage` 对齐：

```go
type TaskVideo struct {
    URL       []string `json:"url"`
    ExpiresAt int64    `json:"expires_at,omitempty"` // 过期时间戳（Unix 秒）
}
```

**是否接入 TTL 逻辑**：暂不在本次范围。现有 `PrivateData.ExpireAt` 由渠道级配置 `TdUrlTTLHours` 计算（见 `service/task_polling.go`），与上游返回的 `expires_at` 是两套来源。是否以上游值覆盖本地配置，属独立议题，**本次仅补字段与解析，不改 TTL 决策**。

---

## 四、路由与链路：无需改动

| 环节 | 是否改动 | 说明 |
|---|---|---|
| `router/relay-router.go` | 否 | `/v1/videos/generations`、`/v1/videos/tasks/:task_id` 已注册 |
| `middleware/distributor.go` | 否 | 视频路径分发已就绪 |
| `relay/constant/relay_mode.go` | 否 | `RelayModeVideoTaskFetchByID` 已就绪 |
| `relay/relay_task.go` | 否 | `videoTaskFetchByIDRespBodyBuilder` 已就绪 |
| `constants.go` | **是** | 仅新增模型名与 `StatusQueued` 常量 |
| `isVideoRequest` | 否 | 路径判定已覆盖；模型命中 spec 亦可 |
| `BuildRequestURL` | 否 | `videoTask` 为真即走 `/v1/videos/generations` |
| `BuildRequestBody` | 否 | `*req` 拷贝后序列化，新字段自动透传 |

---

## 五、文件清单

### 修改

| 文件 | 改动 |
|---|---|
| `relay/channel/task/td/video_spec.go` | `videoModelSpec` 增 `AllowedAspectRatios` / `AllowedDurations` / `RequireDuration`；`supportsAspectRatio` 改方法并加 `aspectRatioHint`；`durationHint` 支持枚举；`videoModelSpecs` 追加 grok 记录 |
| `relay/channel/task/td/video_dto.go` | `VideoGenerateRequest` 增 `Extra` / `InputReference`；新增 `InputReference` 类型及双向 JSON；`TaskVideo` 增 `ExpiresAt` |
| `relay/channel/task/td/video.go` | 时长校验支持枚举与必填；比例校验改 per-model；新增 `extra.reference_images` 单图误用校验；新增 `extractReferenceImages` |
| `relay/channel/task/td/adaptor.go` | `DoResponse` 任务 ID 三级兜底；`convertTudouStatus` 增 `queued` |
| `relay/channel/task/td/dto.go` | `SubmitResponse` 增顶层 `ID` 与 `Data.TaskID` |
| `relay/channel/task/td/constants.go` | 增 `VideoModelGrokImagineVideo15`、`StatusQueued` |
| `relay/channel/task/td/video_test.go` | 新增 grok 相关契约测试 |
| `web/default/src/features/channels/lib/channel-type-config.ts` | Td 渠道 models 提示补 grok 模型名 |

### videoModelSpecs 追加记录（设计形态）

```go
VideoModelGrokImagineVideo15: {
    // 时长是离散档位而非区间：上游文档为 6 / 10 / 15，且标为必填
    AllowedDurations:    []int{6, 10, 15},
    RequireDuration:     true,
    DefaultDuration:     10, // RequireDuration 为真时仅作防御性兜底
    AllowedResolutions:  []string{Resolution720P},
    AllowedAspectRatios: []string{"16:9", "9:16", "1:1"}, // 本模型独有 1:1
    // 上游文档全文无 images 字段：参考图走 input_reference 与
    // extra.reference_images 两条通道，故 images 配 0（传入即拒绝），
    // 避免参数被静默透传给不认该字段的上游。
    MaxImages:           0,
    MaxVideos:           0, // 上游文档未列，不支持
    MaxAudios:           0,
    AllowGenerateAudio:  false,
    // extra.reference_images 上限（首尾帧 2 张、多参考图 2~7 张，取上限）。
    // 存量三模型不填此字段（0），从而完全不进入 extra 参考图校验。
    MaxExtraReferenceImages: 7,
},
```

> `AllowedDurations` 与 `MinDuration` / `MaxDuration` 并存：枚举为空时走原范围逻辑，存量三模型不受影响。
>
> `extra.reference_images` 的 7 张上限**不写入 spec**，因为 `extra` 是直通 map、且该上限与 `images` 语义不同，写在 `validateVideoRequest` 内并注明上游依据（见 3.5）。

---

## 六、影响面与回归风险

| 风险点 | 说明 | 缓解 |
|---|---|---|
| 存量三模型行为变化 | 改动涉及共用的 `videoModelSpec` 与 `video.go` 校验段 | 枚举字段为空 → 走原范围分支；`AllowedAspectRatios` 为空 → 回退全局集合。**存量行为应零变化**，需由既有 31 个测试守住 |
| `convertTudouStatus` 跨分支 | 图片/视频共用，新增 `queued` 分支 | 图片上游不发该状态，无可观测影响；注释已标注 |
| `InputReference` 自定义 JSON | 新增 `MarshalJSON`/`UnmarshalJSON` | 需覆盖三种入参（字符串 / 对象 / 缺省）与回传形态一致性的测试 |
| `extra` 透传 | `map[string]any` 可能带入非预期字段 | 已知约束（单图误用）已校验；其余按「上游为准」透传，与本渠道对图片 `quality` 的策略一致 |
| `SubmitResponse` 字段变更 | 图片分支共用该结构体 | 仅**新增**字段与兜底顺序，原 `data.id` 路径优先级最高，图片链路无变化 |

---

## 七、待确认事项

以下两点上游文档存在留白，实现时需明确取舍（**不猜测，先记录**）：

1. **`extra` 是否真为硬性必填** —— 上游文档参数表标「必填」，但未说明漏传时的具体行为。设计上采取**不本地强制**：仅透传，不因缺失而拒绝。理由：本项目对上游未明确的边界一贯「以文档为准但不额外加码」，若上游实际容忍缺省，本地强制会造成误拒。
2. **grok 的对外模型名** —— 上游文档「固定填 `grok-imagine-video-1.5`」。默认按上游名直接暴露，与另外三个模型口径一致；若需映射为产品名，通过渠道 `model_mapping` 实现，不改代码。

---

## 八、验证计划

| 验证项 | 方法 |
|---|---|
| 存量零回归 | 运行 td 包既有 31 个测试，须全过 |
| grok 能力枚举 | 6/10/15 通过；7/14 拒绝且报错列出枚举集合 |
| `duration` 必填 | 不传 duration 被拒，提示 `duration is required` |
| `1:1` 比例 | grok 通过；存量三模型仍拒绝 |
| `extra` 透传 | 断言上游请求体含 `extra` 原内容 |
| `input_reference` 双写法 | 字符串与对象写法均正确解析并原样回传 |
| 单图误用拦截 | `extra.reference_images` 长度 1 → 400，提示改用 `input_reference` |
| `reference_images` 上限 | 长度 8 → 400；长度 2 与 7 → 通过 |
| `images` 字段不支持 | grok 传 `images` → 400（`does not support images`） |
| 三处 ID 兜底 | `data.id` / `data.task_id` / 顶层 `id` 分别能取到 |
| `queued` 状态 | 上游返回 `queued` → 映射为非 unknown 状态 |
| `expires_at` 解析 | 断言 `TaskVideo.ExpiresAt` 取到上游值 |

> 端到端验证参照 `scripts/test_td_video.sh` 的模式扩展即可（该脚本已覆盖渠道预检、校验用例、提交、轮询、图片回归五类）。

---

## 九、与既有文档的关系

- 本文档是 [td-video-models.md](./td-video-models.md) **第五节「情况二：新协议新模型」的具体实例**；
- 实现完成后，需同步更新 [td-video-models.md](./td-video-models.md) 的模型清单与能力矩阵；
- 若 grok 需对外提供，还需在 [video-user-guide.md](../td/video-user-guide.md) 补充该模型的能力说明。
