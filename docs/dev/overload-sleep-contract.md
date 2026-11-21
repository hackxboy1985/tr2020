# 过载休眠池（Overload Sleep Pool）接口契约 v1

> 本文件由 Lead 编写，作为前后端并行开发的唯一契约。后端实现以本文件为准，前端严格按本文件对接。
>
> **实现状态（最终）：已实现并自测通过。** 后端：`setting/operation_setting/overload_sleep_setting.go`、
> `pkg/sleeppool/`、`model/channel_select_exclude.go`、`model/channel_lookup.go`、
> `model/overload_sleep_log.go`、`service/overload_sleep_pool.go`、`service/channel_select.go`、
> `controller/channel_sleep_pool.go`、`controller/relay.go`、`middleware/distributor.go`、
> `controller/option.go`、`router/api-router.go`。
> 前端：`web/classic/**`（经典主题）与 `web/default/**`（新版主题）。
>
> 与初版契约的差异（以实际实现为准）：
> - `GET /api/channel_sleep_pool/` 的 `data` 额外返回 `sleep_minutes`、`status_codes`、`server_time`，供页面只读展示。
> - `entries[]` 额外返回 `group` 字段。
> - 配置项存储位置：数据库 options 表（经 `setting/config` 注册，随 `/api/option/` 读写），**不是** Redis；
>   只有「休眠池条目」用 Redis，符合「不新增表、休眠池仅 Redis」的要求。

## 1. 需求摘要

1. 上游返回「过载」状态码（可配置，默认 429、503）时，把 **模型 + 渠道** 一起放入 Redis 休眠池，休眠时长可配置（默认 2 分钟）。
2. 之后每次请求，用当前模型去休眠池里找：命中的渠道**跳过**，直接走下一渠道；同时**在日志表写一条「跳过」记录**（避免误认为是 bug）。
3. 若该模型的所有渠道都在休眠池中，则**取最后一个渠道**强行尝试（不能一个都不请求）。
4. 「运维 -> 系统行为」最下方新增配置项（总开关 / 休眠时长 / 过载状态码），保存到数据库配置（注册进 setting/config，随 /api/option/ 一起读写）。
5. 左侧菜单「任务日志」下面新增「休眠模型」菜单，**仅管理员可见**，普通用户绝对不可见。
6. 「休眠模型」页面：展示休眠池；支持按模型名、按渠道筛选；逐行展示 (模型, 渠道) 限制；可选中某行「从休眠池移除」；另有「全部清除」按钮；页面上还要展示总开关状态（只读，不可修改）。
7. 不新增数据库表，休眠池只用 Redis。

## 2. 配置项（走现有 /api/option/）

| option key | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `overload_sleep_setting.enabled` | bool | `false` | 总开关，关闭时整套流程不生效（不写池、不跳过、不写跳过日志） |
| `overload_sleep_setting.sleep_minutes` | int | `2` | 休眠分钟数，范围 1-1440，非法值回落默认 2 |
| `overload_sleep_setting.status_codes` | string | `"429,503"` | 过载状态码，逗号分隔，支持区间如 `429,500-599`；服务端会校验，非法时 PUT /api/option/ 返回 success=false |

读取系统配置：`GET /api/option/`（RootAuth）。更新：`PUT /api/option/` body `{key, value}`（RootAuth）。
注意：「系统行为」页面本身只有 root 能保存，与现有页面一致。

## 3. 休眠池查询/操作接口（新增，AdminAuth = 角色 >= 10）

统一前缀：`/api/channel_sleep_pool`

### 3.1 查询
`GET /api/channel_sleep_pool/`  → 200

```json
{
  "success": true,
  "message": "",
  "data": {
    "enabled": true,
    "sleep_minutes": 2,
    "status_codes": "429,503",
    "redis_enabled": true,
    "server_time": 1763686800,
    "entries": [
      {
        "model_name": "gpt-4o",
        "channel_id": 12,
        "channel_name": "openai-1",
        "channel_type": 1,
        "sleep_until": 1763686920,
        "remaining_seconds": 96,
        "trigger_status_code": 429,
        "trigger_count": 3,
        "total_channel_count": 4,
        "all_channels_sleeping": false,
        "next_channels": [
          { "channel_id": 15, "channel_name": "openai-4", "priority": 10 },
          { "channel_id": 19, "channel_name": "openai-9", "priority": 0 }
        ],
        "fallback_channel": null
      }
    ]
  }
}
```

