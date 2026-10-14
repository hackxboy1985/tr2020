# Td 渠道结果转存对象存储 —— 设计方案

> 状态：**已实现（Phase 1 完成）**
> 范围：仅 `channel_type = Td`（`constant.ChannelTypeTudou = 60`）
> 日期：2026-10-13 设计（2026-10-14 修订为 S3 双兼容并完成实现）
> 存储后端：**S3 协议实现，同时兼容 MinIO 与阿里云 OSS**

---

## 实现完成说明（2026-10-14）

Phase 1 已全部实现，验证结果：

| 项目 | 结果 |
|---|---|
| 后端全量编译 | ✅ 通过 |
| `go vet`（改动包） | ✅ 通过 |
| 新增单元测试 | ✅ 47 个用例全部通过 |
| 前端类型检查（改动文件） | ✅ 无错误 |

**新增文件**：

| 文件 | 说明 |
|---|---|
| `pkg/oss/client.go` | S3 客户端单例、配置指纹检测、超时控制 |
| `pkg/oss/upload.go` | 流式下载 + PutObject、SSRF 校验、体积与类型预检 |
| `pkg/oss/key.go` | 对象键生成、扩展名推断、路径穿越防护 |
| `pkg/oss/url.go` | 公网地址拼接 |
| `pkg/oss/oss_test.go` | 单测 |
| `setting/system_setting/object_storage.go` | 全局配置（`S3_*` 环境变量） |
| `service/task_oss_transfer.go` | 转存判定、异步执行、失败记录、补扫 |
| `service/task_oss_transfer_test.go` | 单测（转存判定 12 例、失败阶段 6 例等） |
| `model/oss_transfer_failure.go` | 失败记录表 |
| `relay/channel/task/taskcommon/media_type.go` | 图片/视频响应体判据（**单一数据源**） |

**实现期的重要决策（与设计稿的差异）**：

1. **判据下沉到 `taskcommon`**：原设计在 `service` 内复刻判据，实现时发现 `td` 包已导入 `service`，反向导入会形成循环依赖。故将 `IsVideoTaskResponse` 下沉至两者都可依赖的 `taskcommon`，并把 `td.isVideoTaskResponse` 改为转发调用，实现**单一数据源**。

2. **原 URL 额外写入消费日志**：除任务表 `private_data.result_url` 外，转存成功时还会把原 URL 写入消费日志的 `other` 字段（`oss_origin_url`），使管理员可在「后台 → 通用日志 → 详情 → Raw Data」中直接查看。经核实该区块仅管理员可见（前端 `isAdmin` 判断 + 后端 `/api/log` 走 `AdminAuth`）。

3. **代理路由短路**：`VideoProxy` 在结果已转存时直接回源对象存储，跳过各渠道的上游拉取逻辑，避免用上游 Key 请求本系统存储，也避免上游 TTL 过期检查误伤。

---

## 0. 修订说明（2026-10-14）

原始方案仅对接阿里云 OSS。经评估，因部署在宝塔面板，改用**自建 MinIO**（本地磁盘存储）更省成本。为兼顾当下与未来，存储层改为 **S3 协议实现**：

| 维度 | 决策 |
|---|---|
| 协议 | S3（`aws-sdk-go-v2/service/s3`） |
| 当前后端 | **MinIO**（宝塔部署，独立数据盘/RAID） |
| 未来后端 | 阿里云 OSS / Cloudflare R2 / 七牛 等任意 S3 兼容服务，**切换只改配置，零代码改动** |
| 转存范围 | 图片与视频**都走 MinIO**（各由 1 个渠道开关控制） |

**为什么选 S3 协议而非 MinIO 专有 SDK**：`minio-go` 或 S3 SDK 均可连接 MinIO，但 S3 协议实现的兼容面更广——同一份代码既能连自建 MinIO，也能连阿里云 OSS（阿里云提供 S3 兼容端点），避免日后更换存储时重写实现层。

> **重要前提**：本方案采用**公有读 bucket 直出 URL**（原决策）。MinIO 部署在内网时用户无法直接访问 `127.0.0.1:9000`，**必须**在宝塔配置域名/反向代理并启用 HTTPS。详见 §4.1.1。

---

## 1. 背景与问题

### 1.1 现状

Td 渠道同时承载**图片**与**视频**两类上游接口：

| 类型 | 提交接口 | 结果字段 |
|---|---|---|
| 图片 | `POST /v1/images/generations/async` | `result.images[0].url[0]` |
| 视频 | `POST /v1/videos/generations` | `result.video_url`（回退 `result.videos[0].url[0]`） |

**当前实现中，结果文件始终留在上游服务器，本系统只保存 URL 字符串。**

### 1.2 代码核实

三个关键节点（均有代码依据）：

**① 解析结果** —— `relay/channel/task/td/adaptor.go` `ParseTaskResult`

按响应体结构分流，不依赖适配器状态：

```go
if isVideoTaskResponse(respBody) {
    return parseVideoTaskResult(respBody)   // result.video_url / result.videos
}
// 否则取 result.images[0].url[0]
```

> 注释中已明确：轮询会新建适配器实例并重新 `Init`，提交阶段的 `videoTask` 标记不会带到查询阶段，因此**必须按响应体结构判定**。本设计的转存判定同样不能依赖适配器状态。

**② 落库** —— `service/task_polling.go` `TaskStatusSuccess` 分支

```go
} else if taskResult.Url != "" {
    task.PrivateData.ResultURL = taskResult.Url   // 上游 URL 原样保存
}
```

**③ 返回用户** —— `relay/relay_task.go`

```go
case constant.ChannelTypeTudou:
    resultURL = rewriteResultURLWithMappings(resultURL, settings.TdUrlProxyBaseURL, settings.TdUrlProxyMappings)
```

### 1.3 核心缺口

现有三个 Td 配置项（`dto/channel_settings.go`）**都不解决文件持久化**：

| 配置项 | 实际作用 | 是否搬移文件 |
|---|---|---|
| `TdUrlTTLHours` | 仅记录 `PrivateData.ExpireAt` 时间戳 | ❌ |
| `TdUrlProxyBaseURL` | 仅改写 URL 的 scheme/host | ❌ |
| `TdUrlProxyMappings` | 按域名映射代理前缀 | ❌ |

