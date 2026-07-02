# 视频生成系统设计文档（方案A：本地保存 + 多渠道架构）

## 概述

本文档描述视频生成功能的完整技术架构。该系统采用**方案A（本地保存 + 多渠道架构）**，支持同时对接 Coze 和三方平台两种渠道。

## 设计目标

1. **多渠道支持**：统一接口，支持 Coze 和三方平台两种渠道
2. **本地状态管理**：在本地数据库持久化项目信息，不完全依赖上游平台
3. **用户隔离**：基于用户ID的权限控制
4. **异步处理**：支持长时间运行的视频生成任务
5. **可扩展性**：易于添加新的视频生成渠道

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Client (前端)                         │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTP API
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   new-api (视频生成网关)                      │
│                                                              │
│  ┌─────────────┐   ┌──────────────┐   ┌─────────────┐     │
│  │  Controller  │──▶│   Service    │──▶│   Adapter   │     │
│  └─────────────┘   └──────────────┘   └─────────────┘     │
│                                         │                    │
│                                         ├─ CozeAdapter       │
│                                         └─ PlatformAdapter   │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              video_projects 表 (本地存储)             │   │
│  └─────────────────────────────────────────────────────┘   │
└───────────────┬────────────────────────────┬────────────────┘
                │                            │
                │ API调用                    │ Webhook回调
                ▼                            ▼
    ┌──────────────────┐          ┌──────────────────┐
    │  Coze 工作流引擎  │          │   三方平台 API    │
    └──────────────────┘          └──────────────────┘
```

### 分层架构

遵循 new-api 的分层架构：

```
router/
  └─ video_generation.go          # 路由注册

controller/
  └─ video_generation.go          # API 处理逻辑

service/
  ├─ video_adapter.go              # 渠道适配器接口定义
  ├─ video_adapter_coze.go         # Coze 渠道实现
  ├─ video_adapter_platform.go     # 三方平台实现
  └─ video_generation_service.go   # 业务逻辑层

model/
  └─ video_project.go              # 数据模型 + 数据库操作

dto/
  └─ video_project.go              # 请求/响应结构定义