字段说明：
- `enabled` 总开关（只读展示）；`sleep_minutes`、`status_codes` 当前生效配置（只读展示）；`redis_enabled` Redis 是否可用；`server_time` 服务器当前秒级时间戳。
- `entries` 按 `model_name` 升序、`channel_id` 升序排序，只含未过期条目。
- `remaining_seconds` 剩余秒数（>0）。
- `total_channel_count` 该模型当前可用渠道总数；`all_channels_sleeping` 为 true 表示该模型所有渠道都在休眠。
- `next_channels` 跳过休眠渠道后会走到的候选渠道（最多 5 个，按 priority 降序、channel_id 升序）。`all_channels_sleeping=true` 时为空数组。
- `fallback_channel` 仅当 `all_channels_sleeping=true` 时非 null，表示「取最后一个渠道」实际会用的渠道；否则为 null。
  「最后一个渠道」的判定与请求时兜底逻辑一致：候选列表末位，即**优先级最低、同优先级内 channel_id 最大**。
- `group` 该条休眠记录写入时的使用分组（auto 分组会记录实际选中的分组），用于回溯「跳过休眠后会走哪个渠道」时按正确的分组计算。

> 实现说明（Redis 存储结构，仅用于运维排查）：
>
> 每个 **(模型, 渠道)** 一个独立 key：
>
> ```
> overload_sleep_pool:model:<模型名>:channel:<渠道ID>  ->  STRING
>   value = JSON{ model_name, channel_id, channel_name, channel_type, group,
>                 sleep_until, trigger_status_code, trigger_count, created_at, updated_at }
>   TTL   = 休眠时长 + 2 分钟宽限
> ```
>
> 为什么不用「按模型一个 HASH」：
> - Redis 的 TTL 只能作用于整个 key，**无法给 HASH 的单个 field 设 TTL**；
> - 若按模型共用一个 HASH，任何渠道刷新 key 的 TTL 都会影响同模型下其它渠道的到期时间。
>   实测会把休眠时长调小后新进入渠道的 TTL 覆盖上去，导致仍在休眠期的老渠道被**提前恢复**。
>   改用每渠道独立 key 后，TTL 由 Redis 原生保证相互独立（有回归测试
>   `TestEachChannelHasIndependentTTL`、`TestKeyTTLNotShorterThanSleepUntil` 覆盖）。
>
> 安全不变式：**key 的物理 TTL 永远 >= 条目自身的 `sleep_until`**，
> 因此不会出现「条目还没睡够、key 先没了」而被提前恢复的情况。
>
> 热路径代价控制：请求时不会 SCAN 全库，而是先用内存缓存拿到该 (分组, 模型) 的候选渠道，
> 再对候选渠道做一次 **MGET**，固定 1 次 Redis 往返，不随休眠池规模增长变慢。
> SCAN 只用于管理端「列出全部 / 按模型列出」这类低频路径。
>
> 写入同一 (模型, 渠道) 会续期并累加 `trigger_count`；
> `trigger_status_code` 记录该渠道因哪个上游状态码进入休眠。

## 3.4 请求时的跳过与兜底规则（实现细节）

1. 总开关关闭 / Redis 不可用 → 完全不介入，行为与改造前一致。
2. 命中休眠渠道 → 跳过，继续当前优先级内其它渠道，再向更低优先级寻找；被跳过的渠道写入日志表。
3. 该模型**全部**渠道都在休眠 → 取「最后一个渠道」强行尝试，并写一条 `overload_sleep_all_sleeping` 日志。
4. 渠道亲和（channel affinity）指定的优先渠道若正在休眠，本次不使用亲和渠道，改走上述正常选择逻辑并记录跳过。
5. 返回上游错误后，若 `newAPIError.StatusCode` 命中 `status_codes` 配置 → 把该 (模型, 渠道) 放入休眠池；
   文本中继（`Relay`）与任务中继（`RelayTask`）均已接入。