**因此：上游 URL 一旦过期（默认 24 小时），用户将永久失去该结果**——因为文件从未被搬运到本系统可控的存储中。

### 1.4 澄清：与「广告视频」链路无关

仓库中存在另一处含 OSS 字样的注释：

- `model/video_project.go`：`VIDEO_CONCAT // 拼接完成，等待 对象存储上传`
- `dto/video_generation.go`：`ProductImgUrl // 产品图 OSS URL`

这些属于**广告视频项目**链路（`video_projects` 表 + `service/video_generation_service.go`），与本方案针对的 **Td 任务**链路（`tasks` 表 + `relay/channel/task/td/`）是**两套完全独立的系统**，无共享字段与状态。本方案**不涉及**广告视频链路。

---

## 2. 目标与非目标

### 2.1 目标

1. 将 Td 生成的图片/视频结果转存至阿里云 OSS，用户获得**不依赖上游 TTL** 的稳定地址
2. 图片与视频**分别由 2 个渠道开关控制**，默认关闭
3. 转存失败**绝不阻断**主流程与计费结算
4. 进程重启后未完成的转存可**自动补偿**

### 2.2 非目标

- ❌ 不回溯转存历史任务（仅对新任务生效）
- ❌ 不改造广告视频链路
- ❌ 不改动 RR / Zy 渠道
- ❌ 不引入签名 URL / STS 临时凭证（采用公有读 bucket）

---

## 3. 总体架构

```
                      上游 Td 生成结果
                             │
                             ▼
                 task_polling 轮询到 SUCCESS
                             │
                ┌────────────┴────────────┐
                │  ① 落库 ResultURL       │
                │  ② 计算 ExpireAt        │
                │  ③ shouldSettle = true  │  ← 计费在此完成，不受转存影响
                └────────────┬────────────┘
                             │
                ┌────────────▼────────────┐
                │  ④ 判定是否需转存        │
                │  渠道≡Td && 开关开 &&    │
                │  对象存储配置完整 &&         │
                │  OssURL 为空（幂等）      │
                └────────────┬────────────┘
                             │ 异步 goroutine
                             ▼
                   ┌──────────────────┐
                   │ 流式下载 → 对象存储上传│
                   └────────┬─────────┘
                      成功 ┌─┴─┐ 失败
                           ▼   ▼
                    OssURL   仅记日志
                    落库     保持降级
                           │
                           ▼
                  返回用户时优先取 OssURL
```

### 3.1 三条核心原则

| 原则 | 说明 |
|---|---|
| **解耦** | 转存与结算完全解耦。转存失败不影响 `shouldSettle`，计费正确性优先 |
| **降级** | `ResultURL` 永远保留上游原址。任何时刻用户都能拿到**可用** URL |
| **幂等** | 对象键以 `task_id` 为基准，重复上传覆盖同一对象，不产生垃圾文件 |

---

## 4. 详细设计

### 4.1 全局对象存储配置

新增 `setting/system_setting/oss.go`，沿用 `ServerAddress` / `WorkerUrl` 的既有模式（`setting/system_setting/system_setting_old.go`）。

#### 4.1.1 配置字段

| 字段 | 环境变量 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `Endpoint` | `S3_ENDPOINT` | ✅ | — | MinIO：`http://127.0.0.1:9000`<br>阿里云：`oss-cn-hangzhou.aliyuncs.com` |
| `Bucket` | `S3_BUCKET` | ✅ | — | 公有读 bucket |
| `AccessKeyId` | `S3_ACCESS_KEY_ID` | ✅ | — | |
| `AccessKeySecret` | `S3_ACCESS_KEY_SECRET` | ✅ | — | |
| `Region` | `S3_REGION` | ❌ | `us-east-1` | MinIO 忽略此值，但 S3 SDK 要求非空 |
| `ForcePathStyle` | `S3_FORCE_PATH_STYLE` | ❌ | `true` | **MinIO 必须 `true`**；阿里云可 `false` |
| `PublicBaseURL` | `S3_PUBLIC_BASE_URL` | ❌ | 空 | 对外访问域名（CDN / 反代域名） |
| `PathPrefix` | `S3_PATH_PREFIX` | ❌ | `td/` | 对象键前缀 |
| `UploadTimeout` | `S3_UPLOAD_TIMEOUT` | ❌ | `300` | 秒 |

**配置完整性判定**：`Endpoint`、`Bucket`、`AccessKeyId`、`AccessKeySecret` 四者**全部非空**才视为「已配置」。未配置时所有转存逻辑短路，行为与现状完全一致。

#### 4.1.2 两种后端的配置差异（关键）

| | MinIO（自建） | 阿里云 OSS |
|---|---|---|
| `Endpoint` | `http://127.0.0.1:9000` | `oss-cn-hangzhou.aliyuncs.com` |
| `ForcePathStyle` | **`true`（必须）** | `false` |
| `PublicBaseURL` | **必须填**（反代域名） | 可留空，自动拼 `https://{bucket}.{endpoint}` |
| `Region` | 任意非空值 | 填实际地域，如 `cn-hangzhou` |

**`ForcePathStyle` 为什么是 MinIO 的硬性要求**：S3 SDK 默认使用 virtual-hosted 风格（`{bucket}.{endpoint}`），该风格依赖 DNS 泛解析。MinIO 部署在裸 IP 或内网时无法解析，必须强制 path-style（`{endpoint}/{bucket}`），否则请求会打到错误地址。这是 MinIO 对接**最容易踩的坑**。

#### 4.1.3 公网地址拼接规则

```
若 PublicBaseURL 非空 → {PublicBaseURL}/{objectKey}
否则                → https://{Bucket}.{Endpoint}/{objectKey}
```

**MinIO 部署必须设置 `PublicBaseURL`**：内网地址 `127.0.0.1:9000` 用户无法访问。需要在宝塔配置反向代理（如 `https://oss.example.com` → `127.0.0.1:9000`），并将其填入 `PublicBaseURL`。

