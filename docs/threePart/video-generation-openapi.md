# 视频生成 OpenAPI 文档

> 无项目概念，纯视频生成接口，入参与页面表单完全对齐
> **开放接口，无需认证**
> 创建日期：2026-07-01

---

## 接口概览

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/openapi/video/generate` | 发起视频生成任务 |
| GET | `/openapi/video/query/{taskId}` | 查询任务状态 |
| GET | `/openapi/video/models` | 获取可用视频模型列表 |

**Base URL**: `http://your-domain/openapi/video`

**认证方式**: 无需认证，完全开放

---

## 1. 发起视频生成任务

### 请求

**POST** `/openapi/video/generate`

**Headers**:
```
Content-Type: application/json
```

**Body** (JSON):

```json
{
  "userId": 123456,
  "videoModel": "42",
  "productName": "Air Max 270",
  "brand": "Nike",
  "tagline": "Just Do It",
  "sellingPoints": "轻盈透气、回弹减震、适合长时间运动",
  "prompt": "夏日清新风格，突出轻盈透气卖点，@产品图1 特写镜头",
  "resolution": "1080p",
  "duration": 30,
  "whstr": "16:9",
  "vtype": "产品展示",
  "vtypeAdd": "活力运动",
  "platform": "抖音",
  "region": "国内电商",
  "language": "中文",
  "mediaList": [
    {
      "mediaType": "PRODUCT",
      "mediaUrl": "https://oss.example.com/product1.jpg",
      "assetId": "asset_12345",
      "roleName": "产品图1",
      "sortOrder": 0
    },
    {
      "mediaType": "PRODUCT",
      "mediaUrl": "https://oss.example.com/product2.jpg",
      "sortOrder": 1
    },
    {
      "mediaType": "ROLE",
      "mediaUrl": "https://oss.example.com/person1.jpg",
      "roleName": "人物1"
    },
    {
      "mediaType": "OTHER",
      "mediaUrl": "https://oss.example.com/ref1.jpg",
      "roleName": "参考场景"
    }
  ]
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| **userId** | long | ✅ | 用户ID |
| **videoModel** | string | ✅ | 视频模型，如 "42"=Seedance 2.0、"44"=Seedance 2.0 Fast |
| **productName** | string | ✅ | 产品名称，最多15字 |
| **brand** | string | ❌ | 品牌，最多15字 |
| **tagline** | string | ❌ | 宣传语，最多15字 |
| **sellingPoints** | string | ❌ | 产品卖点，最多15字 |
| **prompt** | string | ✅ | 补充提示词，最少1字 |
| **resolution** | string | ✅ | 清晰度：`480p` / `720p` / `1080p`（Fast最高720p） |
| **duration** | integer | ✅ | 视频时长（秒）：`15` / `30` / `45` / `60` |
| **whstr** | string | ✅ | 视频比例：`21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` |
| **vtype** | string | ✅ | 视频类型，如："产品展示"、"剧情短片"、"口播" |
| **vtypeAdd** | string | ✅ | 剧情风格，如："活力运动"、"温情治愈"、"搞笑幽默" |
| **platform** | string | ✅ | 投放平台，如："抖音"、"TikTok"、"淘宝" |
| **region** | string | ✅ | 地区，如："国内电商"、"跨境电商" |
| **language** | string | ✅ | 语言，如："中文"、"英文" |
| **mediaList** | array | ✅ | 媒体列表，至少1张产品图 |

#### mediaList 数组元素

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| **mediaType** | string | ✅ | 媒体类型：`PRODUCT`（产品图）/ `ROLE`（出镜人物）/ `OTHER`（参考素材） |
| **mediaUrl** | string | ✅ | 图片URL |
| **assetId** | string | ❌ | 资产ID |
| **roleName** | string | ❌ | 角色名称/标签 |
| **sortOrder** | integer | ❌ | 排序序号 |

**注意事项**：
- `mediaList` 中必须至少有一张 `mediaType=PRODUCT` 的产品图
- `prompt` 中可使用 `@产品图1`、`@人物1` 等引用 `mediaList` 中的图片（根据 `roleName`）
- Fast模型（"44"）最高分辨率为720p，提交1080p会返回错误

### 响应

**成功** (200):
```json
{
  "code": 200,
  "msg": "任务已提交",
  "data": {
    "taskId": 1234567890,
    "status": "COZE_RUNNING"
  }
}
```

**失败** (200):
```json
{
  "code": 500,
  "msg": "积分不足，本次需 300 积分，当前余额 100，请充值后重试"
}
```

---

## 2. 查询任务状态

### 请求

**GET** `/openapi/video/query/{taskId}`

**Headers**: 无需认证

**路径参数**:
- `taskId`: 任务ID（发起生成时返回的 `taskId`）

### 响应

**成功** (200):
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "taskId": 1234567890,
    "status": "ONE_CLICK_GENERATED",
    "videoUrl": "https://oss.example.com/video/final.mp4"
  }
}
```

