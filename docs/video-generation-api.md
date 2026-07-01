# 视频生成 API 文档

## 概述

本 API 提供视频生成项目的创建、查询、管理功能。采用方案 A（独立业务 API），保持业务语义清晰。

## 响应格式

所有接口统一返回格式:

```json
{
  "code": 200,
  "msg": "success message",
  "data": {}
}
```

- `code`: HTTP 状态码（200 成功，400/404/500 失败）
- `msg`: 消息描述
- `data`: 响应数据（成功时包含数据，失败时为 null）

## 接口列表

### 1. 创建视频项目

**接口**: `POST /api/video-generation/create`

**认证**: 需要用户 Token（UserAuth）

**请求参数**:

```json
{
  // 广告基础信息（必填）
  "product_img_url": "https://oss.example.com/product.jpg",
  "brand": "品牌名称",
  "product_name": "产品名称",
  "tagline": "宣传语（可选）",
  "selling_points": "产品卖点（可选）",
  
  // 创意方向（必填）
  "prompt": "用户创意描述，核心输入",
  "vtype": "产品展示",
  "vtype_add": "搞笑（可选）",
  "language": "中文（可选）",
  "platform": "抖音（可选）",
  "region": "国内电商（可选）",
  
  // 角色与参考（可选）
  "roles": "[{\"name\":\"角色1\",\"url\":\"...\"}]",
  "select_audios": "[{\"url\":\"...\",\"remark\":\"...\"}]",
  
  // 输出配置（必填）
  "duration": 30,
  "resolution": "2K",
  "video_model": "seedance",
  "whstr": "16:9"
}
```

**响应示例**:

```json
{
  "code": 200,
  "msg": "video project created successfully",
  "data": {
    "project_id": 123456,
    "project_name": "username_20260701_1719820800",
    "status": "CREATED",
    "created_at": 1719820800
  }
}
```

### 2. 获取视频项目详情

**接口**: `GET /api/video-generation/projects/:id`

**认证**: 需要用户 Token