**必须启用 HTTPS**：若前端页面是 HTTPS，而视频地址是 HTTP，浏览器会因**混合内容（Mixed Content）拦截**而无法播放。宝塔需为该域名申请并配置 SSL 证书。

**密钥安全**：`AccessKeySecret` 的 key 含 `Secret`，命中 `common/constants.go` 中的既有约定——含 `Secret` / `Token` 的 option key 不会通过 `GetOptions` 返回前端，天然满足不泄露要求。

### 4.2 渠道级开关（新增 2 项）

在 `dto.ChannelOtherSettings` 增加：


```go
// TdOssImageEnabled: Td 图片结果是否转存对象存储，默认 false
TdOssImageEnabled bool `json:"td_oss_image_enabled,omitempty"`
// TdOssVideoEnabled: Td 视频结果是否转存对象存储，默认 false
TdOssVideoEnabled bool `json:"td_oss_video_enabled,omitempty"`
```

**为什么拆成 2 个开关**：图片（~MB 级）与视频（可达数百 MB）的体积差异达 2～3 个数量级，占用磁盘空间与清理策略完全不同，运营上需要独立控制。当前决策为**两者都开启并都走 MinIO**，但保留独立开关以便日后按需分级（例如视频改为走云存储）。

**默认 `false`**：未显式开启时，全链路行为与改动前**逐字节一致**，零回归风险。

### 4.2b 旧映射方案：保留但不再使用

> **决策：`TdUrlProxyBaseURL` / `TdUrlProxyMappings` / `TdUrlTTLHours` 三个配置项一律保留，不删除。**

#### 为什么保留而不删除

这三个配置项是**旧的 URL 映射方案**：不搬移文件，只把上游 URL 的 `scheme/host` 替换成配置的代理地址。新方案改为真正把文件转存到对象存储，因此**新流程不再需要它们**。

但**必须保留**，理由如下：

| 理由 | 说明 |
|---|---|
| **存量渠道配置** | 数据库 `channels.other_settings` 中已存有这些字段。删除结构体字段会导致**反序列化丢弃**，管理员已有的配置静默消失 |
| **前端表单已存在** | `td-config-editor.tsx` 与 `channel-type-config.ts` 均有对应 UI 与说明文案，删除需同步改动前端，扩大改动面 |
| **回退路径** | 若对象存储方案出现问题，管理员可关闭新开关，继续使用旧映射方案，无需重新配置 |
| **零成本** | 保留字段不产生任何运行时开销 |

#### 新旧方案的共存规则

| 场景 | 行为 |
|---|---|
| 新开关**开启** + 转存**成功** | 返回对象存储地址。**旧映射不参与**（见下方陷阱说明） |
| 新开关**开启** + 转存**失败/进行中** | 返回上游 URL，**此时仍走旧映射改写**（保持降级后仍可用） |
| 新开关**关闭** | 完全走旧映射方案，行为与改动前一致 |

#### ⚠️ 关键陷阱（与 §4.5 同一问题）

`rewriteResultURLWithMappings()` 只能作用于**上游原始 URL**。若把它应用到对象存储地址上，域名会被错误替换。

因此 §4.5 的实现必须严格保证：

```go
resultURL := originTask.GetPreferredResultURL()
// 仅当结果仍是上游地址时才做代理改写
if originTask.PrivateData.OssURL == "" {
    // … 现有的 rewriteResultURLWithMappings 逻辑保留 …
}
```

**注意**：`TdUrlTTLHours` 的 `ExpireAt` 计算逻辑（`task_polling.go:460`）**同样保留**。它只影响补扫时「上游 URL 是否已超期」的判定，与是否转存无关。

### 4.3 OSS 客户端封装

新增包 `pkg/oss/`：

```
pkg/oss/
  client.go    // 单例客户端：懒加载、配置变更检测与重建、并发安全
  upload.go    // UploadFromURL(ctx, srcURL, objectKey) (publicURL string, err error)
  key.go       // BuildObjectKey(taskID, srcURL, isVideo) string
  oss_test.go  // 对象键生成、URL 拼接、配置判定的单元测试
```

#### 4.3.1 关键设计点

**① 复用既有下载入口，不另造轮子**

通过 `service.DoDownloadRequest()`（`service/download.go`）获取源文件流。该方法**已内置 SSRF 校验**（`common.ValidateURLWithFetchSetting`），且兼容 worker 启用/未启用两种模式。

**② 强制流式，禁止全量读入内存**

视频文件可达数百 MB。必须使用 `io.Copy` 与 S3 `PutObject` 的流式接口（传入 `io.Reader`）。**严禁 `io.ReadAll`**。

> S3 的 `PutObject` 需要预先知道内容长度或使用 multipart。实现时应优先从源响应读取 `Content-Length` 并作为 `ContentLength` 传入；若上游未返回长度，则使用 S3 的 **multipart upload** 分片流式上传，避免为求长度而缓冲整个文件。

**③ 对象键规则**

```
{PathPrefix}{yyyy}/{MM}/{dd}/{task_id}.{ext}
例：td/2026/10/13/task_abc123....mp4
```

- 用 `task_id` 而非随机串：保证**幂等**，重复上传覆盖同一对象
- `ext` 提取顺序：源 URL 路径扩展名 → 响应 `Content-Type` 推断 → 兜底（视频 `.mp4` / 图片 `.png`）
- 按日期分目录：便于生命周期规则与人工排查

**④ 上传前预检**

- `Content-Length` 超过阈值（建议 2 GB，可配）直接拒绝，防止异常大文件打爆带宽
- 校验 `Content-Type` 是否为 `video/*` / `image/*`，防止把 HTML 错误页当作结果上传

**⑤ 超时控制**

使用独立的 `http.Client` + `context.WithTimeout`，超时时间取 `UploadTimeout`。**不得使用无限期 context**。

#### 4.3.2 接口定义

