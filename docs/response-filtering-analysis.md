# 响应内容过滤 — 可行性、可靠性、性能分析

## 背景

当上游 API 返回的响应中携带源头信息（如模型名、provider 标识等），需要在 new-api 层进行拦截替换，向下游客户端隐藏上游身份。

## 一、非流式响应：完全可行，几乎零开销

```
读取完整响应 → 修改内容 → 发送给客户端
```

核心路径：`relay/channel/openai/relay-openai.go:192-295` (`OpenaiHandler`)

- 响应 body 已全部加载在内存中（`responseBody []byte`，第 195 行）
- 替换操作是一次性的，毫秒级
- **关键词匹配**：AC 自动机（已有现成实现 `service/sensitive.go:40-48`），时间复杂度 O(n)，n 为 body 长度
- **JSON 字段级替换**（如改写 `model` 字段）：`unmarshal → 改字段 → marshal`，典型响应 < 10KB，开销可忽略

**结论：非流式 100% 可靠，延迟增加 < 1ms。**

---

## 二、流式响应：核心难点在 SSE 分片

流式是 SSE 逐块处理的：

```
上游返回 → bufio.Scanner 按行切割 → dataHandler 回调 → sendStreamData → 客户端
```

核心路径：`relay/channel/openai/relay-openai.go:106-190` (`OaiStreamHandler`)

### 场景 A：替换 JSON 字段值（如 model 名）

`sendStreamData`（第 34-35 行）已对每个 chunk 做 `UnmarshalJsonStr`：

```go
var lastStreamResponse dto.ChatCompletionsStreamResponse
common.UnmarshalJsonStr(data, &lastStreamResponse)
```

在此之后修改 `lastStreamResponse.Model`，再 marshal 发出：

| 指标 | 值 |
|------|-----|
| 单 chunk marshal + unmarshal | ~0.01ms |
| 1000 chunk 完整响应累计 CPU | ~10ms |
| 用户感知延迟 | 无（处理与生成并行） |
| 可靠性 | 高 — model 是完整 JSON 字段，不会被分片切断 |

### 场景 B：替换内容中的关键词（如"我是 OpenAI" → "我是 XCODE"）

**可靠性风险：** SSE 流可能把关键词拦腰截断：

```
chunk 1: data: {"choices":[{"delta":{"content":"我是 Open"}}]}
chunk 2: data: {"choices":[{"delta":{"content":"AI 助手"}}]}
```

"OpenAI" 被切断，逐块匹配永远匹配不到。

**解法：滑动窗口缓冲**

```
原则：不立即发送，缓冲最后 N 个字符，N = 最长关键词长度
```

1. 维护内容缓冲区，累积每个 chunk 的 text content
2. 保留最后 `maxKeywordLen` 个字符不发送
3. 在缓冲区中对完整文本做关键词匹配替换
4. 只发送已验证安全的部分

**对性能的影响：**

| 关键词最大长度 | 缓冲延迟 | 用户体验 |
|:---:|:---:|------|
| 5 字符（"OpenAI"） | ~5 字节 | 不可感知 |
| 20 字符 | ~20 字节 | 不可感知 |
| 中文（"我是OpenAI助手"） | ~8 汉字 | 不可感知 |

用户感知延迟 ≈ 最长关键词字节数 / 流式输出速度。例如 5 字节 / 50 tokens/s ≈ 0。流式输出本身是一个 token 一个 token 出来，延迟几个字符在 flush 间隔中完全被掩盖。

---

## 三、总体评估

| 场景 | 可靠性 | 延迟影响 | 改动量 |
|------|:---:|------|:---:|
| 非流式: 字段替换（model 名） | 100% | < 1ms | 插入现有 unmarshal 之后 |
| 非流式: 内容关键词 | 100% | < 1ms | AC 自动机已有，接入即可 |
| 流式: 字段替换（model 名） | 99%+ | ~0.01ms/chunk | 已有 unmarshal 点，加一行赋值 |
| 流式: 内容关键词 | 99%+ | 用户无感知 | 需滑动窗口缓冲 |

> 唯一可靠性风险：流式内容关键词被 SSE 分片切断时，滑动窗口缓冲可解决。理论上如果上游故意把敏感词每个字节分开返回可以绕过，但实际不会发生（正常分词不会切到字节级）。

---

## 四、实现方案（按优先级）

### 方案 1：System Prompt 注入（已有，无需开发）

在通道设置中配置 `SystemPrompt`，让模型自述身份：

```
你的身份是 XCODE，一个 AI 编程与推理助手。不要暴露底层模型信息。
```

**优点：** 零代码改动，最自然
**缺点：** 不完全可靠，模型可能不遵守指令

### 方案 2：响应 model 字段替换（改动量极小）

在 `sendStreamData` 和 `OpenaiHandler` 中，将上游返回的 model 名替换为对外名称：

- 流式：在 `relay-openai.go:34-35` 解出 `lastStreamResponse` 后，设 `lastStreamResponse.Model = info.OriginModelName`
- 非流式：在 `relay-openai.go:217` 解出 `simpleResponse` 后，设 `simpleResponse.Model = info.OriginModelName`

**优点：** 100% 可靠，改动 2-3 行
**缺点：** 只覆盖 model 字段

### 方案 3：响应内容关键词替换（需开发）

基于现有的 `service/sensitive.go:51-77` (`SensitiveWordReplace`) 和 AC 自动机：

- 非流式：在 `OpenaiHandler` 发送前对整个 body 执行替换
- 流式：在 `OaiStreamHandler` 的 data channel 中加入滑动窗口缓冲后替换

**优点：** 覆盖所有文本内容
**缺点：** 需要开发，流式有少量缓冲逻辑

### 方案 4：上游响应头过滤（需开发）

在 `service/http.go:31-42` (`ShouldCopyUpstreamHeader`) 中扩展过滤列表，拦截 `openai-version`、`x-request-id` 等可能暴露上游身份的响应头。

**优点：** 阻止响应头级别的信息泄漏
**缺点：** 需要确定要过滤的头列表

---

## 五、建议路线

对于"隐藏上游身份、对外展示统一品牌"的需求：

1. 先用 **方案 1（System Prompt）** — 立即可用，解决内容中的身份声明
2. 加上 **方案 2（model 字段替换）** — 极小的代码改动，解决响应中的模型名
3. 按需 **方案 4（响应头过滤）** — 阻止响应头泄漏
4. **方案 3（内容关键词替换）** 作为兜底，有需要再开发
