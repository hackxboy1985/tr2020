# n-axis 接入待办

> 更新时间：2026-11-13 20:17:35.495 UTC+8
> 用途：记录 n-axis 接入的决策、已完成项与待办，每次对话末尾会附「待办提醒」，直到全部关闭。

## 一、决策记录

| # | 决策 | 说明 |
|---|---|---|
| D1 | **素材库 / 人脸核验 / 视频统一走 n-axis 火山方舟原生 Action 入口** | 上游请求形如 `POST https://platform.n-axis.com/?Action=…&Version=2024-01-01`。渠道配置 `asset_upstream_version: "naxis"`（默认即 Action）。 |
| D2 | **不走平台 `/v1` 入口** | `"naxis_rest"` 仅作为回滚开关保留在代码里，不配置就不会触发；确认稳定后可以删除该分支。 |
| D3 | 视频走 ark 原生路径 | `/api/v3/contents/generations/tasks`（`doubao_video_*_path` 显式配置）。 |
| D4 | 下游两套接口不变 | Action 入口（`/api/seedance/assets/v2/?Action=`）与 `/api/seedance/*` 的字段/结构/状态码均保持原样。 |
| D5 | 不新增数据库字段 | Action 返回的签名代理 URL 不入库、不返回下游；原始响应存已有 `raw_data`。 |
| D6 | 跨渠道自动同步**保留素材组结构**（方案 B） | 补登记时按「组名 + 组类型」在当前渠道找同名组，没有则建同名组；素材落到对应组，不一律塞默认组。仅 AIGC 适用。 |
| D7 | 自动补登记默认**关闭** | 新增渠道配置 `asset_auto_sync`（默认 false）。打开后才会在解析 `asset://` 时自动补登记；关闭时缺绑定直接返回"需重新登记"。 |
| D8 | LivenessFace 换渠道不可自动同步 | 跨上游账号无法复用真人核验组；缺绑定时返回明确错误，提示用户在新渠道重新核验/重新生成。 |
| D9 | 成片地址校验口径 | 校验的是**返回给下游的成片 URL**，不是上游请求地址；只要 http/https 绝对链接即可，**不强制 https**，所有上游同一口径。 |
| D10 | 渠道配置「是否允许返回非官链」 | `ChannelOtherSettings.allow_non_official_video_url`，默认 `false`。官链判定：**host 以 `.volces.com` 结尾**（TOS 直链，带 X-Tos 签名参数也算官链）；该判定**只对 n-axis 上游强制**，老咪咕 gateway / sdos2 保持原行为。关闭时非官链（相对路径/非 volces 域名）→ `fail_code=NON_OFFICIAL_VIDEO_URL` + 任务失败；打开时相对路径用渠道 Base URL 拼接，并代理拉流。 |
| D10-b | 匿名播放配置 | 复用 `sdos2_video_content_public`：`false`（默认）代理下载需下游带 token；`true` 允许匿名播放（浏览器 `<video>` 场景）。 |
| D12 | 成片代理地址基址 | 复用现有逻辑：渠道 `sdos2_relay_base_url` 优先，其次系统 `ServerAddress`；不新增配置项。 |
| D13 | 匿名播放 | 复用现有 `sdos2_video_content_public` 开关（名字历史遗留，不新增字段）；打开后 `NeedsVideoContentProxy` 的渠道均允许匿名拉流。 |
| D14 | 允许非官链的渠道**统一代理** | 只要开关打开，下游拿到的 URL 一律是本服务 `/v1/videos/{task_id}/content`（即使上游给的是公开绝对地址），保证行为确定；代价是成片流量走本服务。 |
| D11 | 允许非官链的渠道**必须同时提供成片代理** | 打开开关的渠道，非官链的成片地址替换为本服务的 `/v1/videos/{task_id}/content`；下游调用时由后端按"同格式"请求上游（带上游 Key、透传 Range/状态/头、流式回传）。复用 `VideoProxy` / `VideoProxyPublic` / `BuildVideoContentURL`。 |
| D11-b | 逐 URL 判定（不是按渠道一刀切） | 每条任务按**它自己的成片地址**决定是否代理：官链（`.volces.com`）→ 下游直连不代理（省带宽）；非官链 → 渠道允许时代理；sdos2 受保护地址 → 总是代理。 |

## 二、视频成片地址策略（修正版）

目标：下游拿到的成片地址必须是**可直接访问的绝对地址**，坏地址不能让用户拿到。

分两层，逐层加严：

| 层 | 规则 | 状态 |
|---|---|---|
| 第 1 层（底线） | 返回的成片地址必须是一个**绝对 http(s) 链接**（http、https 均可，**不强制 https**）。空值、相对路径（如 `/v1/videos/xxx/content` 没拼出 baseURL）、非 http(s) 协议、缺 host → **记 error 日志 + 任务置 failed**，不透传下游。 | ✅ 已实现 |
| 第 2 层（非官链拒绝） | 只接受官方域名（TOS 直链），拒绝「平台签名代理直链」 | ⏳ 待定，缺判定规则 |

