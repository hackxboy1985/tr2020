# Td 渠道视频模型接入设计（minimax-h3 / seedance-2.0 / seedance-2.5）

> **状态**：已实现
> **最后更新**：2026-10-11
> **适用渠道**：Td（土豆，ChannelType 60）
> **上游基址**：`https://api.ai-tudou.net`

本文档记录 Td 渠道接入视频模型的完整设计。**后续新增视频模型时请先读本文档**，绝大多数情况只需在 `videoModelSpecs` 追加一条记录。

---

## 一、背景与结论

Td 渠道原本只承载**图片**生成（`gpt-image-2-all`）。土豆另提供一套**视频**生成接口，当前有三个模型：

- `minimax-h3`
- `seedance-2.0`
- `seedance-2.5`

**核心结论：三者共用同一套协议，差异只在参数取值边界上。**

| 对比项 | minimax-h3 | seedance-2.0 | seedance-2.5 |
|---|---|---|---|
| 提交 | `POST /v1/videos/generations` | 同 | 同 |
| 查询 | `GET /v1/tasks/{task_id}` | 同 | 同 |
| 认证 | `Authorization: Bearer <key>` | 同 | 同 |
| 响应 | `code/message/data.id`、`data.result.video_url` | 同 | 同 |
| 状态机 | submitted / processing / completed / failed | 同 | 同 |

差异（能力矩阵）：

| 能力 | minimax-h3 | seedance-2.0 | seedance-2.5 |
|---|---|---|---|
| `duration` | 3–15 | 3–15 | **4–30** |
| `resolution` | 720p / 1080p / **2k** | **仅 720p** | **仅 720p** |
| `images` 上限 | 9 | 9 | **30** |
| `videos` 参考 | **不支持** | 3 | 10 |
| `audios` 参考 | **不支持** | 3 | 10 |
| `generate_audio` | **不支持** | 支持 | 支持 |

通用约束：`aspect_ratio ∈ {16:9, 9:16}`；`audios` 不能单独使用（需配合 images 或 videos）。

因为协议同构，**没有为每个模型单独建适配器**，而是「一套协议 + 一张能力矩阵」。

---

## 二、总体架构

Td 渠道内按请求形态**分流**为图片、视频两个分支：

```
Td 渠道 (ChannelTypeTudou = 60)
├── 图片分支  POST /v1/images/generations/async   （原有，未改动）
└── 视频分支  POST /v1/videos/generations         （本次新增）
    └── 按模型查 videoModelSpecs 决定参数边界
        ├── minimax-h3    已知
        ├── seedance-2.0  已知
        ├── seedance-2.5  已知
        └── 其它          400 拒绝（不静默透传）
```

对外 API：

| 接口 | 说明 |
|---|---|
| `POST /v1/videos/generations` | 提交视频任务 |
| `GET /v1/videos/tasks/:task_id` | 查询视频任务，返回 `result.video_url` |
| `POST /v1/images/tasks` | 提交图片任务（原有） |
| `GET /v1/images/tasks/:task_id` | 查询图片任务（原有，**响应结构未改动**） |

---

## 三、文件清单

### 新增

| 文件 | 职责 |
|---|---|
| `relay/channel/task/td/video_spec.go` | **能力矩阵 `videoModelSpecs`** —— 扩展新模型主要改这里 |
| `relay/channel/task/td/video_dto.go` | 视频独立的请求/响应结构体 |
| `relay/channel/task/td/video.go` | 视频分支：校验、建体、解析、分支判定 |
| `relay/channel/task/td/video_test.go` | 视频契约测试 |

### 修改

| 文件 | 改动 |
|---|---|
| `relay/channel/task/td/adaptor.go` | 五处分流：校验 / 建体 / 解析 / 计费 / 响应 object |
| `relay/channel/task/td/constants.go` | 视频路径与模型常量 |
| `relay/constant/relay_mode.go` | 新增 `RelayModeVideoTaskFetchByID` + 路径判定 |
| `middleware/distributor.go` | `/v1/videos/*` 路径分发 |
| `relay/relay_task.go` | 新增 `videoTaskFetchByIDRespBodyBuilder` 并注册 |
| `router/relay-router.go` | 注册两条视频路由 |
| `web/default/src/features/channels/lib/channel-type-config.ts` | Td 渠道 models 提示补三个模型名 |

---