```go
// IsConfigured 全局对象存储配置是否完整
func IsConfigured() bool

// UploadFromURL 从源 URL 流式转存到对象存储，返回公网可访问地址
// 幂等：相同 objectKey 重复调用覆盖同一对象
func UploadFromURL(ctx context.Context, srcURL, objectKey string) (string, error)

// PublicURL 由对象键拼接公网地址（PublicBaseURL 优先，否则按虚拟主机风格）
func PublicURL(objectKey string) string
```

**包内文件划分**：

```
pkg/oss/
  client.go    // S3 客户端单例：Endpoint/Region/ForcePathStyle 配置、并发安全、配置变更重建
  upload.go    // UploadFromURL：流式下载 + PutObject/multipart
  key.go       // BuildObjectKey、扩展名推断
  url.go       // PublicURL 拼接
  oss_test.go  // 单测：键生成 / URL 拼接 / 配置判定 / path-style 分支
```

**SDK 选型说明**：使用 `github.com/aws/aws-sdk-go-v2/service/s3`。项目 `go.mod` 已引入 `aws-sdk-go-v2`（v1.41.5）、`aws-sdk-go-v2/credentials`，仅需新增 `service/s3` 一个模块，**与既有依赖同源，不引入第二套 SDK 体系**。

客户端构造要点：

```go
cfg := aws.Config{
    Region:      settings.Region,          // MinIO 忽略，但必填
    Credentials: credentials.NewStaticCredentialsProvider(ak, sk, ""),
}
client := s3.NewFromConfig(cfg, func(o *s3.Options) {
    o.BaseEndpoint = aws.String(settings.Endpoint)
    o.UsePathStyle = settings.ForcePathStyle   // MinIO 必须 true
})
```

### 4.4 转存触发与补偿

#### 4.4.1 实时触发（`service/task_polling.go`）

在 `TaskStatusSuccess` 分支中，**`shouldSettle = true` 之后**追加：

```go
// 现有代码
task.PrivateData.ResultURL = taskResult.Url
// … 现有 ExpireAt 计算 …
shouldSettle = true

// 新增：判定并异步转存（不阻塞、不影响结算）
if shouldTransferToOss(ch, task, taskResult.Url) {
    go transferTaskResultToOss(task.TaskID, taskResult.Url, isVideo)
}
```

**判定函数 `shouldTransferToOss` 的全部前置条件**（须同时满足）：

| # | 条件 | 理由 |
|---|---|---|
| 1 | `ch.Type == constant.ChannelTypeTudou` | 仅 Td 渠道 |
| 2 | 对应开关已开启（视频看 `TdOssVideoEnabled`，图片看 `TdOssImageEnabled`） | 运营可控 |
| 3 | `oss.IsConfigured()` 为真 | 配置不完整时短路 |
| 4 | `task.PrivateData.OssURL == ""` | **幂等**，避免轮询重复上传 |
| 5 | URL 非空、非 `data:` 前缀、非本系统代理地址 | 无实际文件可转存 |

**判定「图片 or 视频」的依据**：不能依赖适配器状态。以 `taskResult.Url` 与响应体结构判定——沿用 `td.isVideoTaskResponse` 的同一判据（`result` 中出现 `video_url` 或 `videos` 即为视频）。若实现时难以在轮询侧获取原始响应体，则退化为按 URL 扩展名/`Content-Type` 判定，并在代码中明确注释该取舍。

#### 4.4.2 重启补偿（新增扫描逻辑）

**问题**：异步 goroutine 在进程重启时会丢失，导致任务已 SUCCESS 但 `OssURL` 长期为空。

**方案**：在既有轮询任务中增加**补扫**分支，每次 tick 扫描「需转存但未转存」的任务并重试。

新增查询（`model/task.go`）：

```go
// GetPendingOssTransferTasks 查询已成功但尚未转存对象存储 的任务
// 仅返回未超期的任务，避免对已失效 URL 做无意义重试
func GetPendingOssTransferTasks(channelType int, limit int) []*Task
```

**筛选条件**：
- `status = SUCCESS`
- `channel_id` 对应渠道类型为 Td（且开关开启）
- `private_data` 中 `oss_url` 为空
- `expire_at = 0` 或 `expire_at > now`（**未超期**）

> **跨库实现注意**：`private_data` 是 `type:json` 列。在 SQLite / MySQL / PostgreSQL 三库上做 JSON 内部字段的 SQL 过滤会引入方言差异（PostgreSQL `->>`、MySQL `JSON_EXTRACT`、SQLite `json_extract`），违反 AGENTS.md Rule 2。
>
> **因此采用应用层过滤**：先按 `status` + `channel_id` + `expire_at` 等**普通列**做 DB 过滤（这些条件已能大幅缩小结果集），再在 Go 侧解析 `private_data` 判断 `oss_url` 是否为空。这样三库行为完全一致。

**限流**：单次 tick 补扫上限（建议 20 条），避免大量积压任务同时上传拖垮带宽。

#### 4.4.3 重试上限（防止无限重试）

**决策：每个任务最多尝试 3 次，超过则停止并标记。**

在 `TaskPrivateData` 记录尝试次数：

```go
OssTransferAttempts int `json:"oss_transfer_attempts,omitempty"` // 已尝试转存次数
```

补扫时的判定条件增加一条：`OssTransferAttempts < 3`。达到上限后**不再重试**，任务保留降级状态（用户仍可用上游 URL 直到其过期），并已在失败表中留有记录可供排查。

**为什么设上限**：若上游 URL 已失效、或对象存储持续不可用，无限重试只会：
① 刷满失败记录表；② 持续占用带宽与 goroutine；③ 污染日志。3 次足以覆盖网络抖动等瞬时故障。

### 4.5 转存失败记录表

**决策：新增独立表记录转存失败。每次失败都写入一条记录（含全部重试），供排查与统计失败率。首期只建表，管理 API 与前端页面放到二期。**

#### 4.5.1 表设计

新增 `model/oss_transfer_failure.go`：

