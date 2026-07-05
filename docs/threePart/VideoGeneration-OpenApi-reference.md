# 归一【三方供应商接口文档】 视频生成 OpenAPI 文档 v3

> 面向第三方接入的产品创意视频生成接口。无「项目」概念，一次调用发起一个独立的视频生成任务。

## 基础信息

| 项 | 说明 |
|------|------|
| 路径前缀 | `/openapi/video` |
| 认证方式 | 请求头 `Authorization: Bearer <apiKey>` |
| 数据格式 | 请求/响应均为 `application/json`（`/generate` 的 body 为 JSON） |
| userId 来源 | 从 apiKey 解析得到，无需在参数中传入 |
| 积分金额换算 | **1 积分 = 0.1 元**，金额保留 1 位小数（四舍五入） |

### 认证说明

- 所有 `/openapi/video/**` 接口都必须在请求头携带 `Authorization: Bearer <apiKey>`。
- apiKey 无效、不存在或已过期时，返回 HTTP 401，body 为 JSON 错误对象。
- 缺少 `Authorization` 头时，返回 HTTP 401「缺少 Authorization 请求头」。

### 统一响应结构

接口返回 RuoYi 标准 `AjaxResult` 结构：

```json
{
  "code": 200,
  "msg": "任务已提交",
  "data": { }
}
```

- `code`：200 成功，其余为失败（如 500）。
- `msg`：提示信息，成功时为可读文本，失败时为错误原因。
- `data`：业务数据，结构见各接口说明。

---

## 整体调用流程

```
第 1 步 ── 获取可用视频模型 ─────────────── GET  /openapi/video/models
第 2 步 ── 发起视频生成任务 ─────────────── POST /openapi/video/generate      →  拿到 taskId
第 3 步 ── 轮询任务状态直到终态 ─────────── GET  /openapi/video/query/{taskId}
```

终态为 `ONE_CLICK_GENERATED`（成功）或 `FAILED`（失败）。建议轮询间隔 5~10 秒。

---

## 接口一：获取可用视频模型

### `GET /openapi/video/models`

**使用时机：** 发起生成前，获取当前启用的视频模型列表，供 `videoModel` 取值。

**请求参数：** 无（仅需认证头）。

**响应 `data`：** 模型对象数组。

| 字段 | 类型 | 说明 |
|------|------|------|
| `value` | String | 模型 ID，作为 `/generate` 的 `videoModel` 传入 |
| `label` | String | 模型显示名称 |

**响应示例：**
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": [
    { "value": "42", "label": "Seedance 2.0" },
    { "value": "44", "label": "Seedance 2.0 Fast" }
  ]
}
```

> 模型列表由后台配置控制，实际可用值以本接口返回为准，请勿硬编码。

---

## 接口二：发起视频生成任务

### `POST /openapi/video/generate`

**使用时机：** 提交产品信息、创意提示词与媒体素材，发起一次视频生成。提交即扣除积分。

**请求 body：** `application/json`

#### 基础信息参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `videoModel` | String | 是 | 视频模型 ID，取值来自 `/models` 接口的 `value`（如 `"42"`=Seedance 2.0、`"44"`=Seedance 2.0 Fast） |
| `productName` | String | 是 | 产品名称，最多 15 字 |
| `brand` | String | 否 | 品牌，最多 15 字 |
| `tagline` | String | 否 | 宣传语/slogan，最多 15 字 |
| `sellingPoints` | String | 否 | 产品卖点，最多 200 字 |
| `prompt` | String | 是 | 补充提示词，至少 1 字（驱动整个生成的核心输入） |

#### 输出配置参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resolution` | String | 是 | 清晰度：`480p` / `720p` / `1080p`（Fast 模型最高 720p） |
| `duration` | Integer | 是 | 视频时长（秒）：`15` / `30` / `45` / `60` |
| `whstr` | String | 是 | 视频比例：`21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` |
| `vtype` | String | 是 | 视频类型（如产品展示/剧情短片/口播等） |
| `vtypeAdd` | String | 是 | 剧情风格（如搞笑/温情/悬疑等） |
| `platform` | String | 是 | 投放平台（如抖音/淘宝/TikTok） |
| `region` | String | 是 | 投放地区（如国内电商/跨境电商） |
| `language` | String | 是 | 语言 |