**进行中** (200):
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "taskId": 1234567890,
    "status": "COZE_RUNNING",
    "videoUrl": null
  }
}
```

**失败** (200):
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "taskId": 1234567890,
    "status": "FAILED",
    "videoUrl": null,
    "errorMsg": "Coze 工作流执行失败：内部错误"
  }
}
```

### 状态说明

| status | 说明 | 是否终态 | 建议轮询间隔 |
|--------|------|----------|-------------|
| `CREATED` | 已创建 | ❌ | 3s |
| `COZE_RUNNING` | Coze工作流运行中 | ❌ | 3s |
| `VIDEO_PROCESSING` | 视频处理中 | ❌ | 3s |
| `VIDEO_PREPARING` | 等待拼接（可重试） | ❌ | 5s |
| `VIDEO_CONCAT` | 拼接完成（等待OSS） | ❌ | 2s |
| `ONE_CLICK_GENERATED` | 生成完成 | ✅ | - |
| `FAILED` | 失败 | ✅ | - |

**轮询建议**：
- 非终态（`CREATED` / `COZE_RUNNING` / `VIDEO_PROCESSING` / `VIDEO_PREPARING` / `VIDEO_CONCAT`）时，每 3 秒轮询一次
- 终态（`ONE_CLICK_GENERATED` / `FAILED`）时停止轮询
- 最长轮询时间建议不超过 10 分钟

### 返回字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `taskId` | long | 任务ID |
| `status` | string | 任务状态 |
| `videoUrl` | string | 最终视频URL（`ONE_CLICK_GENERATED` 时返回，其他状态为 null） |
| `errorMsg` | string | 错误信息（`FAILED` 时返回） |

---

## 3. 获取可用视频模型列表

### 请求

**GET** `/openapi/video/models`

**Headers**: 无需认证

### 响应

**成功** (200):
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": [
    {
      "value": "42",
      "label": "Seedance 2.0"
    },
    {
      "value": "44",
      "label": "Seedance 2.0 Fast"
    }
  ]
}
```

---

## 完整调用示例

### 1. 获取可用模型

```bash
curl -X GET "http://your-domain/openapi/video/models"
```

### 2. 发起生成任务

```bash
curl -X POST "http://your-domain/openapi/video/generate" \
  -H "Content-Type: application/json" \
  -d '{
    "userId": 123456,
    "videoModel": "42",
    "productName": "Air Max 270",
    "brand": "Nike",
    "tagline": "Just Do It",
    "sellingPoints": "轻盈透气、回弹减震",
    "prompt": "夏日清新风格，突出轻盈透气卖点",
    "resolution": "1080p",
    "duration": 30,
    "whstr": "16:9",
    "vtype": "产品展示",
    "vtypeAdd": "活力运动",
    "platform": "抖音",
    "region": "国内电商",
    "language": "中文",
    "mediaList": [
      {
        "mediaType": "PRODUCT",
        "mediaUrl": "https://oss.example.com/product1.jpg",
        "roleName": "产品图1"
      }
    ]
  }'
```

响应：
```json
{
  "code": 200,
  "msg": "任务已提交",
  "data": {
    "taskId": 1234567890,
    "status": "COZE_RUNNING"
  }
}
```

### 3. 轮询查询状态

```bash
# 每3秒轮询一次
while true; do
  curl -X GET "http://your-domain/openapi/video/query/1234567890"
  sleep 3