## 四、关键设计决策（含原因）

### 4.1 为什么用独立 DTO 而不复用 `TaskSubmitReq`

`relaycommon.TaskSubmitReq` 是**全渠道共用**的任务请求结构体（hailuo/kling/sora/doubao/vertex/jimeng 等十余个适配器都在解析它），不能直接复用：

1. **它没有 `videos` / `audios` / `generate_audio` 字段** —— 复用它这些参数会在反序列化时被静默丢弃；
2. **在公共结构体上加这些字段会波及所有渠道** —— 不支持参考视频的渠道（如 Kling）也会静默接受 `videos`，造成跨渠道误接收；
3. **它的 `Duration` 是非指针 `int`** —— 无法区分「未传」与「显式传 0」。

因此视频分支使用独立结构体 `VideoGenerateRequest`，并遵守 AGENTS.md 规则 6：可选标量用**指针 + omitempty**，保证「未传 => 省略，显式传零值 => 照样下发」。

`GenerateAudio *bool` 尤其重要：显式传 `false` 不能被 `omitempty` 丢掉。

### 4.2 为什么校验走 `videoModelSpecs` 而不是散落各处的 `if`

- **单一事实来源**：模型能力集中为数据，新增模型不用改逻辑；
- **行为可预期**：未知模型 400 拒绝，而不是把参数静默透传给上游（否则用户以为参数生效、实际被忽略）；
- **与上游文档一一对应**：每条记录都能追溯到文档参数表。

### 4.3 未知模型为什么拒绝而不是放行

上游（土豆）才是能力边界的唯一权威，理论上「放行交给上游」更利于兼容上游新增模型。但本项目选择**拒绝**，原因：

1. 视频参数复杂（时长、分辨率、三类素材数量），无法校验就只能盲传；
2. 用户无法区分「参数生效」与「被上游静默忽略」，排障成本高；
3. 新增模型是低频操作，改一行 `videoModelSpecs` 的成本远低于线上参数静默失效的成本。

> 注意：这与图片分支对 `quality` 的「放行交给上游」策略**刻意不同**。图片 quality 是单一枚举、上游档位常变；视频是复合参数边界，两者不适合用同一策略。改动时请勿「统一」掉。

### 4.4 模型名校验时序问题（重要）

`RelayTaskSubmit` 的执行顺序是：

```
步骤 1    adaptor.ValidateRequestAndSetAction(...)   ← 校验在这里
步骤 2    info.OriginModelName / UpstreamModelName = modelName
步骤 2.5  helper.ModelMappedHelper(...)              ← 模型映射在这里
```

**校验早于模型映射**，此时 `info.UpstreamModelName` 尚为空。而 `videoModelSpecs` 的 key 是**映射后的上游模型名**。

如果直接读 `info.UpstreamModelName`，用户配了 `model_mapping` 时会拿到空串而**误拒合法请求**。

**解法**：`resolveVideoModelName()` 在校验阶段提前按 `c.GetString("model_mapping")` 解析映射后的模型名，语义与 `helper.ModelMappedHelper` 保持一致（含链式重定向与环检测）。

### 4.5 结果解析必须按「响应结构」判定，不能靠 adaptor 标记（踩过的坑）

**这是实现过程中发现并修掉的真实 bug，改动此处时务必注意。**

最初的实现用 adaptor 上的 `videoTask` 标记来分流 `ParseTaskResult`。但轮询服务 `service/task_polling.go` 会通过 `GetTaskAdaptorFunc` **新建适配器实例**再 `Init`，**提交阶段的标记根本带不到查询阶段**。

后果：视频结果被按图片（`result.images`）解析，`result.video_url` 取不到，任务显示完成却拿不到视频地址。

**解法**：改为 `isVideoTaskResponse()` **按响应体结构判定**（`result` 中出现 `video_url` 或 `videos` 即为视频），与 Zy 适配器的做法一致。

`videoTask` 标记仅保留给 `BuildRequestURL` 和 `DoResponse` —— 这两处确实在同一请求生命周期内。

防回归测试 `TestParseTaskResultDispatch` **刻意用不带标记的适配器**模拟轮询场景；若谁改回依赖标记，测试会直接失败。

### 4.6 路由前缀顺序

`/v1/videos/tasks/` 的判定**必须排在 `/v1/videos/` 之前**，因为前缀重叠。`relay/relay_task.go` 的 `ResolveOriginTask` 里有 `/v1/videos/` 的 remix 判断，顺序颠倒会走错分支。

