# 备用模型兜底（Fallback Model）契约

## 需求

1. **API 密钥级开关**：新增 `fallback_model_enabled`，默认关闭。绝大多数模型不需要配置。
2. **系统级备用模型映射**：进入休眠池的模型可配置备用模型，形如 `原模型=备用模型`。
3. **兜底触发条件**（三个条件同时满足）：
   - 原模型的**全部渠道**都在过载休眠池中；
   - 请求所用 API 密钥开启「使用相近模型兜底」；
   - 该系统配置了该原模型的备用模型。
   → 此时把请求改走**备用模型自己的渠道**。
4. **禁止死循环**：
   - 配置层面允许 `3.6=3.7` 且 `3.7=3.6`（运维可以这样配）；
   - 运行时必须检测并中断；
   - 渠道层面：走备用模型 B 时，若某渠道的 ModelMapping 把 B 映射回原模型 A，说明绕回原模型 → **跳过该渠道**，换下一个渠道尝试；
   - 兜底只做一次，禁止 A→B→C 无限链。

## 配置项（运维 -> 系统行为，最底部）

| 配置键 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `overload_sleep_setting.enabled` | bool | false | 总开关 |
| `overload_sleep_setting.sleep_minutes` | int | 2 | 休眠分钟 |
| `overload_sleep_setting.status_codes` | string | "429,503" | 过载状态码 |
| `overload_sleep_setting.fallback_models` | string | "" | 备用模型，每行 `原模型=备用模型` |

`fallback_models` 解析规则：
- 每行一条，忽略空行与 `#` 注释行；
- 用第一个 `=` 切分，两侧去空白；
- 两侧都非空、且不相等才生效（自映射被丢弃）；
- 同一原模型重复配置时后者覆盖前者。

## 密钥字段

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `fallback_model_enabled` | bool | false | 使用相近模型兜底 |

- DB 列通过 `ensureTokenFallbackModelEnabledColumn()` 增量迁移（SQLite/MySQL/PostgreSQL 通用，走 GORM Migrator，不写死 SQL）。
- `Token.Update()` 的 `Select(...)` 必须包含该字段，否则保存不生效。
- `middleware/auth.go` 需写入 context：`ContextKeyTokenFallbackModelEnabled`。

## 运行语义

### 触发流程

```
请求模型 A
  └─ 选择 A 的渠道
       ├─ 有可用渠道 → 正常请求（不触发兜底）
       └─ 全部在休眠池（AllSleeping）
            ├─ 未开总开关 / 密钥未开兜底 / 未配 A 的备用模型 → 走原兜底（A 的最后一个渠道）
            ├─ 配置存在环（A=B 且 B=A） → 不切换，走原兜底，记日志
            └─ 有备用模型 B
                 ├─ 取 B 的候选渠道，剔除「B 被渠道映射回 A」的渠道
                 ├─ 剔除后仍有渠道 → 切换：上下文模型名改为 B，用 B 的渠道请求
                 └─ 剔除后无渠道 → 不切换，走原兜底，记日志说明
```

### 循环防护要点

- **配置环**：`DescribeFallbackModelCycle` 检测，非空即放弃切换（避免 A/B 来回）。
- **渠道映射环**：`ChannelModelMappingReturnsToOrigin` 做链式解析（含 visited 集合防自身死循环），
  判定「备用模型经该渠道映射后最终是否落到原模型」，是则跳过该渠道。
- **二次兜底**：`CurrentFallbackOriginModel` 非空即不再兜底。

## 日志

- 兜底切换、循环跳过均写 `logger.LogInfo`，前缀统一便于 grep：`备用模型兜底：`。
- 日志 `other` 字段仅在真实发生跳过/兜底时写入，正常请求不增加任何字段。

## 不变式（review 必查）

1. 未开启兜底时行为与之前**完全一致**，无额外 Redis/DB 开销。
2. 任何路径都必须返回一个渠道或明确错误，**不得返回 nil channel**。
3. 兜底失败必须回退到「原模型的最后渠道」兜底，保证「不能一个都不请求」。
4. 所有 JSON 读写走 `common.Marshal/Unmarshal`，不得直接用 `encoding/json`。
5. 迁移代码必须三库兼容，不得写死 SQL 方言。