```go
// OssTransferFailure 对象存储转存失败记录
// 每次转存失败写入一条（含重试），用于统计失败率与排查根因
type OssTransferFailure struct {
    Id           int64  `json:"id" gorm:"primaryKey"`
    CreatedAt    int64  `json:"created_at" gorm:"bigint;index:idx_oss_fail_created_id,priority:1"` // 本次失败时间（Unix 秒）

    TaskId       string `json:"task_id" gorm:"type:varchar(191);index;index:idx_oss_fail_task_attempt,priority:1"` // 任务 ID
    ChannelId    int    `json:"channel_id" gorm:"index"`            // 渠道 ID
    ChannelType  int    `json:"channel_type" gorm:"index"`          // 渠道类型（预留，当前恒为 Td）

    SourceUrl    string `json:"source_url" gorm:"type:text"`        // 上游原始 URL
    ObjectKey    string `json:"object_key" gorm:"type:varchar(255)"`// 计划写入的对象键

    FailReason   string `json:"fail_reason" gorm:"type:text"`       // 失败原因（完整错误信息）
    FailStage    string `json:"fail_stage" gorm:"type:varchar(30);index"` // 失败阶段：download / upload / precheck

    AttemptNo    int    `json:"attempt_no" gorm:"index;index:idx_oss_fail_task_attempt,priority:2"` // 第几次尝试（1/2/3）
    IsFinal      int    `json:"is_final" gorm:"type:tinyint;default:0;index"` // 是否最后一次（1=已达上限，不再重试）

    MediaType    string `json:"media_type" gorm:"type:varchar(10);index"` // image / video
    FileSize     int64  `json:"file_size" gorm:"default:0"`         // 源文件大小（字节），0=未知
}

func (OssTransferFailure) TableName() string {
    return "oss_transfer_failures"
}
```

**注册方式**：加入 `model/main.go` 的 `DB.AutoMigrate(...)` 列表（与 `&Task{}`、`&VideoProject{}` 同级）。GORM 自动建表，**三库兼容**（AGENTS.md Rule 2 自动满足，未使用任何方言特性）。

#### 4.5.2 字段设计说明

| 字段 | 用途 |
|---|---|
| `TaskId` | 关联 `tasks.task_id`，便于定位原始任务 |
| `SourceUrl` | **你明确要求的字段**。保留原始地址，即使上游 URL 日后失效也能追溯来源 |
| `FailReason` | **你明确要求的字段**。完整错误信息，用于归类根因 |
| `FailStage` | 区分是**下载**失败还是**上传**失败。这两类问题的排查方向完全不同 |
| `AttemptNo` | **第几次尝试**（1/2/3）。每次失败独立成行，可直接看出「每次都失败」还是「第 2 次才失败」 |
| `IsFinal` | 是否已达重试上限。**快速筛出真正不可恢复的任务**，无需再按 `task_id` 聚合 |
| `MediaType` | 图片/视频分开统计（两者的失败诱因不同，视频更可能是超时/体积） |
| `FileSize` | 失败时的文件大小。用于判断失败是否与体积相关（如超大视频超时） |

#### 4.5.3 写入策略（每次失败一条）

**决策：每一次失败都 INSERT 一条新记录，不做 UPDATE。**

```
第1次失败 → INSERT（AttemptNo=1, IsFinal=0）
第2次失败 → INSERT（AttemptNo=2, IsFinal=0）
第3次失败 → INSERT（AttemptNo=3, IsFinal=1）← 达上限，标记终止
转存成功  → 不写入任何记录
```

**为什么改成"每次一条"**（相比原先的「每任务一条」）：

| 优势 | 说明 |
|---|---|
| **完整重试轨迹** | 能看到第 1/2/3 次的失败原因**分别是什么**。原方案只保留最新原因，会丢失「前两次是超时，第三次是磁盘满」这类关键演变 |
| **失败原因可演变** | 重试失败的原因往往不同，逐条记录才能定位真实根因 |
| **统计更准确** | 可直接算「总失败次数」而非「失败任务数」 |
| **无并发竞态** | 纯 INSERT，不需要 `SELECT` 再决定 INSERT/UPDATE，实现更简单、无竞态风险 |

**代价（可接受）**：
- 表体积最多为失败任务数的 **3 倍**（受重试上限约束，不会无限增长）
- 同一 `task_id` 会有多行，查询明细时需按 `created_at` 排序理解顺序

**为什么可以接受**：重试上限锁死在 3 次，所以单任务最多 3 行。即使 1 万个任务全失败，也只有 3 万行——对三库都毫无压力。

#### 4.5.4 失败率的口径

由于每次失败独立成行，可计算三个层次的指标：

| 指标 | 公式 | 含义 |
|---|---|---|
| **失败次数** | `COUNT(*)` | 总失败次数（含重试） |
| **失败任务数** | `COUNT(DISTINCT task_id)` | 有多少任务至少失败过一次 |
| **最终失败率** | `COUNT(DISTINCT task_id WHERE is_final=1) / 期间转存任务总数` | **最关心的指标**——真正不可恢复的比例 |
| **平均重试次数** | `COUNT(*) / COUNT(DISTINCT task_id)` | 反映故障是瞬时还是持续性 |

**首期（仅建表）** 可用 SQL 直接查：

```sql
-- 每日失败次数 与 失败任务数
SELECT date(created_at, 'unixepoch') AS d,
       COUNT(*)                     AS fail_times,
       COUNT(DISTINCT task_id)      AS fail_tasks
FROM oss_transfer_failures
GROUP BY d ORDER BY d DESC;

-- 按失败阶段归类（快速定位是下载还是上传问题）
SELECT fail_stage, COUNT(*) FROM oss_transfer_failures GROUP BY fail_stage;

-- 按媒体类型
SELECT media_type, COUNT(*), COUNT(DISTINCT task_id) 
FROM oss_transfer_failures GROUP BY media_type;

-- 最终失败（已达重试上限）的任务明细
SELECT task_id, source_url, attempt_no, fail_stage, fail_reason, created_at
FROM oss_transfer_failures
WHERE is_final = 1
ORDER BY created_at DESC LIMIT 50;

-- 同一任务的三次失败原因演变（定位根因变化）
SELECT task_id, attempt_no, fail_stage, fail_reason
FROM oss_transfer_failures
WHERE task_id = 'task_xxx'
ORDER BY attempt_no;

-- 平均重试次数（>1 说明多为瞬时故障）
SELECT COUNT(*)*1.0 / COUNT(DISTINCT task_id) AS avg_attempts 
FROM oss_transfer_failures;
```