6. 同一请求内同一渠道只记录一次跳过日志。

支持 `?model_name=<模型名>` 按模型精确检索（key 前缀天然支持），
不传则返回全部。条目额外含 `trigger_status_code`（因哪个错误码进入休眠）。

### 3.2 移除单条
`DELETE /api/channel_sleep_pool/?model_name=<模型名>&channel_id=<渠道ID>` → 200

```json
{ "success": true, "message": "", "data": { "removed": 1 } }
```

参数缺失/非法 → 400 `{"success": false, "message": "..."}`。

### 3.3 全部清除
`DELETE /api/channel_sleep_pool/?all=true` → 200

```json
{ "success": true, "message": "", "data": { "removed": 7 } }
```

## 4. 跳过日志记录（写入 logs 表，无需前端展示新逻辑）

- 类型：`LogTypeSystem(4)`
- content：`渠道 #<id>（<name>）命中过载休眠池，已跳过（模型 <model>，剩余 <n> 秒）`
- other.admin_info 中带：`overload_sleep_skip: true`、`skip_channel_id`、`skip_channel_name`、`skip_channel_type`、`skip_remaining_seconds`、`skip_reason`
- 同一请求内同一渠道只写一条（去重）。

## 5. 前端落地位置

### 5.1 classic 主题（当前默认主题，必须做）
- 配置项：`web/classic/src/components/settings/OperationSetting.jsx` **最下方**新增卡片（放在「签到设置」之后）。
- 菜单：`web/classic/src/components/layout/SiderBar.jsx` 的 `workspaceItems` 里，「任务日志」之后新增 `休眠模型`，仅 `isAdmin()` 可见。
- 页面：`web/classic/src/App.jsx` 加路由（如 `/overload_sleep`）+ 新页面组件；页面内四个区域：
  1. 只读状态条：总开关（开/关，不可编辑）、休眠时长、过载状态码、Redis 是否可用。
  2. 筛选区：模型名输入 + 渠道（渠道名称/ID 输入或下拉）。
  3. 表格：模型名、渠道ID/名称、渠道类型、触发状态码、触发次数、剩余秒数（可倒计时）、跳过后的目标渠道（`next_channels` 或「全部休眠 -> fallback」）。
  4. 行选择 + 「从休眠池移除」按钮；右上「全部清除」按钮（二次确认）。空态、加载态、错误提示、刷新按钮。
- i18n：`web/classic/src/i18n/locales/{en,zh,zh-CN,zh-TW,fr,ja,ru,vi}.json` 全部补齐。

### 5.2 default 主题（`web/default`，也做，保证切主题后功能存在）
- 配置项：`web/default/src/features/system-settings/general/system-behavior-section.tsx` **最下方**新增（zod schema + `useUpdateOption` 保存）；同时补 `types.ts` 的 `OperationsSettings`、`operations/index.tsx` 的默认值、`operations/section-registry.tsx` 的 defaultValues。
- 菜单：`web/default/src/hooks/use-sidebar-data.ts` 的 `general` 组里「Task Logs」之后新增「休眠模型」，并在 layout 的 item 类型/侧边栏过滤逻辑里支持 **item 级 adminOnly**（普通用户绝对不可见）。
- 页面：新增路由 `web/default/src/routes/_authenticated/channel-sleep-pool/index.tsx`（`beforeLoad` 校验 role < ROLE.ADMIN 则 redirect /403），feature 目录 `web/default/src/features/channel-sleep-pool/`（api.ts/types.ts/index.tsx/components/**），表格 + 筛选 + 行操作 + 全部清除，样式与现有 feature 一致。
- i18n：`web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json` 全部补齐。

## 6. 约定

- 前端所有存量文案一律 `t('English Source String')`，新增 key 用英文源文案。
- 后端用 `common.Marshal/Unmarshal`，不要直接 import encoding/json 做序列化。
- 禁止提交 git。
