# 广告视频生成系统启动测试指南

## 前置条件

1. ✅ 代码已实现完成
2. ✅ 代码完整性检查通过
3. ⚠️ 需要配置上游API密钥

## 快速启动步骤

### 步骤1: 运行代码完整性检查

```bash
cd /Users/mac/Desktop/ecap/new-api
bash check-video-impl.sh
```

**预期结果**: 所有检查项显示 ✓

### 步骤2: 配置环境变量

复制配置模板:
```bash
cp .env.video-test.example .env.video-test
```

编辑 `.env.video-test`，选择一种渠道配置:

**选项A - 三方平台渠道（推荐）:**
```bash
VIDEO_GENERATION_CHANNEL=platform
PLATFORM_BASE_URL=https://your-platform.com
PLATFORM_API_KEY=your_api_key
PLATFORM_API_SECRET=your_secret
```

**选项B - Coze渠道:**
```bash
VIDEO_GENERATION_CHANNEL=coze
COZE_API_KEY=your_coze_key
COZE_WORKFLOW_ID=your_workflow_id
COZE_WEBHOOK_SECRET=your_secret
```

### 步骤3: 启动服务

**方式1 - Docker（推荐）:**
```bash
# 加载测试配置
export $(cat .env.video-test | xargs)

# 构建并启动
./start.sh
```

**方式2 - 直接运行（需要Go环境）:**
```bash
# 加载测试配置
export $(cat .env.video-test | xargs)

# 编译
go build -o new-api main.go

# 运行
./new-api
```

### 步骤4: 验证服务启动

检查服务是否正常运行:
```bash
curl http://localhost:3000/api/status
```

查看数据库表是否创建:
```bash
# 连接数据库查看
# MySQL示例:
mysql -u root -p -e "SHOW TABLES LIKE 'video_projects';" new-api
```

**预期结果**: 
- HTTP 200响应
- `video_projects` 表已创建

### 步骤5: 获取测试Token

登录系统获取用户Token:
```bash
# 方法1: 通过Web界面登录
# 访问 http://localhost:3000
# 登录后从浏览器开发者工具中获取token

# 方法2: 通过API登录
curl -X POST http://localhost:3000/api/user/login \
  -H "Content-Type: application/json" \
  -d '{"username":"root","password":"123456"}'
```

将返回的token设置为环境变量:
```bash
export USER_TOKEN="sk-xxxxxxxxxxxxx"
```

### 步骤6: 运行API测试

```bash
bash test-video-api.sh
```

或带token参数运行:
```bash
bash test-video-api.sh "sk-xxxxxxxxxxxxx"
```

**测试内容**:
1. ✓ 创建视频项目
2. ✓ 获取项目详情
3. ✓ 获取项目列表
4. ✓ 删除项目

## 测试场景说明

### 场景1: 完整工作流测试

1. **创建项目** → 本地数据库创建记录 + 调用上游API
2. **上游处理** → Coze/Platform开始生成视频
3. **Webhook回调** → 上游完成后回调更新状态
4. **查询详情** → 获取最新状态和结果

### 场景2: Webhook测试

配置上游平台的Webhook地址:
- Coze: `https://your-domain.com/api/video-generation/webhook/coze`
- Platform: `https://your-domain.com/api/video-generation/webhook/platform`

模拟Webhook调用:
```bash
curl -X POST http://localhost:3000/api/video-generation/webhook/platform \
  -H "Content-Type: application/json" \
  -H "X-Signature: your_signature" \
  -d '{
    "remote_project_id": "remote_id_123",
    "status": "VIDEO_PROCESSING",
    "progress": "50%",
    "main_image_url": "https://example.com/main.jpg"
  }'
```

### 场景3: 状态同步测试

主动查询项目状态（会自动同步上游状态）:
```bash
curl -H "Authorization: Bearer $USER_TOKEN" \
  http://localhost:3000/api/video-generation/projects/123456
```

## 预期日志输出

启动成功时应看到:
```
[GIN-debug] POST   /api/video-generation/create
[GIN-debug] GET    /api/video-generation/projects
[GIN-debug] GET    /api/video-generation/projects/:id
[GIN-debug] DELETE /api/video-generation/projects/:id
[GIN-debug] POST   /api/video-generation/webhook/:channel
...
[INFO] Server started on :3000
[INFO] VideoProject table migrated successfully
```

## 常见问题排查

### 1. 表未创建
**症状**: 查询时报错 "table video_projects doesn't exist"

**解决**:
- 检查 `model/main.go` 的 AutoMigrate 中是否包含 `&VideoProject{}`
- 重启服务触发迁移

### 2. 路由404
**症状**: 访问 `/api/video-generation/*` 返回404

**解决**:
- 检查 `router/video-router.go` 是否被正确引入
- 确认 `SetVideoRouter()` 在 `main.go` 中被调用

### 3. 上游API调用失败
**症状**: 创建项目时报错 "failed to call adapter"

**解决**:
- 检查环境变量配置是否正确
- 验证API密钥是否有效
- 检查网络连接

### 4. Webhook签名验证失败
**症状**: webhook回调返回 "invalid signature"

**解决**:
- 确认 `COZE_WEBHOOK_SECRET` 或 `PLATFORM_API_SECRET` 配置正确
- 检查上游平台的签名算法是否为 HMAC-SHA256
- 临时禁用签名验证用于调试

## 调试模式

启用详细日志:
```bash
export LOG_LEVEL=debug
export GIN_MODE=debug
```

查看SQL查询:
```bash
export LOG_SQL=true
```

## 性能监控

查看API响应时间:
```bash
# 创建项目
time curl -X POST http://localhost:3000/api/video-generation/create ...

# 查询详情
time curl http://localhost:3000/api/video-generation/projects/123456 ...
```

## 数据库查询示例

```sql
-- 查看所有项目
SELECT id, project_name, status, channel_type, created_at 
FROM video_projects 
ORDER BY created_at DESC 
LIMIT 10;

-- 查看特定状态的项目
SELECT COUNT(*) 
FROM video_projects 
WHERE status = 'RUNNING' AND deleted = 0;

-- 查看用户的项目
SELECT * 
FROM video_projects 
WHERE user_id = 1 AND deleted = 0;
```

## 下一步

1. ✅ 完成基础功能测试
2. 🔄 配置真实的上游API密钥
3. 🔄 测试完整的视频生成工作流
4. 🔄 配置Webhook回调
5. 🔄 前端集成
6. 🔄 生产环境部署

---

**文档版本**: 1.0  
**更新时间**: 2026-07-01  
**状态**: ✅ 代码实现完成，等待配置和测试