> ⚠️ 上述 `date(..., 'unixepoch')` 是 SQLite 语法。MySQL / PostgreSQL 需用各自函数（`FROM_UNIXTIME` / `to_timestamp`）。这是**运维排查用的一次性查询**，不属于业务代码，故不受 AGENTS.md Rule 2 约束；但若二期做成 API，必须改用 GORM 或 Go 侧聚合。

#### 4.5.5 二期规划（本期不实现）

以下内容**明确延后到二期**，本期只保证表结构就绪：

1. 管理 API：`GET /api/oss/transfer/stats`（失败率、按阶段/类型分组、趋势）
2. 管理 API：`GET /api/oss/transfer/failures`（分页明细）
3. 前端管理页面：失败列表 + 趋势图
4. 失败记录的手动清理策略（避免长期累积）

**本期需保证的**：表结构一旦确定，二期新增查询接口无需再改表。

### 4.6 结果返回（`relay/relay_task.go`）

两处 builder 修改：

- `imageTaskFetchByIDRespBodyBuilder`（约 609 行）
- `videoTaskFetchByIDRespBodyBuilder`（约 676 行）

**在 `model/task.go` 增加统一取值方法**，把「优先对象存储、降级上游」的规则收敛到一处：

```go
// GetPreferredResultURL 优先返回转存后的对象存储地址；未转存时回退上游原始 URL。
// 使用场景：对外返回任务结果时。
func (t *Task) GetPreferredResultURL() string {
    if t.PrivateData.OssURL != "" {
        return t.PrivateData.OssURL
    }
    return t.GetResultURL()
}
```

#### ⚠️ 关键陷阱：对象存储地址不得进入 URL 改写

`rewriteResultURLWithMappings()` 的用途是**改写上游域名**。若对已经是本系统对象存储的地址再执行改写，会把域名改成错误的代理地址。

**必须用条件分支隔开**：

```go
resultURL := originTask.GetPreferredResultURL()
// 仅当结果仍是上游地址时才做代理改写
if originTask.PrivateData.OssURL == "" {
    // … 现有的 rewriteResultURLWithMappings 逻辑保留 …
}
```

这是实现阶段**最易出错**的一点，需重点测试。

### 4.7 数据字段（`model/task.go`）

`TaskPrivateData` 增加：

```go
OssURL              string `json:"oss_url,omitempty"`               // 转存后的对象存储公网地址
OssObjectKey        string `json:"oss_object_key,omitempty"`        // 对象键，便于后续清理/迁移
OssUploadedAt       int64  `json:"oss_uploaded_at,omitempty"`       // 转存完成时间（Unix 秒）
OssTransferAttempts int    `json:"oss_transfer_attempts,omitempty"` // 转存尝试次数（上限 3）
```

> **无需 DDL 迁移**：`PrivateData` 是 `gorm:"column:private_data;type:json"`，新增字段随 JSON 序列化自动生效。三库兼容（AGENTS.md Rule 2）自动满足。

---

## 5. 状态语义

| 场景 | `ResultURL` | `OssURL` | 返回用户 | 失败表 | 用户可用性 |
|---|---|---|---|---|---|
| 开关关闭 | 上游 URL | 空 | 上游 URL | 不写入 | ✅（受上游 TTL 限制） |
| 转存中 | 上游 URL | 空 | 上游 URL | 不写入 | ✅ |
| 转存成功 | 上游 URL（保留） | 对象存储地址 | **对象存储地址** | 不写入 | ✅ **不受 TTL 限制** |
| 转存失败 1 次 | 上游 URL | 空 | 上游 URL（经旧映射改写） | **写入 1 行**（`AttemptNo=1`） | ✅（降级） |
| 转存失败 2 次 | 上游 URL | 空 | 上游 URL（经旧映射改写） | **写入 2 行** | ✅（降级） |
| 转存失败 3 次（上限） | 上游 URL | 空 | 上游 URL（经旧映射改写） | **写入 3 行**，末行 `IsFinal=1` | ✅ 降级；上游超期后 ❌ |
| 转存失败且上游已超期 | 上游 URL | 空 | 上游 URL（失效） | 保留全部记录 | ❌ |

> 倒数第二行是本方案**刻意接受**的边界：重试 3 次仍失败则停止，用户仍可依赖上游 URL（直至其过期）。这是「避免无限重试刷表」与「尽力恢复」之间的折中。
>
> **替代方案（未采纳）**：若该风险不可接受，可改为「同步转存」——等待上传完成再落库。代价是轮询循环被大文件上传阻塞。当前选择异步 + 补偿，是吞吐与可靠性之间的折中。

**关键性质**：`ResultURL` **永不覆盖**。保留上游原址的价值：① 转存失败时仍可降级；② 人工对账时可回溯原始来源；③ 便于重试。

---

## 6. 风险与对策