#### 媒体列表参数 `mediaList`（数组，必填，至少 1 张产品图）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `mediaType` | String | 是 | 媒体类型：`PRODUCT`（产品图）/ `ROLE`（出镜人物）/ `OTHER`（参考素材） |
| `mediaUrl` | String | 是 | 图片 URL |
| `assetId` | String | 否 | 资产 ID |
| `roleName` | String | 否 | 角色名（`ROLE` 类型时使用） |
| `sortOrder` | Integer | 否 | 排序序号，不传则按数组顺序 |

**媒体数量限制：**
- 产品图（`PRODUCT`）：至少 1 张，最多 3 张。
- 全部媒体（产品图 + 出镜人物 + 参考素材）合计最多 7 张。

**请求示例：**
```json
{
  "videoModel": "42",
  "productName": "轻氧面膜",
  "brand": "XX美妆",
  "tagline": "熬夜也能水光肌",
  "sellingPoints": "补水保湿、熬夜急救、温和不刺激",
  "prompt": "展示女性敷面膜后皮肤水润有光泽，突出补水急救效果",
  "resolution": "720p",
  "duration": 30,
  "whstr": "9:16",
  "vtype": "产品展示",
  "vtypeAdd": "温情",
  "platform": "抖音",
  "region": "国内电商",
  "language": "中文",
  "mediaList": [
    { "mediaType": "PRODUCT", "mediaUrl": "https://oss.example.com/product1.jpg", "sortOrder": 0 },
    { "mediaType": "ROLE", "mediaUrl": "https://oss.example.com/role1.jpg", "roleName": "女主", "sortOrder": 1 }
  ]
}
```

**响应 `data`（成功时）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `taskId` | Long | 生成任务 ID，用于后续 `/query` 查询 |
| `status` | String | 固定为 `COZE_RUNNING`（任务已提交，工作流运行中） |
| `creditAmount` | Long | 本次任务扣除的积分。**后续可能因失败或部分生成退回，最终净扣以 `/query` 为准** |
| `moneyAmount` | Number | 本次扣除对应的金额（元），= `creditAmount` × 0.1 |

**响应示例：**
```json
{
  "code": 200,
  "msg": "任务已提交",
  "data": {
    "taskId": 100234,
    "status": "COZE_RUNNING",
    "creditAmount": 300,
    "moneyAmount": 30.00
  }
}
```

**常见失败原因（`code` 非 200）：**

| 提示 | 原因 |
|------|------|
| 未授权，请重新登录 | apiKey 无效 |
| 产品名称不能为空 | 缺 `productName` |
| 补充提示词不能为空 | 缺 `prompt` |
| 请至少上传1张产品图片 | `mediaList` 为空 |
| 请至少上传1张产品图片（mediaType=PRODUCT） | 无 `PRODUCT` 类型媒体 |
| 产品卖点最多 200 字 | `sellingPoints` 超长 |
| 请选择视频模型、分辨率和时长 | `videoModel`/`resolution`/`duration` 缺失 |
| 所选视频模型不存在/已下线，请重新选择 | `videoModel` 无效 |
| 积分价格未配置，请联系管理员配置 | 该模型+分辨率未配置价格 |
| 产品图片最多 3 张 | `PRODUCT` 超过 3 张 |
| 产品图 + 出镜人物 + 参考素材合计不能超过 7 张 | 媒体合计超 7 张 |
| 积分不足，本次需 X 积分，当前余额 Y，请充值后重试 | 余额不足 |

