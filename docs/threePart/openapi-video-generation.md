# 归一【三方供应商接口文档】 视频生成 OpenAPI 文档 v2

> Base Path: `/openapi/video`
> 认证：所有接口通过 HTTP Header `Authorization: Bearer <token>` 传递 Token。
> 未携带或 Token 无效：`{ "code": 500, "msg": "未授权，请重新登录" }`

---

## 1. 发起视频生成任务

**POST** `/openapi/video/generate`

**Content-Type**: `application/json`

### Request Body

| 字段 | 类型 | 必填 | 说明 | 可选值 / 示例 |
|------|------|------|------|--------------|
| videoModel | string | 是 | 视频模型 ID，从 `/models` 接口拉取 | `"42"`（Seedance 2.0）、`"44"`（Seedance 2.0 Fast）等（以 `/models` 返回为准） |
| productName | string | 是 | 产品名称，最多 15 字 | `"Air Max 270"` |
| prompt | string | 是 | 补充提示词，最少 1 字。支持 `@产品图N` / `@人物N` / `@参考N` 引用 mediaList 里的图片 | `"夏日清新，@产品图1 突出轻盈透气"` |
| resolution | string | 是 | 清晰度（Fast 类模型最高 720p） | `"480p"` / `"720p"` / `"1080p"` |
| duration | integer | 是 | 视频时长（秒） | `15` / `30` / `45` / `60` |
| whstr | string | 是 | 视频比例 | `"21:9"` / `"16:9"` / `"4:3"` / `"1:1"` / `"3:4"` / `"9:16"` |
| vtype | string | 是 | 视频类型 | `"随机"` / `"产品展示"` / `"剧情短片"` / `"口播"` / `"混剪"` |
| vtypeAdd | string | 是 | 剧情风格。预设值或自定义（最多 20 字） | 预设：`"随机"` / `"搞笑"` / `"温情"` / `"悬疑"` / `"炫酷"` / `"治愈"` / `"现代"` / `"古装"` / `"科幻"` / `"奇幻"` / `"武侠"` / `"都市"` / `"校园"`。自定义如 `"赛博朋克"` / `"复古胶片"` |
| platform | string | 是 | 投放平台名（按 `region` 选择对应集合） | 国内：`"通用"` / `"淘宝"` / `"京东"` / `"拼多多"` / `"1688"` / `"小红书"` / `"抖音"` / `"天猫"` / `"快手"` / `"微信"`<br/>国际：`"Amazon"` / `"Temu"` / `"Shopee"` / `"TikTok"` / `"AliExpress"` / `"阿里巴巴"` / `"OZON"` / `"Lazada"` / `"DHgate"` / `"Coupang"` / `"11Street"` / `"Wayfair"` / `"Etsy"` / `"Noon"` / `"eBay"` |
| region | string | 是 | 电商区域 | `"国内电商"` / `"国际电商"` |
| language | string | 是 | 广告语言 | `"中文简体"` / `"中文繁体"` / `"英文"` / `"美式英语"` / `"英式英语"` / `"日文"` / `"韩文"` / `"西班牙文"` / `"葡萄牙文"` / `"法文"` / `"德文"` / `"意大利文"` / `"俄文"` / `"波兰文"` / `"荷兰文"` / `"土耳其文"` / `"瑞典文"` / `"挪威文"` / `"丹麦文"` / `"阿拉伯文"` / `"希伯来文"` / `"波斯文"` / `"泰文"` / `"越南文"` / `"印尼文"` / `"马来文"` / `"菲律宾文"` / `"印地文"` / `"孟加拉文"` / `"乌尔都文"` / `"斯瓦希里文"` / `"豪萨文"` |
| brand | string | 否 | 品牌名，最多 15 字 | `"Nike"` |
| tagline | string | 否 | 宣传语 Slogan，最多 15 字 | `"Just Do It"` |
| sellingPoints | string | 否 | 产品卖点，最多 15 字 | `"轻盈透气、回弹减震"` |
| mediaList | array | 是 | 媒体列表，至少 1 张产品图（`mediaType=PRODUCT`），总数 ≤ 10 | 见下表 |

