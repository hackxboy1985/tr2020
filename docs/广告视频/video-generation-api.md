# 广告视频生成  API 接口文档-对外文档
- 本文档提供给下游用户调用，用于创建广告视频项目。

**url**：`http://open.mints-id.com`

**鉴权**：所有接口需在 Header 中携带 API Key：
```
Authorization: Bearer <your_api_key>
```




---

## 1. 创建视频项目

**POST** `/api/video-generation/create`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 | 可选值 / 示例 |
|------|------|------|------|--------------|
| `video_model` | string | 是 | 模型 | `alpha-pro`（高质量）/ `alpha-flash`（快速） |
| `product_name` | string | 是 | 产品名称，最多 15 字 | `"仿生物形象智能音箱Pro"` |
| `prompt` | string | 是 | 补充提示词，最少 1 字。支持 `@产品图N` / `@人物N` / `@参考N` 引用 mediaList 里的图片 | `"夏日清新，@产品图1 突出轻盈透气"` |
| `resolution` | string | 是 | 清晰度（flash 类模型最高 720p） | `"480p"` / `"720p"` / `"1080p"` |
| `duration` | integer | 是 | 视频时长（秒） | `15` / `30` / `45` / `60` |
| `whstr` | string | 是 | 视频比例 | `"21:9"` / `"16:9"` / `"4:3"` / `"1:1"` / `"3:4"` / `"9:16"` |
| `vtype` | string | 是 | 视频类型 | `"随机"` / `"产品展示"` / `"剧情短片"` / `"口播"` / `"混剪"` |
| `vtype_add` | string | 是 | 剧情风格。预设值或自定义（最多 20 字） | 预设：`"随机"` / `"搞笑"` / `"温情"` / `"悬疑"` / `"炫酷"` / `"治愈"` / `"现代"` / `"古装"` / `"科幻"` / `"奇幻"` / `"武侠"` / `"都市"` / `"校园"`。自定义如 `"赛博朋克"` / `"复古胶片"` |
| `platform` | string | 是 | 投放平台名（按 `region` 选择对应集合） | 国内：`"通用"` / `"淘宝"` / `"京东"` / `"拼多多"` / `"1688"` / `"小红书"` / `"抖音"` / `"天猫"` / `"快手"` / `"微信"`<br/>国际：`"Amazon"` / `"Temu"` / `"Shopee"` / `"TikTok"` / `"AliExpress"` / `"阿里巴巴"` / `"OZON"` / `"Lazada"` / `"DHgate"` / `"Coupang"` / `"11Street"` / `"Wayfair"` / `"Etsy"` / `"Noon"` / `"eBay"` |
| `region` | string | 是 | 电商区域 | `"国内电商"` / `"国际电商"` |
| `language` | string | 是 | 广告语言 | `"中文简体"` / `"中文繁体"` / `"英文"` / `"美式英语"` / `"英式英语"` / `"日文"` / `"韩文"` / `"西班牙文"` / `"葡萄牙文"` / `"法文"` / `"德文"` / `"意大利文"` / `"俄文"` / `"波兰文"` / `"荷兰文"` / `"土耳其文"` / `"瑞典文"` / `"挪威文"` / `"丹麦文"` / `"阿拉伯文"` / `"希伯来文"` / `"波斯文"` / `"泰文"` / `"越南文"` / `"印尼文"` / `"马来文"` / `"菲律宾文"` / `"印地文"` / `"孟加拉文"` / `"乌尔都文"` / `"斯瓦希里文"` / `"豪萨文"` |
| `brand` | string | 否 | 品牌名，最多 15 字 | `"Nike"` |
| `tagline` | string | 否 | 宣传语 Slogan，最多 15 字 | `"Just Do It"` |
| `selling_points` | string | 否 | 产品卖点，最多 15 字 | `"轻盈透气、回弹减震"` |
| `mediaList` | array | 是 | 媒体列表，至少 1 张产品图（`mediaType=PRODUCT`），总数 ≤ 10 | 见下表 |

### mediaList 元素结构

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| mediaType | string | 是 | `"PRODUCT"`（产品图）/ `"ROLE"`（出镜人物，最多 3 张）/ `"OTHER"`（参考素材） |
| mediaUrl | string | 是 | 图片 URL（不支持视频文件） |
| roleName | string | 否 | 角色名（仅 ROLE 类型有意义），配合 `prompt` 里 `@人物N` 使用 |
| sortOrder | number | 否 | 排序 |

