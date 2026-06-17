# request id 链式拼接说明

## 背景

当用户请求通过多层 new-api 实例代理时（即 new-api 的上游也是 new-api），错误消息中会出现多个 `(request id: ...)` 后缀。

示例：

```
API Error: 400 工具名称过长... (request id: 202606140137586572997678268d9d6I1NbUZ1W) (request id: 202606140137584957534618268d9d6daSyLj3h) (request id: 202606140137582005967208268d9d6brib6XBV)
```

## 原因

### 生成机制

1. **request ID 格式**：每个 new-api 实例在 `middleware/request-id.go:23` 中生成唯一 ID，格式为 `时间戳(14位) + buildHash(8位) + 随机串(8位)`，并通过 `c.Header()` 写入响应头 `X-Oneapi-Request-Id`。

2. **拼接时机**：`controller/relay.go:92` 的 defer 块中，当发生错误时会调用 `common.MessageWithRequestId()` 将本实例的 request ID 追加到错误消息末尾。

3. **链式传递**：当上游也是 new-api 实例时，上游返回的错误消息中已包含其 request ID。本实例将其视为普通文本，再次拼接自己的 request ID。

### 链路示意

```
客户端 → new-api A → new-api B → new-api C → AI厂商
                                                   ↓ 返回: "400 xxx"
                                   C 拼接: "400 xxx (request id: C)"
                       B 拼接: "400 xxx (request id: C) (request id: B)"
               A 拼接: "400 xxx (request id: C) (request id: B) (request id: A)"
客户端 ← 收到3个 request id
```

### 关键代码

- `middleware/request-id.go:23` — 生成本实例 request ID
- `common/utils.go:278` — `MessageWithRequestId()` 无条件拼接
- `controller/relay.go:92` — 错误时调用拼接

## 性质

这是**单层实例设计的副作用，而非刻意设计的链式功能**。理由：

1. `MessageWithRequestId` 不做重复检测，不剥离已有 ID
2. 平铺格式不含链式语义（如 `chain: A → B → C`）
3. 无相关配置项可控制
4. git 历史无多层链相关提交

## 排查方法

链式 ID 对调试有一定价值：每个 ID 可在对应那层实例的日志中独立搜索，帮助定位问题发生在哪一层。

- 搜索条件：管理后台 → 日志 → 按 `request_id` 筛选
- 日志表：`logs` 表的 `request_id` 和 `upstream_request_id` 字段
- 链路中各层也可通过自己的日志回溯

## 相关文件

| 文件 | 作用 |
|------|------|
| `middleware/request-id.go` | 生成本实例 request ID 并写入响应头 |
| `common/utils.go:278` | `MessageWithRequestId()` 拼接函数 |
| `controller/relay.go:92` | 错误时调用拼接 |
| `service/http.go:26` | `ShouldCopyUpstreamHeader()` 过滤上游响应头中的 `X-Oneapi-Request-Id`（只过滤 header，不过滤错误消息文本） |
| `model/log.go` | 日志存储，含 `request_id` 和 `upstream_request_id` 字段 |