> **扣费与退款说明：** 发起时按 `模型单价 × 时长` 扣除积分。若提交工作流失败，会自动全额退回；任务执行阶段失败或部分生成，也会在任务变为终态时退回（见接口三）。

---

## 接口三：查询视频生成任务状态

### `GET /openapi/video/query/{taskId}`

**使用时机：** 轮询任务进度，直到 `status` 为终态。

**路径参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `taskId` | Long | 是 | `/generate` 返回的任务 ID |

**响应 `data`：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `taskId` | Long | 任务 ID |
| `status` | String | 任务状态，见下方枚举 |
| `videoUrl` | String | 最终视频 URL，仅在 `ONE_CLICK_GENERATED` 时有值，其余为 `null` |
| `errorMsg` | String | 错误信息，仅在 `FAILED` 且有错误详情时返回 |
| `creditAmount` | Long | 发起时扣除的积分总额 |
| `creditRefund` | Long | 已退回的积分（失败全退 / 部分生成按比例退） |
| `creditNet` | Long | 实际净消耗积分 = `creditAmount` − `creditRefund` |
| `moneyAmount` | Number | 扣除积分对应金额（元），= `creditAmount` × 0.1 |
| `moneyRefund` | Number | 退回积分对应金额（元），= `creditRefund` × 0.1 |
| `moneyNet` | Number | 实际净消耗金额（元），= `creditNet` × 0.1 |

**`status` 状态枚举：**

| 状态值 | 含义 | 是否终态 |
|--------|------|---------|
| `CREATED` | 已创建，等待处理 | 否 |
| `COZE_RUNNING` | 工作流运行中 | 否 |
| `VIDEO_PROCESSING` | 视频处理中 | 否 |
| `VIDEO_PREPARING` | 等待拼接（可重试） | 否 |
| `VIDEO_CONCAT` | 拼接完成（等待 OSS 上传） | 否 |
| `ONE_CLICK_GENERATED` | 生成完成（成功终态） | 是 |
| `FAILED` | 失败（失败终态） | 是 |

**成功响应示例：**
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "taskId": 100234,
    "status": "ONE_CLICK_GENERATED",
    "videoUrl": "https://oss.example.com/result/100234.mp4",
    "errorMsg": null,
    "creditAmount": 300,
    "creditRefund": 0,
    "creditNet": 300,
    "moneyAmount": 30.00,
    "moneyRefund": 0.00,
    "moneyNet": 30.00
  }
}
```

**失败退款响应示例：**
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "taskId": 100235,
    "status": "FAILED",
    "videoUrl": null,
    "errorMsg": "视频生成失败",
    "creditAmount": 300,
    "creditRefund": 300,
    "creditNet": 0,
    "moneyAmount": 30.00,
    "moneyRefund": 30.00,
    "moneyNet": 0.00
  }
}
```

**失败原因：**

| 提示 | 原因 |
|------|------|
| 未授权，请重新登录 | apiKey 无效 |
| 任务不存在 | `taskId` 不存在 |
| 无权访问该任务 | 该任务不属于当前 apiKey 对应用户 |

> **⚠️ 关于净扣金额的准确性（重要）：**
> 退款是异步写入的。只有当 `status` 到达终态（`ONE_CLICK_GENERATED` / `FAILED`）时，`creditRefund` / `creditNet`（及对应金额）才是最终值。
> 若在中间态（如 `VIDEO_PROCESSING`）查询，退款可能尚未落库，此时 `creditNet` 可能偏大。**请以终态返回的 `creditNet` / `moneyNet` 作为最终实际消耗。**

---

## 附录：积分与金额换算

- 换算关系：**1 积分 = 0.1 元**。
- 金额字段（`moneyAmount` / `moneyRefund` / `moneyNet`）均由对应积分 ÷ 10 得到，保留 2 位小数，四舍五入。
- 例：300 积分 = 30.00 元；15 积分 = 1.50 元。
