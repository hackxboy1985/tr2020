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

## 二、视频成片地址策略（修正版）

目标：下游拿到的成片地址必须是**可直接访问的绝对地址**，坏地址不能让用户拿到。

分两层，逐层加严：

| 层 | 规则 | 状态 |
|---|---|---|
| 第 1 层（底线） | 地址必须存在且是绝对 URL；n-axis 上游额外要求 `https://` 开头。空值、相对路径（如 `/v1/videos/xxx/content` 没拼出 baseURL）、非 http(s) 协议、缺 host → **记 error 日志 + 任务置 failed**，不透传下游。 | ✅ 已实现（本次） |
| 第 2 层（非官链拒绝） | 只接受官方域名（TOS 直链），拒绝「平台签名代理直链」 | ⏳ 待定，缺判定规则 |

第 1 层实现位置：`relay/channel/task/doubao/adaptor.go`
- `validateVideoResultURL()` / `applyResultURLValidation()`
- 日志关键字：`[Doubao Video] invalid result url rejected: task_id=… model=… upstream_video_url=… err=…`
- 失败任务 `status=failed`，`reason=invalid video url from upstream: <原因>`

**待确认**：目前 `https` 强制只对 n-axis 上游生效，其他上游（如 sdos2 的 seedance-gateway 可能仍是 http）只要求绝对 http(s)。如果要求**所有上游一律 https**，需要改一行默认值 —— 请确认。

## 三、已完成

- [x] Action 素材适配器：建组/查组/改组/删组、建素材/查素材/改素材/删素材（8 个方法），创建后自动回查补全对象
- [x] Action 人脸核验：`CreateVisualValidateSession` / `GetVisualValidateResult`，含 30 分钟有效期、snake/Pascal 双键解析、跨入口 token 互通
- [x] 平台 REST 适配器（回滚用，默认不启用）
- [x] controller 人脸核验分支（老 gateway 逻辑零改动 + 守护测试；下游返回结构对齐历史，GET 补回 `expires_at`）
- [x] `ast_` / `asset-` 双前缀、无 `fv_` 前缀的人脸 token 兜底
- [x] 成片地址第 1 层校验（https / 绝对地址 → 否则失败）
- [x] 单测：service 18 + controller 1 + doubao 视频 5 组；真机验证：素材 8 操作、人脸跨入口
- [x] 配置文档（渠道类型 54 / Base URL / key / 上游路径配置 JSON）

## 四、待办

| # | 待办 | 阻塞条件 | 备注 |
|---|---|---|---|
| T1 | 视频成片地址**第 2 层**：只认官方域名，非官链报错并置失败 | 需要你提供两条真实 URL 样例（官方 TOS 直链 / 平台签名直链），或直接给域名白名单 | 当前只做到第 1 层 |
| T2 | 确认是否「所有上游一律 https」（而非仅 n-axis） | 你的确认 | 改一行即可 |
| T3 | 视频真机端到端提交（提交→轮询→成片下载） | 需要授权费用 + 模型/时长/分辨率 | 会产生上游费用 |
| T4 | LivenessFace 真机闭环（扫码 → verified → GroupId → 素材 Active） | 需要人工扫码配合 | 代码路径已实现，只差真实核验数据 |
| T5 | 前端渠道配置 UI（当前靠「上游路径配置」JSON） | 你确认是否要做 | 不做也能用 |
| T6 | 代码提交 git | 你下指令 | 按约定不主动提交/推送 |
| T7 | n-axis 测试 key 轮换 | 你操作 | key 已出现在会话记录中 |
| T8 | 可选：删除 `naxis_rest` 回滚分支 | 稳定运行一段时间后 | 减少维护面 |

## 五、提醒约定

- 每次回复末尾附「待办提醒」，列出未关闭项，直到全部完成。
- 本文件随进展更新。