done
```

当 `status` 为 `ONE_CLICK_GENERATED` 时，从 `videoUrl` 字段获取最终视频。

---

## 错误码

| code | msg | 说明 |
|------|-----|------|
| 200 | 操作成功 | 成功 |
| 500 | userId 不能为空 | 缺少用户ID |
| 500 | 产品名称不能为空 | 缺少必填参数 |
| 500 | 补充提示词不能为空 | 缺少必填参数 |
| 500 | 请至少上传1张产品图片 | mediaList 为空 |
| 500 | 请至少上传1张产品图片（mediaType=PRODUCT） | mediaList 中没有产品图 |
| 500 | 请选择视频模型、分辨率和时长 | 缺少必填参数 |
| 500 | 视频模型参数无效 | videoModel 不是合法数字 |
| 500 | 所选视频模型不存在，请重新选择 | videoModel 对应的模型不存在 |
| 500 | 所选视频模型已下线，请重新选择 | videoModel 对应的模型已禁用 |
| 500 | 积分价格未配置，请联系管理员配置 | 后台价格配置缺失 |
| 500 | 积分不足，本次需 X 积分，当前余额 Y，请充值后重试 | 用户积分不足 |
| 500 | 提交 Coze 工作流失败: ... | Coze 提交异常 |
| 500 | 任务不存在 | taskId 无效 |
| 500 | 无权访问该任务 | 查询他人任务 |

---

## 积分计费

- 计费公式：`积分 = pricePerSecond × duration`
- 价格配置：`good_price_config` 表（model + resolution → price_per_second）
- 扣费时机：调用 `/generate` 接口时立即扣除
- 退费场景：
  - Coze 提交失败 → 全额退回
  - Coze 执行失败（`executeStatus=Fail`）→ 全额退回
  - 生成视频数量不足（`content_list` 少于预期）→ 按比例退回差额

---

## 技术细节

### 内部流程

```
/openapi/video/generate
  → 校验参数（productName/prompt/mediaList）
  → 构建 GoodProductProject 实体（projectId=null, projectName=OpenAPI_timestamp）
  → preValidate（模型/分辨率/时长/积分校验）
  → insert good_product_project
  → saveMedia（写入 good_project_media + good_project_media_rel）
  → generateOneClick（扣积分 → 提交 Coze 工作流）
  → 返回 taskId=project.id

/openapi/video/query/{taskId}
  → 查询 good_product_project by id
  → buildDetail（不含技术详情）
  → 提取 videoUrl（从 mediaList 中找 VIDEO 类型）
  → 返回简化详情（taskId + status + videoUrl + errorMsg）
```

### 与页面接口的差异

| 页面接口 | OpenAPI | 差异 |
|---------|---------|------|
| `/api/productProject/create` | `/openapi/video/generate` | OpenAPI 不需要 `unifiedProjectId`，内部 `projectId=null` |
| `/api/productProject/{id}` | `/openapi/video/query/{taskId}` | OpenAPI 只返回 `taskId` + `status` + `videoUrl` + `errorMsg`，不暴露内部细节 |
| 返回 `project` 完整对象 | 返回 `taskId` + `status` | OpenAPI 隐藏项目概念 |

### 复用组件

- `ProductProjectLogic.preValidate()` — 校验模型/分辨率/时长/积分
- `ProductProjectLogic.saveMedia()` — 保存媒体关联
- `ProductProjectLogic.generateOneClick()` — 提交 Coze 工作流
- `ProductProjectLogic.buildDetail()` — 构建详情（含技术详情）

---

## 注意事项

1. **开放接口**：无需任何认证，调用方需要提供 `userId` 参数
2. **项目概念隐藏**：OpenAPI 不暴露 `projectId`，内部自动生成 `projectName=OpenAPI_timestamp`
3. **积分扣费**：生成时立即扣除，失败时自动退回
4. **轮询建议**：建议每 3 秒轮询一次，最长不超过 10 分钟
5. **视频拼接**：多段视频（> 4s）会自动拼接，无需客户端处理
6. **Coze 流程**：角色解析 → 分镜AI → 生图 → 生视频 → 拼接，全流程约 5-10 分钟

---

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-07-01 | 初始版本，支持生成和查询 |

---

**实现文件**：`com.zsapps.api.controller.openapi.VideoGenerationOpenApiController`