---

## 五、【后续扩展指南】新增视频模型怎么做

### 情况一：同协议新模型（最常见）

上游再出一个模型，仍然走 `POST /v1/videos/generations` + `GET /v1/tasks/{id}`。

**只需两步：**

**① 在 `video_spec.go` 的 `videoModelSpecs` 追加一条记录**

```go
"kling-v3": {
    MinDuration:        5,
    MaxDuration:        10,
    DefaultDuration:    5,
    AllowedResolutions: []string{Resolution720P, Resolution1080P},
    MaxImages:          2,
    MaxVideos:          0,     // 不支持参考视频
    MaxAudios:          0,     // 不支持参考音频
    AllowGenerateAudio: false,
},
```

**② 在 `constants.go` 加模型名常量**（可选，但推荐，便于测试引用）

```go
VideoModelKlingV3 = "kling-v3"
```

**就完成了。** 校验、拒绝策略、计费注入、结果解析全部自动生效 —— 因为这些都是由 `videoModelSpecs` 驱动的。

**③ 在渠道配置的 `models` 字段填上新模型名**，并配好模型价格。

> 若要写测试，参考 `video_test.go` 中 `TestVideoModelSpecsConsistency`，它会对每条记录做自洽性断言（时长范围合法、默认值落在范围内、分辨率非空、图片上限为正）。

### 情况二：新协议新模型

如果新模型**不是** `/v1/videos/generations` 协议（不同路径、不同状态机、不同响应结构），则：

1. **路径不同** —— 在 `constants.go` 加新常量，在 `video_spec.go` 的 spec 里增加路径字段，或另开分支；
2. **响应结构不同** —— 扩展 `video_dto.go` 的结构体，并调整 `isVideoTaskResponse` / `parseVideoTaskResult`；
3. **状态机不同** —— 调整 `convertTudouStatus`。

此时改动量取决于差异大小；若差异过大，考虑新建渠道适配器（参考方案 B 的思路：新渠道号 + 独立 adaptor）。

### 情况三：图片模型

图片分支与视频分支完全独立，**直接按原有方式配置即可**，不需要动本文档涉及的任何代码。

---

## 六、计费

视频任务的计费参数由 `InjectBillingParams` 注入 `duration` / `resolution` / `aspect_ratio`，管理员可在计费表达式中用 `param()` 按档位定价：

```
param("duration") == 20 && param("resolution") == "720p" ? tier(..., 0.5) : tier("fallback", 0.2)
```

**注意**：`duration` 缺省时也会被注入（填模型默认值 4），不会因字段缺失导致 `param()` 取不到值而落到兜底档位。这与 Td/Zy 图片渠道的既有做法一致。

---

## 七、响应结构契约

### 提交响应

```json
{
  "id": "task_xxxx",
  "object": "video.task",
  "status": "submitted",
  "model": "seedance-2.5",
  "created": 1700000000
}
```

### 查询响应（`GET /v1/videos/tasks/:task_id`）

成功：

```json
{
  "id": "task_xxxx",
  "object": "video.task",
  "status": "succeeded",
  "model": "seedance-2.5",
  "created": 1700000000,
  "result": {
    "video_url": "https://cdn.example.com/out/xxx.mp4",
    "videos": [{ "url": ["https://cdn.example.com/out/xxx.mp4"] }]
  }
}
```

失败：

```json
{
  "id": "task_xxxx",
  "object": "video.task",
  "status": "failed",
  "error": { "message": "...", "code": "task_failed" }
}
```

**`/v1/images/tasks` 的响应结构完全未变**（`object: "image.task"` + `result.data[].url`），由独立的 `videoTaskFetchByIDRespBodyBuilder` 与 `imageTaskFetchByIDRespBodyBuilder` 分别实现，互不影响。

---

## 八、错误码

所有本地校验失败均返回 **400**，`code` 为 `invalid_request`：