#### mediaList 元素结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| mediaType | string | 是 | `"PRODUCT"`（产品图）/ `"ROLE"`（出镜人物，最多 3 张）/ `"OTHER"`（参考素材） |
| mediaUrl | string | 是 | 图片 URL（不支持视频文件） |
| assetId | number | 否 | 资产 ID |
| roleName | string | 否 | 角色名（仅 ROLE 类型有意义），配合 `prompt` 里 `@人物N` 使用 |
| sortOrder | number | 否 | 排序 |

#### 数量约束

- `PRODUCT`：至少 1 张
- `ROLE`：最多 3 张
- 三种合计：≤ 10 张

### 请求示例

```json
{
  "videoModel": "42",
  "productName": "Air Max 270",
  "brand": "Nike",
  "tagline": "Just Do It",
  "sellingPoints": "轻盈透气、回弹减震",
  "prompt": "夏日清新风格，@产品图1 突出轻盈透气",
  "resolution": "720p",
  "duration": 15,
  "whstr": "9:16",
  "vtype": "剧情短片",
  "vtypeAdd": "随机",
  "platform": "通用",
  "region": "国内电商",
  "language": "中文简体",
  "mediaList": [
    { "mediaType": "PRODUCT", "mediaUrl": "https://.../shoe1.jpg", "sortOrder": 1 },
    { "mediaType": "ROLE",    "mediaUrl": "https://.../person.jpg", "roleName": "人物1" },
    { "mediaType": "OTHER",   "mediaUrl": "https://.../ref.jpg" }
  ]
}
```

### 响应

```json
{
  "code": 200,
  "msg": "任务已提交",
  "data": {
    "taskId": 123456,
    "status": "COZE_RUNNING"
  }
}
```

### 常见错误

| msg | 说明 |
|-----|------|
| 未授权，请重新登录 | Token 缺失/失效 |
| 产品名称不能为空 | `productName` 空 |
| 补充提示词不能为空 | `prompt` 空 |
| 请至少上传1张产品图片 | `mediaList` 空 |
| 请至少上传1张产品图片（mediaType=PRODUCT） | mediaList 里没有 PRODUCT |

---

## 2. 查询任务状态

**GET** `/openapi/video/query/{taskId}`

- 需登录；只能查询自己创建的任务，越权访问返回 `无权访问该任务`。

### 响应

```json
{
  "code": 200,
  "data": {
    "taskId": 123456,
    "status": "ONE_CLICK_GENERATED",
    "videoUrl": "https://.../result.mp4",
    "errorMsg": null
  }
}
```

### status 枚举

| status | 说明 |
|--------|------|
| CREATED | 已创建 |
| COZE_RUNNING | Coze 工作流运行中 |
| VIDEO_PROCESSING | 视频处理中 |
| VIDEO_PREPARING | 等待拼接（可重试） |
| VIDEO_CONCAT | 拼接完成（等待 OSS） |
| ONE_CLICK_GENERATED | 生成完成，`videoUrl` 有值 |
| FAILED | 失败，`errorMsg` 有值 |

---

## 3. 获取可用视频模型

**GET** `/openapi/video/models`

### 响应

```json
{
  "code": 200,
  "data": [
    {
      "value": "42",
      "label": "Seedance 2.0",
      "priceResolutions": ["480p", "720p", "1080p"],
      "prices": [
        { "resolution": "480p", "pricePerSecond": 5 },
        { "resolution": "720p", "pricePerSecond": 10 },
        { "resolution": "1080p", "pricePerSecond": 20 }
      ]
    },
    {
      "value": "44",
      "label": "Seedance 2.0 Fast",
      "priceResolutions": ["480p", "720p"]
    }
  ]
}
```

- 调用方可用 `priceResolutions` 校验 `resolution` 是否被当前模型支持。
- 积分预估：`prices[].pricePerSecond × duration`（向上取整）。

---

## 变更记录

- 2026-07-02
  - `userId` 从请求体移除，改为从 `Authorization` Token 解析（对齐 `/api/frontend/chat/stream`）。
  - `GET /query/{taskId}` 新增登录 + 归属校验，防止越权访问。