第 1 层实现位置：`relay/channel/task/doubao/adaptor.go`
- `validateVideoResultURL()` / `applyResultURLValidation()`
- 日志关键字：`[Doubao Video] invalid result url rejected: task_id=… model=… upstream_video_url=… err=…`
- 失败任务 `status=failed`，`reason=invalid video url from upstream: <原因>`

**已确认**：用户澄清"没有 https 开头肯定是错的"指**返回的成片地址**而非上游请求地址；不强制 https，只要求是绝对链接而不是相对路径。**所有上游统一口径，不再区分 n-axis。**

## 三、已完成

- [x] Action 素材适配器：建组/查组/改组/删组、建素材/查素材/改素材/删素材（8 个方法），创建后自动回查补全对象
- [x] Action 人脸核验：`CreateVisualValidateSession` / `GetVisualValidateResult`，含 30 分钟有效期、snake/Pascal 双键解析、跨入口 token 互通
- [x] 平台 REST 适配器（回滚用，默认不启用）
- [x] controller 人脸核验分支（老 gateway 逻辑零改动 + 守护测试；下游返回结构对齐历史，GET 补回 `expires_at`）
- [x] `ast_` / `asset-` 双前缀、无 `fv_` 前缀的人脸 token 兜底
- [x] 成片地址第 1 层校验（绝对 http(s) 链接 → 否则失败；不强制 https）
- [x] 单测：service 18 + controller 1 + doubao 视频 5 组；真机验证：素材 8 操作、人脸跨入口
- [x] 配置文档（渠道类型 54 / Base URL / key / 上游路径配置 JSON）
- [x] 视频异常任务标记：`allow_non_official_video_url` 渠道开关 + `Task.FailCode`（索引）+ `TaskDto.fail_code`（T11）
- [x] 非官链渠道成片代理：下游 URL 改写、带上游 Key 拉流、Range/206（T12）

## 四、当前任务

| # | 任务 | 状态 | 要点 |
|---|---|---|---|
| T9 | 跨渠道资产库**自动同步** | ✅ 代码完成（未提交） | 渠道配置 `asset_auto_sync`（默认关）；跨渠道查询；`EnsureAssetForChannel` 惰性补登记（方案 B 组结构、LivenessFace 报错、幂等）；视频提交 `asset://` 改写 + 约定错误结构；controller GET/PATCH/DELETE 翻译（跨渠道删除只删本地）；列表按当前渠道去重。已知限制：并发无唯一约束兜底（依赖上游幂等）、列表 total 未按去重后重算 |
| T9-a | 资产失效/同步失败的错误契约 | 已确认格式 | 任务提交接口返回 HTTP 400 与扁平结构：`{"code":"InvalidParameter","message":"asset not ready (status=failed): <asset-id>","param":null,"type":"BadRequest"}`；上游细节只记日志不透传。注意：AIGC 平台不预检 source_url，URL 已死只能在上游提交视频时暴露，届时同样映射为该结构 |
| T10 | 放宽同上游账号的 `channel_id` 过滤 | 并入 T9 | 同账号换渠道行时旧素材查询不再 404 |
| T11 | 视频异常任务标记（方案 C） | ✅ 已实现（未提交，测试通过） | `Task.FailCode varchar(64) index` + `TaskDto.fail_code`；三个码：`INVALID_VIDEO_URL` / `CHANNEL_BASE_URL_MISSING` / `NON_OFFICIAL_VIDEO_URL`（**不需要新增字段**，只是列的新取值）；轮询落库，日志含 fail_code |
| T12 | 非官链渠道成片代理 | ✅ 已实现（未提交，测试通过） | 逐 URL 判定（官链直连 / 非官链代理 / sdos2 总代理）；下游 URL 改写（OpenAI/Ark 两种响应）；带上游 Key 拉流；Range/If-Range 透传、支持 206；匿名播放沿用 `sdos2_video_content_public` |

## 五、归档（不再提醒，需要时再重启）

| # | 事项 | 结论 |
|---|---|---|
| T1 | 视频成片只认官方域名（第 2 层） | 缺判定规则，暂不做；当前只做第 1 层（绝对 http(s) 链接） |
| T2 | 是否所有上游一律 https | ✅ 已定：不强制，http/https 均可（见 D9） |
| T3 | 视频真机端到端提交 | 会产生费用，待授权；暂不做 |
| T4 | LivenessFace 真机扫码闭环 | 行为已定（D8 报错提示重新核验），扫码验证不做 |
| T5 | 前端渠道配置 UI | 暂不做 |
| T6 | 代码提交 git | ✅ `1df153798` 已推送 `dev_mints` |
| T7 | n-axis 测试 key 轮换 | 由你自行处理 |
| T8 | 删除 `naxis_rest` 回滚分支 | 稳定运行后再定 |

## 六、提醒约定

- 每次回复末尾的「待办提醒」只列**当前任务**，归档项不再逐条提醒。
- 本文件随进展更新。