| # | 风险 | 严重度 | 对策 |
|---|---|---|---|
| 1 | 转存失败导致结果不可用 | 高 | 保留 `ResultURL` 降级；失败仅记日志；补偿重试 |
| 2 | 轮询重复上传同一任务 | 中 | `OssURL` 非空即跳过 + `task_id` 作对象键（覆盖式） |
| 3 | 大文件打爆内存 | 高 | 强制流式，代码评审禁止 `io.ReadAll` |
| 4 | 阻塞轮询循环 | 高 | 后台 goroutine；转存与结算解耦 |
| 5 | 转存地址被错误改写 | **高** | `GetPreferredResultURL` + `OssURL == ""` 分支隔离（见 4.5） |
| 6 | AK/SK 泄露到前端 | 高 | 走含 `Secret` 的 option key 约定，自动过滤 |
| 7 | 进程重启丢失转存 | 中 | 补扫机制（4.4.2） |
| 8 | 三库 JSON 过滤方言差异 | 中 | 应用层过滤，不用 JSON 内部字段做 SQL 条件 |
| 9 | 上游返回错误页被当结果上传 | 中 | 校验 `Content-Type` 与 `Content-Length` |
| 10 | 大量积压任务同时上传打满带宽 | 中 | 单 tick 上限 20 条 + 上传超时 |
| 11 | 上游 URL 已过期仍重试 | 低 | 补扫时按 `expire_at` 过滤 |
| **12** | **MinIO 未开 `ForcePathStyle`，请求打错地址** | **高** | 默认 `true`；见 4.1.2 |
| **13** | **MinIO 内网地址用户访问不到** | **高** | 必须配反代域名并填入 `PublicBaseURL`；见 4.1.3 |
| **14** | **HTTP 地址被浏览器混合内容拦截** | **高** | 反代必须启用 HTTPS 证书 |
| **15** | **磁盘写满导致转存全部失败** | **高** | MinIO 侧配置磁盘告警；见 §8 部署检查清单 |
| **16** | **MinIO 单点故障** | 中 | 独立数据盘/RAID 已缓解；建议配置定期备份 |
| 17 | MinIO 开源版 AGPLv3 授权 | 低 | 自用部署无影响；若对外商业分发需法务确认 |
| **18** | **失败记录表增长** | 中 | 每次失败一行，但重试上限 3 次锁死单任务最多 3 行；二期加清理策略 |
| **19** | **并发写失败记录竞态** | 低 | 单任务转存串行；`OssURL` 非空即跳过 |

---

## 7. 文件改动清单

### 7.1 本期（Phase 1）

| 文件 | 类型 | 说明 |
|---|---|---|
| `pkg/oss/client.go` | 新增 | S3 客户端单例、配置完整性判定、`ForcePathStyle` 分支 |
| `pkg/oss/upload.go` | 新增 | 流式下载 + PutObject / multipart |
| `pkg/oss/key.go` | 新增 | 对象键生成与扩展名推断 |
| `pkg/oss/url.go` | 新增 | 公网 URL 拼接 |
| `pkg/oss/oss_test.go` | 新增 | 单元测试（键生成/URL 拼接/配置判定/path-style） |
| `setting/system_setting/oss.go` | 新增 | 全局配置 + 环境变量读取 |
| `service/task_oss_transfer.go` | 新增 | 转存判定、异步执行、失败记录写入 |
| `model/oss_transfer_failure.go` | 新增 | **失败记录表模型 + 查询/写入方法** |
| `model/main.go` | 修改 | **注册 `OssTransferFailure` 到 `AutoMigrate`** |
| `dto/channel_settings.go` | 修改 | +2 渠道开关（旧 3 项**保留不动**） |
| `model/task.go` | 修改 | +4 `PrivateData` 字段、`GetPreferredResultURL`、`GetPendingOssTransferTasks` |
| `service/task_polling.go` | 修改 | 终态触发转存 + 补扫分支（`ExpireAt` 逻辑保留） |
| `relay/relay_task.go` | 修改 | 2 处 builder 改用 `GetPreferredResultURL` + 改写隔离 |
| `go.mod` / `go.sum` | 修改 | +`aws-sdk-go-v2/service/s3`（与既有 aws-sdk 同源） |
| `web/default` 渠道配置表单 | 修改 | 2 个开关 UI（旧字段 UI **保留**） |
| `.env.example` | 修改 | 补充对象存储环境变量示例 |

### 7.2 二期（Phase 2，本期不实现）

| 内容 | 说明 |
|---|---|
| `GET /api/oss/transfer/stats` | 失败率、按阶段/类型分组统计 |
| `GET /api/oss/transfer/failures` | 失败明细分页查询 |
| 前端管理页面 | 失败列表 + 趋势图 |
| 失败记录清理策略 | 定期归档或删除历史记录 |

> **本期约束**：表结构必须一次定稿，二期新增 API **无需改表**。

---

## 8. MinIO 部署检查清单（宝塔环境）

> 本节为部署前置条件。**任一项未完成都会导致转存后用户无法访问。**

### 8.1 必须完成

| # | 检查项 | 说明 |
|---|---|---|
| 1 | 独立数据盘 / RAID | 已确认具备。MinIO 数据目录**必须**指向独立盘，不得放在系统盘 |
| 2 | 创建 bucket | 例如 `td-results`，与 `S3_BUCKET` 一致 |
| 3 | **设置 bucket 为公有读** | 本方案采用直出 URL，私有读会导致 403。MinIO 控制台 → Bucket → Access Policy → `public`（或自定义只读策略） |
| 4 | **配置反向代理** | 宝塔网站 → 反向代理 → 目标 `http://127.0.0.1:9000`，域名填入 `S3_PUBLIC_BASE_URL` |
| 5 | **启用 HTTPS** | 宝塔申请 Let's Encrypt 证书并开启强制 HTTPS。**否则浏览器混合内容拦截** |
| 6 | 生成 Access Key | MinIO 控制台创建 Access Key（非 root），填入 `S3_ACCESS_KEY_ID` / `S3_ACCESS_KEY_SECRET` |
| 7 | 校验 `ForcePathStyle=true` | 见 4.1.2，MinIO 的硬性要求 |

### 8.2 强烈建议

| # | 建议 | 理由 |
|---|---|---|
| 1 | 磁盘使用率告警 | 视频占空间增长快，写满会导致**所有**转存失败 |
| 2 | 定期备份 | 虽有 RAID，仍需防误删/勒索。可只备份元数据或异地同步 |
| 3 | 配置生命周期规则 | 按前缀（如 `td/`）设置过期删除，控制磁盘增长 |
| 4 | 确认 AGPLv3 合规 | 自用无影响；对外商业分发需法务确认 |

### 8.3 验证连通性

部署后先手工验证，再开启渠道开关：

```bash
# 1. 上传测试（需 aws-cli 或 MinIO 客户端 mc）
mc alias set local http://127.0.0.1:9000 <AK> <SK>
mc cp test.mp4 local/td-results/td/test.mp4

# 2. 公网访问测试（关键：验证反代 + HTTPS + 公有读）
curl -I https://<你的OSS域名>/td/test.mp4
# 预期：HTTP 200，Content-Type: video/mp4
```