```

## 数据模型

### video_projects 表结构

```sql
CREATE TABLE `video_projects` (
  -- 主键
  `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
  `created_at` DATETIME NOT NULL,
  `updated_at` DATETIME NOT NULL,

  -- 项目基础信息
  `project_name` VARCHAR(255) NOT NULL,
  `user_id` INT NOT NULL,
  `username` VARCHAR(255) DEFAULT '',
  `channel_type` VARCHAR(20) DEFAULT 'platform',  -- 'coze' 或 'platform'
  `remote_project_id` VARCHAR(255),              -- 三方平台返回的项目ID

  -- 广告基础信息
  `product_img_url` TEXT,
  `brand` VARCHAR(50),
  `product_name` VARCHAR(50),
  `tagline` VARCHAR(255),
  `selling_points` TEXT,

  -- 创意方向
  `prompt` TEXT NOT NULL,
  `vtype` VARCHAR(50) NOT NULL,
  `vtype_add` VARCHAR(50),
  `language` VARCHAR(20),
  `platform` VARCHAR(50),
  `region` VARCHAR(50),

  -- 角色与参考
  `roles` TEXT,              -- JSON字符串
  `select_audios` TEXT,      -- JSON字符串

  -- 输出配置
  `duration` INT NOT NULL,
  `resolution` VARCHAR(20) NOT NULL,
  `video_model` VARCHAR(50),
  `whstr` VARCHAR(20) NOT NULL,

  -- 生成结果
  `main_image_url` TEXT,
  `main_image_asset_id` VARCHAR(255),
  `generated_result` TEXT,   -- 完整的回调JSON
  `first_video_url` TEXT,

  -- 状态管理
  `status` VARCHAR(50) NOT NULL DEFAULT 'CREATED',
  `error_msg` TEXT,
  `progress` VARCHAR(255),
  `deleted` TINYINT DEFAULT 0,

  -- 索引
  INDEX `idx_user_id` (`user_id`),
  INDEX `idx_channel_remote` (`channel_type`, `remote_project_id`),
  INDEX `idx_status` (`status`),
  INDEX `idx_deleted` (`deleted`),
  INDEX `idx_created_at` (`created_at`)
);
```

### 状态流转

```
CREATED                   # 已创建，等待调用上游
    ↓
COZE_RUNNING              # 工作流执行中
    ↓
VIDEO_PROCESSING          # 视频已生成，等待拼接
    ↓
VIDEO_CONCAT              # 拼接完成，等待上传
    ↓
ONE_CLICK_GENERATED       # 全流程完成（最终状态）

FAILED                    # 生成失败（终止状态）
VIDEO_PREPARING           # 拼接失败，需手动重试（需人工介入）
```

## 渠道适配器设计

### 接口定义

```go
type VideoGenerationAdapter interface {
    GetName() string
    CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error)
    GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error)
    ValidateWebhook(ctx context.Context, signature string, body []byte) error
    ParseWebhookPayload(body []byte) (*dto.WebhookPayload, error)
}
```

### Coze 渠道实现

- **API端点**：`https://api.coze.cn/v1/workflow/run`
- **认证方式**：Bearer Token
- **配置项**：
  - `COZE_API_KEY`：Coze API密钥
  - `COZE_WORKFLOW_ID`：工作流ID
  - `COZE_WEBHOOK_SECRET`：Webhook签名密钥
  - `COZE_BASE_URL`：API基础URL（默认：https://api.coze.cn）

### 三方平台渠道实现

- **API端点**：`{PLATFORM_BASE_URL}/api/video/create`
- **认证方式**：Bearer Token
- **配置项**：
  - `PLATFORM_BASE_URL`：平台API基础URL
  - `PLATFORM_API_KEY`：API密钥
  - `PLATFORM_API_SECRET`：Webhook签名密钥（可选）

## API接口规范

### 1. 创建视频项目

**接口**：`POST /api/video-generation/create`

**认证**：需要用户Token（UserAuth）

**请求体**：
```json
{
  "product_img_url": "https://oss.example.com/product.jpg",
  "brand": "品牌名称",
  "product_name": "产品名称",
  "prompt": "创意描述",
  "vtype": "产品展示",
  "duration": 30,
  "resolution": "2K",
  "whstr": "16:9",
  "channel_type": "platform"  // 可选，默认使用系统配置
}
```

**响应**：
```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "project_id": 123456,
    "project_name": "username_20260701_1719820800",
    "status": "CREATED",
    "created_at": 1719820800
  }
}
```

### 2. 获取项目详情

**接口**：`GET /api/video-generation/projects/:id`

**认证**：需要用户Token

**响应**：
```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "project_id": 123456,
    "project_name": "username_20260701_1719820800",
    "status": "VIDEO_PROCESSING",
    "progress": "50%",
    "main_image_url": "https://...",
    "first_video_url": "https://...",
    "created_at": 1719820800,
    "updated_at": 1719820900
  }
}
```

### 3. 获取项目列表

**接口**：`GET /api/video-generation/projects?page=1&page_size=10`

**认证**：需要用户Token

**响应**：
```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 10
  }
}
```

### 4. 删除项目

**接口**：`DELETE /api/video-generation/projects/:id`

**认证**：需要用户Token

**响应**：
```json
{
  "code": 200,
  "msg": "project deleted successfully",
  "data": null
}
```

### 5. 管理员获取所有项目

**接口**：`GET /api/video-generation/admin/projects?page=1&page_size=10&status=COZE_RUNNING`

**认证**：需要管理员Token

### 6. 管理员更新项目状态

**接口**：`PUT /api/video-generation/admin/projects/:id/status`

**认证**：需要管理员Token

**请求体**：
```json
{
  "status": "VIDEO_PROCESSING",
  "main_image_url": "https://...",
  "generated_result": "{...}"
}
```

### 7. 管理员删除项目

**接口**：`DELETE /api/video-generation/admin/projects/:id`

**认证**：需要管理员Token

### 8. Webhook 回调（统一入口）

**接口**：`POST /api/video-generation/webhook/:channel`

其中 `:channel` 可以是 `coze` 或 `platform`

**认证**：通过签名验证

**请求体（通用格式）**：
```json
{
  "remote_project_id": "abc-xyz-123",
  "status": "VIDEO_PROCESSING",
  "main_image_url": "https://...",
  "main_image_asset_id": "asset_xxx",
  "generated_result": "{...}",
  "error_msg": ""
}
```

## 核心业务流程

### 创建项目流程

```
1. Controller 接收请求，验证用户身份
2. Service 层根据 channel_type 选择适配器
3. 在本地数据库创建项目记录（status=CREATED）
4. 调用适配器的 CreateProject 方法
5. 适配器发送HTTP请求到上游平台
6. 上游返回 remote_project_id
7. 更新本地记录：
   - remote_project_id = 上游返回的ID
   - status = COZE_RUNNING
8. 返回响应给客户端
```

### Webhook 回调流程

```
1. 上游平台完成任务，发送 Webhook 到 new-api
2. Controller 接收回调请求
3. Service 根据 channel 参数选择适配器
4. 适配器验证签名（ValidateWebhook）
5. 适配器解析载荷（ParseWebhookPayload）
6. Service 通过 remote_project_id 查找本地项目
7. 更新本地数据库：
   - status = payload.status
   - main_image_url = payload.main_image_url
   - generated_result = payload.generated_result
8. 触发后续逻辑（计费、通知）
9. 返回 200 OK 给上游
```

### 状态同步流程（轮询兜底）

```
1. 定时任务每5分钟执行一次
2. 查询状态为 COZE_RUNNING 或 VIDEO_PROCESSING 的项目
3. 对每个项目：
   a. 调用适配器的 GetProjectStatus
   b. 对比远程状态和本地状态
   c. 如果不一致，更新本地数据库
4. 记录同步日志
```

## 配置管理

### 方式1：管理员后台（推荐）

系统设置 → Content → Video Generation，支持运行时动态配置，无需重启服务。

| 字段 | 说明 |
|------|------|
| Enable video generation | 全局开关 |
| Channel | 渠道选择：`platform`（三方平台）或 `coze` |
| **Platform 配置** | 选择 platform 渠道时显示 |
| Base URL | 三方平台 API 地址 |
| API Key | 三方平台密钥 |
| Webhook Secret | Webhook 签名验证密钥（可选） |
| **Coze 配置** | 选择 coze 渠道时显示 |
| API Key | Coze API 密钥 |
| Workflow ID | Coze 工作流 ID |
| Webhook Secret | Webhook 签名验证密钥 |
| Base URL | Coze API 地址（默认 https://api.coze.cn） |

页面底部还会显示当前渠道对应的 Webhook 回调地址，便于复制到上游平台配置。

### 方式2：环境变量（兜底初始值）

环境变量仅作为系统首次启动的初始默认值，后台保存后以数据库为准。

```bash
# Coze 配置
COZE_API_KEY=your_coze_api_key
COZE_WORKFLOW_ID=your_workflow_id
COZE_WEBHOOK_SECRET=your_webhook_secret
COZE_BASE_URL=https://api.coze.cn

# 三方平台配置
PLATFORM_BASE_URL=https://platform.example.com
PLATFORM_API_KEY=your_platform_api_key
PLATFORM_API_SECRET=your_platform_secret
```

### 配置优先级

1. **请求参数指定**：`req.channel_type`（每次请求可覆盖，优先级最高）
2. **数据库配置**：管理员后台保存的值（`common.VideoGenerationChannel`）
3. **默认值**：`platform`

### 配置存储实现

所有配置项通过 `model/option.go` 的 `OptionMap` 机制持久化到数据库，启动时加载到 `common` 包全局变量：

| 变量 | Option Key |
|------|-----------|
| `common.VideoGenerationEnabled` | `VideoGenerationEnabled` |
| `common.VideoGenerationChannel` | `VideoGenerationChannel` |
| `common.VideoGenerationPlatformBaseURL` | `VideoGenerationPlatformBaseURL` |
| `common.VideoGenerationPlatformApiKey` | `VideoGenerationPlatformApiKey` |
| `common.VideoGenerationPlatformApiSecret` | `VideoGenerationPlatformApiSecret` |
| `common.VideoGenerationCozeApiKey` | `VideoGenerationCozeApiKey` |
| `common.VideoGenerationCozeWorkflowId` | `VideoGenerationCozeWorkflowId` |
| `common.VideoGenerationCozeWebhookSecret` | `VideoGenerationCozeWebhookSecret` |
| `common.VideoGenerationCozeBaseURL` | `VideoGenerationCozeBaseURL` |

> **注意**：`ApiKey`、`Secret` 等敏感字段在 `GetOptions` 接口中会被过滤，不会返回到前端，符合现有安全规范。

## 数据一致性保障

### 问题

远程平台数据与本地数据可能不一致（网络故障、Webhook丢失等）

### 解决方案

1. **Webhook 优先**：正常情况下通过 Webhook 实时更新
2. **轮询兜底**：定时任务同步未完成项目的状态
3. **查询时同步**：用户查询项目详情时，如果状态是运行中，主动调用 GetProjectStatus 同步
4. **幂等性设计**：Webhook 可能重复推送，使用 `remote_project_id` + `status` 做幂等判断

## 扩展性设计

### 添加新渠道

1. 实现 `VideoGenerationAdapter` 接口
2. 在 `NewVideoGenerationService` 中添加 case 分支
3. 添加环境变量配置
4. 注册 Webhook 路由

示例：
```go
// service/video_adapter_newchannel.go
type NewChannelAdapter struct { ... }

func (a *NewChannelAdapter) CreateProject(...) { ... }
func (a *NewChannelAdapter) GetProjectStatus(...) { ... }
func (a *NewChannelAdapter) ValidateWebhook(...) { ... }
func (a *NewChannelAdapter) ParseWebhookPayload(...) { ... }

// service/video_generation_service.go
case "newchannel":
    adapter = NewNewChannelAdapter()
```

## 安全考虑

1. **Webhook 签名验证**：防止伪造回调
2. **用户隔离**：用户只能访问自己的项目
3. **管理员权限**：敏感操作需要管理员Token
4. **输入验证**：所有请求参数进行校验（binding标签）
5. **SQL注入防护**：使用GORM参数化查询

## 监控与日志

### 关键指标

- 项目创建成功率
- 平均生成时长
- Webhook 失败率
- 各渠道使用占比

### 日志记录

- 项目创建：用户ID、渠道类型、项目ID
- 渠道调用：请求/响应、耗时
- Webhook 回调：签名验证结果、载荷内容
- 状态变更：旧状态 → 新状态

## 未来优化方向

1. **异步队列**：将渠道调用放入消息队列（Redis/RabbitMQ）
2. **重试机制**：失败任务自动重试（指数退避）
3. **缓存优化**：热门项目状态缓存到Redis
4. **批量操作**：支持批量创建、批量删除
5. **用户配额**：限制用户并发项目数量
6. **计费集成**：根据视频时长、分辨率计费

## 部署说明

### 环境要求

- Go 1.22+
- MySQL/PostgreSQL/SQLite
- Redis（可选，用于缓存）

### 启动步骤

1. 配置环境变量（见上文）
2. 数据库自动迁移（AutoMigrate）
3. 启动 new-api 服务
4. 配置上游平台的 Webhook 回调地址

### Webhook 回调地址

- Coze：`https://your-domain.com/api/video-generation/webhook/coze`
- 三方平台：`https://your-domain.com/api/video-generation/webhook/platform`

---

**文档版本**：1.0  
**最后更新**：2026-07-01  
**维护者**：开发团队
