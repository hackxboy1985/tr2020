# Doubao API 测试脚本使用说明

本目录包含 3 个测试脚本，用于测试 Doubao 相关接口。

## 测试脚本列表

### 1. test_doubao_complete_flow.sh（推荐）
**完整流程测试**，包含素材管理和视频生成的完整流程。

**测试内容：**
- ✅ 创建真人认证会话
- ✅ 获取认证结果（GroupId）
- ✅ 查询素材组信息
- ✅ 上传素材资产
- ✅ 查询素材状态
- ✅ 查询素材列表
- ✅ 使用素材生成视频
- ✅ 查询任务状态
- ✅ 查询任务列表
- ✅ 多条件筛选

**使用场景：** 测试完整的业务流程

### 2. test_doubao_official_api.sh
**视频生成接口测试**，专注测试 Doubao 官方视频生成接口。

**测试内容：**
- ✅ 创建视频任务
- ✅ 查询单个任务
- ✅ 查询任务列表
- ✅ 按状态筛选
- ✅ 按模型筛选
- ✅ 多条件筛选
- ✅ 参数验证
- ✅ 状态转换验证

**使用场景：** 只测试视频生成相关接口

### 3. test_asset_api.sh
**Asset API 测试**，专注测试素材管理接口。

**测试内容：**
- ✅ 创建真人认证会话
- ✅ 获取认证结果
- ✅ Asset Group 管理（CRUD）
- ✅ Asset 管理（CRUD）

**使用场景：** 只测试素材管理相关接口

## 使用步骤

### 1. 配置 API Key

编辑脚本文件，修改以下配置：

```bash
# 修改这两行
BASE_URL="http://localhost:3000"  # 改为实际的服务地址
API_KEY="sk-your-api-key-here"    # 改为实际的 API Key
```

### 2. 配置测试图片（仅 test_doubao_complete_flow.sh 需要）

```bash
# 修改测试图片 URL
TEST_IMAGE_URL="https://example.com/test-portrait.jpg"  # 改为实际的图片地址
```

### 3. 运行测试

```bash
# 运行完整流程测试
./test_doubao_complete_flow.sh

# 或运行视频生成接口测试
./test_doubao_official_api.sh

# 或运行素材管理接口测试
./test_asset_api.sh
```

## 接口路径对比

### Doubao 官方视频生成 API
```
POST   /api/v3/contents/generations/tasks              创建任务
GET    /api/v3/contents/generations/tasks/{task_id}    查询任务
GET    /api/v3/contents/generations/tasks              查询列表
DELETE /api/v3/contents/generations/tasks/{task_id}    取消任务（暂不支持）
```

### Seedance Asset API
```
POST /api/seedance/assets/v2/?Action=CreateVisualValidateSession&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=GetVisualValidateResult&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=CreateAssetGroup&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=GetAssetGroup&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=ListAssetGroups&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=UpdateAssetGroup&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=DeleteAssetGroup&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=CreateAsset&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=GetAsset&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=ListAssets&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=UpdateAsset&Version=2024-01-01
POST /api/seedance/assets/v2/?Action=DeleteAsset&Version=2024-01-01
```

## 状态值说明

### 视频任务状态（Doubao 官方格式）
- `queued` - 排队中
- `running` - 运行中
- `succeeded` - 成功
- `failed` - 失败
- `cancelled` - 已取消

### 素材状态
- `Processing` - 处理中
- `Active` - 可用（可用于生成视频）
- `Failed` - 处理失败

## 常见问题

### 1. 真人认证如何完成？
- 脚本会生成 H5 认证链接
- 在浏览器中打开链接
- 按提示完成人脸识别
- 完成后会跳转到 CallbackURL

### 2. 素材需要多久处理完成？
- 通常需要 10-60 秒
- 脚本会自动轮询查询状态
- 只有状态为 `Active` 才能用于视频生成

### 3. 视频生成需要多久？
- 5 秒视频通常需要 2-5 分钟
- 脚本会自动轮询查询状态
- 建议设置合理的超时时间

### 4. 如何使用自己的素材？
- 完成真人认证获取 GroupId
- 上传素材获取 AssetId
- 使用 `asset://{AssetId}` 格式引用素材
- 在 prompt 中使用"图片1"指代素材

## 依赖工具

- `curl` - 发送 HTTP 请求
- `jq` - JSON 解析（可选，用于美化输出）

如果没有安装 jq：
```bash
# macOS
brew install jq

# Ubuntu/Debian
sudo apt-get install jq
```

## 注意事项

1. **API Key 安全**：不要将包含真实 API Key 的脚本提交到代码仓库
2. **测试环境**：建议先在测试环境运行
3. **资源清理**：测试完成后记得清理测试数据
4. **并发限制**：注意 API 的 QPS 限制
5. **费用**：视频生成会消耗 token，请注意成本控制

## 故障排查

### 401 Unauthorized
- 检查 API_KEY 是否正确
- 检查 token 是否过期

### 404 Not Found
- 检查 BASE_URL 是否正确
- 检查路由是否已启用

### 503 Service Unavailable
- 检查渠道配置是否正确
- 检查上游服务是否可用

### Asset 状态一直是 Processing
- 检查图片 URL 是否可访问
- 检查图片格式是否符合要求
- 查看上游服务日志

## 更多信息

详细的 API 文档请参考：
- `docs/咪咕/doubao-official-api.zh-CN.md`