### 计费规则

- 基础费用 = `duration（秒）× 单秒单价（分辨率）`
- 创建时**预扣**，任务完成后按上游实际消耗**结算**（多退少补）

### 请求示例

```json
{
  "product_name": "仿生物形象智能音箱Pro",
  "brand": "SoundMax",
  "tagline": "听见好声音",
  "selling_points": "高保真音质、24h续航",
  "prompt": "现代客厅场景，阳光透过落地窗洒进来。年轻女性坐在沙发上，轻声对@产品图1说'播放音乐'，产品亮起柔和的蓝光。镜头从全景推到产品特写，展现质感。",
  "resolution": "480p",
  "duration": 15,
  "whstr": "16:9",
  "vtype": "剧情短片",
  "vtype_add": "温情",
  "platform": "抖音",
  "region": "国内电商",
  "language": "中文简体",
  "video_model": "alpha-flash",
  "mediaList": [
    {
      "mediaType": "PRODUCT",
      "mediaUrl": "https://static.horse-world.mints-id.com//general/1/image/2026-06-17/ecom/1_1781692735099.png",
      "sortOrder": 1
    }
  ]
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `project_id` | int | 项目 ID |
| `project_name` | string | 项目名称 |
| `status` | string | 状态，初始为 `CREATED` |
| `created_at` | int | 创建时间（Unix 时间戳） |

### 响应示例

```json


{
  "code": 200,
  "data": {
    "project_id": 34,
    "project_name": "176_20260706_1783341251",
    "status": "RUNNING",
    "created_at": 1783341251
  },
  "msg": "video project created successfully"
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

> **扣费与退款说明：** 发起时按 `模型单价 × 时长` 扣除积分。若提交工作流失败，会自动退回；任务执行阶段失败或部分生成，也会在任务变为终态时退回失败部分。


---

## 2. 查询视频项目

**GET** `/api/video-generation/projects/:id`

### 状态枚举

| status | 说明 |
|---|---|
| `CREATED` | 已创建，等待处理 |
| `RUNNING` | 生成中 |
| `VIDEO_PROCESSING` | 视频处理中 |
| `SUCCESS` | 完成 ✓ |
| `FAILED` | 失败 |

**轮询建议**：进行中状态每 10~30 秒查询一次，终态（`SUCCESS` / `FAILED`）停止轮询。

### 响应字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `project_id` | int | 项目 ID |
| `project_name` | string | 项目名称 |
| `status` | string | 状态，见上方枚举 |
| `error_msg` | string | 失败原因（失败时有值） |
| `product_img_url` | string | 产品图 URL |
| `brand` | string | 品牌名 |
| `product_name` | string | 产品名 |
| `first_video_url` | string | 视频地址（完成后有值） |
| `created_at` | int | 创建时间（Unix 时间戳） |
| `updated_at` | int | 更新时间（Unix 时间戳） |
| `billing` | object | 计费信息，仅 `SUCCESS` 且结算完成后有值，见下表 |

### billing 字段说明

| 字段 | 类型 | 说明 |
|---|---|---|
| `upstream_net` | string | 上游实际扣费金额（元）= 预扣 - 退款 |
| `charged_seconds` | float | 实际计费秒数，可直接用于下游计费 |

**下游计费公式：**
```
实际应收 = 用户报价 / 请求秒数 × charged_seconds
```

### 响应示例

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "project_id": 34,
    "project_name": "176_20260706_1783341251",
    "status": "SUCCESS",
    "product_img_url": "",
    "brand": "SoundMax",
    "product_name": "仿生物形象智能音箱Pro",
    "first_video_url": "https://static.horse-world.mints-id.com/good/project/4/video-merged/178334159029.mp4",
    "billing": {
      "upstream_net": "0.030",
      "charged_seconds": 15
    },
    "created_at": 1783341251,
    "updated_at": 1783341343
  }
}
```

---

## 错误响应格式

```json
{ "code": 400, "msg": "错误说明", "data": null }
```

| code | 说明 |
|---|---|
| 400 | 参数错误 |
| 401 | 鉴权失败 |
| 404 | 项目不存在 |
| 429 | 超出请求频率限制 |
| 500 | 服务器内部错误 |