**响应示例**:

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "project_id": 123456,
    "project_name": "username_20260701_1719820800",
    "status": "COZE_RUNNING",
    "error_msg": "",
    "progress": "",
    "product_img_url": "https://...",
    "brand": "品牌名称",
    "product_name": "产品名称",
    "main_image_url": "https://...",
    "main_image_asset_id": "asset_xxx",
    "generated_result": "{...}",
    "first_video_url": "",
    "created_at": 1719820800,
    "updated_at": 1719820900
  }
}
```

### 3. 获取用户的视频项目列表

**接口**: `GET /api/video-generation/projects`

**认证**: 需要用户 Token

**请求参数**:

- `page`: 页码（默认 1）
- `page_size`: 每页数量（默认 10）

**响应示例**:

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "items": [
      {
        "project_id": 123456,
        "project_name": "username_20260701_1719820800",
        "status": "ONE_CLICK_GENERATED",
        "brand": "品牌名称",
        "product_name": "产品名称",
        "created_at": 1719820800,
        "updated_at": 1719820900
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

### 4. 删除视频项目

**接口**: `DELETE /api/video-generation/projects/:id`

**认证**: 需要用户 Token

**响应示例**:

```json
{
  "code": 200,
  "msg": "video project deleted successfully",
  "data": null
}
```

### 5. 管理员获取所有项目列表

**接口**: `GET /api/video-generation/admin/projects`

**认证**: 需要管理员 Token

**请求参数**:

- `page`: 页码
- `page_size`: 每页数量
- `status`: 状态筛选（可选）

**响应格式**: 同接口 3

### 6. 管理员更新项目状态

**接口**: `PUT /api/video-generation/admin/projects/:id/status`

**认证**: 需要管理员 Token

**请求参数**:

```json
{
  "status": "VIDEO_PROCESSING",
  "error_msg": "",
  "main_image_url": "https://...",
  "main_image_asset_id": "asset_xxx",
  "generated_result": "{...}"
}
```

### 7. 管理员删除项目

**接口**: `DELETE /api/video-generation/admin/projects/:id`

**认证**: 需要管理员 Token

### 8. Coze Webhook 回调

**接口**: `POST /api/video-generation/webhook/coze`

**认证**: 无（需实现签名验证）

**请求参数**:

```json
{
  "project_id": 123456,
  "status": "VIDEO_PROCESSING",
  "error_msg": "",
  "main_image_url": "https://...",
  "main_image_asset_id": "asset_xxx",
  "generated_result": "{...}"
}
```

## 项目状态说明

| 状态 | 说明 |
|------|------|
| `CREATED` | 已创建，等待 Coze 处理 |
| `COZE_RUNNING` | Coze 工作流执行中 |
| `VIDEO_PROCESSING` | 视频已生成，等待拼接 |
| `VIDEO_CONCAT` | 拼接完成，等待 OSS 上传 |
| `ONE_CLICK_GENERATED` | OSS 上传完成，全流程结束 |
| `VIDEO_PREPARING` | 拼接失败，需手动重试 |
| `FAILED` | 生成失败 |

## 待实现功能

1. **Coze 工作流调用**: 在 `CreateVideoProject` 中调用 Coze API 触发视频生成
2. **Webhook 签名验证**: 在 `CozeWebhook` 中实现签名验证机制
3. **视频 URL 关联**: 查询和返回生成的视频 URL（需要额外的 `video_media` 表）
4. **进度查询**: 实现实时进度更新和查询

## 数据库表结构

```sql
CREATE TABLE `video_projects` (
  `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME,
  `updated_at` DATETIME,
  `project_name` VARCHAR(255),
  `user_id` INT,
  `product_img_url` TEXT,
  `brand` VARCHAR(50),
  `product_name` VARCHAR(50),
  `tagline` VARCHAR(255),
  `selling_points` TEXT,
  `prompt` TEXT,
  `vtype` VARCHAR(50),
  `vtype_add` VARCHAR(50),
  `language` VARCHAR(20),
  `platform` VARCHAR(50),
  `region` VARCHAR(50),
  `roles` TEXT,
  `select_audios` TEXT,
  `duration` INT,
  `resolution` VARCHAR(20),
  `video_model` VARCHAR(50),
  `whstr` VARCHAR(20),
  `main_image_url` TEXT,
  `main_image_asset_id` VARCHAR(255),
  `generated_result` TEXT,
  `status` VARCHAR(50),
  `error_msg` TEXT,
  `deleted` TINYINT DEFAULT 0,
  INDEX `idx_user_id` (`user_id`),
  INDEX `idx_status` (`status`),
  INDEX `idx_deleted` (`deleted`),
  INDEX `idx_created_at` (`created_at`)
);
```

## 使用示例

### cURL 示例

```bash
# 创建视频项目
curl -X POST https://api.example.com/api/video-generation/create \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_img_url": "https://oss.example.com/product.jpg",
    "brand": "示例品牌",
    "product_name": "示例产品",
    "prompt": "创建一个30秒的产品展示视频",
    "vtype": "产品展示",
    "duration": 30,
    "resolution": "2K",
    "whstr": "16:9"
  }'

# 查询项目状态
curl -X GET https://api.example.com/api/video-generation/projects/123456 \
  -H "Authorization: Bearer YOUR_TOKEN"

# 获取项目列表
curl -X GET "https://api.example.com/api/video-generation/projects?page=1&page_size=10" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 集成说明

1. **认证方式**: 使用系统现有的 Token 认证机制
2. **权限控制**: 
   - 普通用户只能访问自己的项目
   - 管理员可以访问所有项目
3. **错误处理**: 所有错误返回统一的 `{code, msg, data}` 格式
4. **数据库迁移**: 系统启动时自动创建 `video_projects` 表