| 场景 | 错误信息 |
|---|---|
| 未知模型 | `unsupported video model: xxx` |
| 时长越界 | `invalid duration: N, model X supports A-B seconds` |
| 时长显式传 0 | 同上（因低于模型下限被拒，不会被当作缺省） |
| 分辨率不合法 | `invalid resolution: X, model Y supports: ...` |
| 比例不合法 | `invalid aspect_ratio: X, must be one of: 16:9, 9:16` |
| 图片超限 | `too many reference images: N, model X allows at most M` |
| 模型不支持图片 | `model X does not support images` |
| 视频超限 / 不支持 | `too many reference videos: ...` / `model X does not support videos` |
| 音频超限 / 不支持 | `too many reference audios: ...` / `model X does not support audios` |
| 音频单独使用 | `audios cannot be used alone, at least one image or video is required` |
| 不支持音效开关 | `model X does not support generate_audio` |
| prompt 为空 | `prompt is required` |

---

## 九、验证情况

| 验证项 | 结果 |
|---|---|
| `go build ./...` | 通过 |
| Td 包测试（31 个） | 全部通过（含原有图片渠道测试，零回归） |
| 路由分发 | `/v1/videos/tasks/` → mode 34，不与 `/v1/images/tasks/`（mode 33）冲突 |
| 视频响应结构 | 真实 SQLite 集成测试验证通过 |
| 图片响应结构 | 实测确认完全未变 |
| 能力矩阵边界 | 逐项实测（duration / 分辨率 / 图片数 / 视频数 / 音频数 / generate_audio） |
| 模型映射 | `my-video → seedance-2.5` 正确命中 |
| 端到端链路 | 对本地 new-api + mock 上游跑通，16/16 通过 |

### 测试脚本

`scripts/test_td_video.sh` 覆盖校验、提交、轮询、图片回归四类验证。

```bash
# 用法一：位置参数（与 scripts/td_test.sh 一致）
./scripts/test_td_video.sh http://127.0.0.1:3000 sk-xxxxx

# 用法二：环境变量
NEWAPI_BASE_URL=http://127.0.0.1:3000 NEWAPI_TOKEN=sk-xxxxx ./scripts/test_td_video.sh

# 只跑校验用例，不实际提交任务（不消耗额度）
SKIP_E2E=1 ./scripts/test_td_video.sh http://127.0.0.1:3000 sk-xxxxx

# 验证「未登记 spec 的模型被拒绝」
SPEC_PROBE_MODEL=<渠道已配但未登记在 videoModelSpecs 的模型名> \
  ./scripts/test_td_video.sh http://127.0.0.1:3000 sk-xxxxx
```

**两个使用注意**（脚本已内置预检，但排查时容易困惑）：

1. 脚本会先做**渠道可用性预检**。若 Td 渠道未配置，分发器会返回 503
   `No available channel`，请求根本到不了适配器，校验用例会「全部失败」。
   预检会直接输出配置指引。
2. `SPEC_PROBE_MODEL` **必须填「渠道已配置、但未登记在 videoModelSpecs」的模型名**。
   填一个随便的未知模型名是测不到的 —— 那种情况在分发器就被 503 拦掉了。

**未验证项**：

1. **前端 typecheck 未执行** —— 编写环境中 `web/default/node_modules/.bin` 为空、`bun` 不存在。前端改动仅为 `hints` 对象内的字符串字面量，已人工核对类型（`hints.other` 是 `string?` 合法字段）与引号闭合，但**建议本地执行一次 `bun run typecheck`**。
2. **未对真实上游发请求** —— 端到端测试用的是本地 mock 上游。已核对 new-api 实际发出的请求体（路径、参数、无图片字段污染）正确，但**上线前仍建议用真实土豆 key 走一次完整链路**。

---

## 十、排查手册

| 现象 | 可能原因 |
|---|---|
| 提交返回 `unsupported video model` | 模型名不在 `videoModelSpecs`；或模型映射配置有误（排查 `model_mapping`） |
| 提交返回 `model X does not support videos` | 该模型确实不支持参考视频（如 minimax-h3），核对上游文档 |
| 提交返回时长错误 | 时长超出该模型范围（如向 seedance-2.0 传 16 秒，应改用 seedance-2.5） |
| 任务完成但拿不到视频地址 | 检查 `result.video_url` 是否被上游返回；确认 `isVideoTaskResponse` 未失效（若上游改了响应结构） |
| 走错上游路径 | 检查 `BuildRequestURL` 的 `a.videoTask` 标记是否被正确设置；检查路由前缀顺序 |
| 图片渠道行为异常 | 视频分支理论上不影响图片分支；用 `TestVideoAndImageEndpointSplit` 与图片原有测试定位 |