**第 2 步返回 200 之前，不要开启渠道开关。**

---

## 9. 验证计划

### 9.1 单元测试

| 用例 | 断言 |
|---|---|
| 对象键生成 | 格式为 `{prefix}{date}/{task_id}.{ext}`；扩展名推断与兜底正确 |
| 公网 URL 拼接 | `PublicBaseURL` 优先；为空时回退虚拟主机风格 |
| 配置完整性判定 | 四要素任一缺失即 `IsConfigured() == false` |
| 转存判定 | 5 个前置条件各自的短路行为；开关关闭时不触发 |
| **path-style 分支** | `ForcePathStyle=true` 时请求 URL 形如 `{endpoint}/{bucket}/{key}`；`false` 时为 `{bucket}.{endpoint}/{key}` |
| **重试上限** | `OssTransferAttempts >= 3` 时不再触发转存 |
| **失败记录逐次写入** | 3 次失败产生 3 行，`AttemptNo` 分别为 1/2/3；第 3 行 `IsFinal=1` |
| **IsFinal 标记** | 仅最后一次失败（`AttemptNo=3`）标记 `IsFinal=1`，前两次为 0 |
| **失败阶段分类** | 下载失败记 `download`，上传失败记 `upload`，预检失败记 `precheck` |

### 9.2 集成验证

| 场景 | 预期 |
|---|---|
| 开关全关 | 行为与改动前一致，无对象存储调用；**旧映射方案仍生效** |
| 仅视频开关开 | 视频转存，图片不转存 |
| 仅图片开关开 | 图片转存，视频不转存 |
| 配置缺失 + 开关开 | 短路，不报错，返回上游 URL |
| 转存成功 | 返回对象存储地址；`OssURL`/`OssObjectKey`/`OssUploadedAt` 正确落库 |
| 转存失败（伪造无效 endpoint） | 任务仍 SUCCESS、计费正常、返回上游 URL、**失败表新增 1 行（`AttemptNo=1`）** |
| **失败降级仍走旧映射** | 转存失败时，返回的仍是经 `TdUrlProxyBaseURL` 改写后的上游地址 |
| **重试计数** | 连续触发轮询，`OssTransferAttempts` 递增至 3 后停止重试 |
| **失败表逐次累积** | 重试 3 次后，该 `task_id` 在表中**有 3 行**，`AttemptNo` 为 1/2/3，仅第 3 行 `IsFinal=1` |
| **重试原因可追溯** | 3 行的 `fail_reason` 可各不相同，能看出失败根因的演变 |
| 重启补偿 | 手工置空 `oss_url` 后触发轮询，能重新转存 |
| 重复轮询 | 不产生重复上传 |
| **转存地址不被改写** | 开启 `TdUrlProxyBaseURL` 后，转存地址**保持原样** |
| 上游 URL 超期 | 补扫**不再**重试该任务 |
| **大文件** | 上传 200MB+ 视频，进程内存无显著增长（验证流式） |
| **公网可播放** | 转存后的地址在浏览器中可直接播放，无混合内容报错 |

### 9.3 三库验证

在 SQLite / MySQL / PostgreSQL 上各跑一次任务生命周期，确认：

1. `private_data` 读写正常
2. 补扫查询**未**使用 JSON 内部字段做 SQL 条件
3. **`oss_transfer_failures` 表由 `AutoMigrate` 正确创建**，索引生效
4. 失败记录的 INSERT / UPDATE 在三库上行为一致

---

## 10. 待实现阶段确认的细节

以下问题不影响架构，但编码前应定稿：

1. **图片/视频判定的数据来源**：轮询侧能否直接拿到原始响应体？若不能，退化为按 URL 扩展名 / `Content-Type` 判定（需明确注释取舍）。
2. **上传体积上限**：建议默认 2 GB，是否需要按渠道可配？
3. **生命周期规则**：MinIO 侧按前缀（如 `td/`）设置过期删除策略的具体天数？建议由运营决定，文档给出配置位置即可。
4. **前端开关位置**：Td 渠道配置表单中，紧邻现有 `TdUrlTTLHours` / `TdUrlProxyBaseURL` 放置（**旧字段 UI 保留**）。
5. **配置命名**：已统一采用 `S3_*` 前缀（语义准确，不与阿里云强绑定）。代码内的字段名仍保留 `OssURL` 等既有称谓，若需彻底中性化可改名 `StorageURL`。
6. **失败表是否记录成功任务**：当前设计**只记失败**。若二期需要精确的「失败率分母」，需额外统计成功转存数（可从 `tasks` 表按 `oss_url` 非空计数，无需改表）。

---

## 11. 决策记录（变更历史）

| 日期 | 决策 | 说明 |
|---|---|---|
| 10-13 | 存储后端：阿里云 OSS | 初始方案 |
| 10-13 | 触发时机：终态异步转存 | 不阻断结算 |
| 10-13 | 兼容性：失败降级不阻断 | 保留上游 URL |
| 10-13 | 范围：仅 Td 渠道视频 | 初始范围 |
| 10-13 | URL 形式：公有读直出 | 非签名 URL |
| 10-14 | **新增 Td 图片结果转存**（2 个独立开关） | 原仅视频 |
| 10-14 | **存储后端改为 MinIO**（宝塔部署） | 利用本地独立数据盘 |
| 10-14 | **SDK 改为 S3 协议** | 同时兼容 MinIO 与阿里云 OSS |
| 10-14 | **配置前缀 `S3_*`** | 语义中性 |
| 10-14 | **重启补偿：补扫机制** | 未转存且未超期的任务重试 |
| 10-14 | **不回溯历史任务** | 仅对新任务生效 |
| 10-14 | **旧映射配置保留但不使用** | 不删除，避免存量配置丢失 |
| 10-14 | **新增转存失败记录表** | 每次失败一行，含重试 |
| 10-14 | **失败率查看延后二期** | 本期仅建表，可用 SQL 查询 |
| 10-14 | **重试上限 3 次** | 超过则停止，避免刷表 |
