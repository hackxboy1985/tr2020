# Seedance 2.0 测试脚本使用说明

## 快速开始

### 1. 显示帮助信息

```bash
./test-seedance-simple.sh
```

### 2. 执行测试（RESTful 格式，默认）

```bash
./test-seedance-simple.sh --execute
```

---

## 渠道配置说明

在执行测试前，需要先配置渠道。

### Gateway 上游（咪咕）

**最简配置**（推荐）：
```json
{
  "asset_upstream_version": "gateway",
  "seedance_asset_base_url": "https://sd.dawnloadai.com:9444"
}
```

**完整配置**（可选）：
```json
{
  "asset_upstream_version": "gateway",
  "seedance_asset_base_url": "https://sd.dawnloadai.com:9444",
  "seedance_relay_mode": false,
  "doubao_video_generate_path": "/api/v3/contents/generations/tasks",
  "doubao_video_fetch_path": "/api/v3/contents/generations/tasks"
}
```

---

### KWJM 上游

**最简配置**（推荐）：
```json
{
  "asset_upstream_version": "kwjm",
  "kwjm_asset_base_url": "https://kwjm.com",
  "kwjm_asset_model": "sd-video-v2"
}
```

**完整配置**（可选）：
```json
{
  "asset_upstream_version": "kwjm",
  "kwjm_asset_base_url": "https://kwjm.com",
  "kwjm_asset_model": "sd-video-v2",
  "doubao_video_generate_path": "/v1/videos/generations",
  "doubao_video_fetch_path": "/v1/videos/generations"
}
```

---

### 配置字段说明

| 上游 | 必填字段 | 说明 |
|------|---------|------|
| **Gateway** | `asset_upstream_version` | 值为 `"gateway"` |
| | `seedance_asset_base_url` | Gateway 资产库地址，如 `https://sd.dawnloadai.com:9444` |
| **KWJM** | `asset_upstream_version` | 值为 `"kwjm"` |
| | `kwjm_asset_base_url` | KWJM 资产库地址，如 `https://kwjm.com` |
| | `kwjm_asset_model` | KWJM 默认模型，如 `sd-video-v2` |

**注意**：`doubao_video_generate_path` 和 `doubao_video_fetch_path` 会根据 `asset_upstream_version` 自动推断，无需手动配置。

---

## 资产接口格式说明

### RESTful 格式（推荐）

**适用场景**：
- Gateway 上游（咪咕）
- KWJM 上游的 RESTful 模式
- 新 API 标准接口

**接口路径**：
- 创建素材：`POST /api/seedance/assets`
- 查询素材：`GET /api/seedance/assets/{id}`

**使用方法**：
```bash
./test-seedance-simple.sh --execute
# 或显式指定
ASSET_API_FORMAT=restful ./test-seedance-simple.sh --execute
```

---

### Action 格式

**适用场景**：
- 火山官方 API
- KWJM 上游的 Action 模式
- 兼容旧版 API

**接口路径**：
- 创建素材：`POST /api/seedance/assets/v2/?Action=CreateAsset&Version=2024-01-01`
- 查询素材：`POST /api/seedance/assets/v2/?Action=GetAsset&Version=2024-01-01`

**使用方法**：
```bash
ASSET_API_FORMAT=action ./test-seedance-simple.sh --execute
```

---

## 环境变量配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `NEWAPI_BASE_URL` | 本地 new-api 地址 | `http://book2:3002` |
| `NEWAPI_API_KEY` | API 密钥 | `sk-zoPYrUW81cYIFdmcD8JHvhLtGWdTvKh41vBBJW8KSBvzWxVu` |
| `MODEL` | 视频模型 | `doubao-seedance-2-0-sd` |
| `ASSET_API_FORMAT` | 资产接口格式 | `restful` |
| `PROMPT` | 提示词 | 女子人物面对镜头自然说话... |
| `DURATION` | 视频时长（秒） | `4` |
| `RESOLUTION` | 分辨率 | `480p` |
| `RATIO` | 宽高比 | `16:9` |
| `GENERATE_AUDIO` | 生成音频 | `true` |
| `WATERMARK` | 添加水印 | `false` |
| `ROLE_IMAGE_URL` | 角色图片 URL | (需上传到资产库) |
| `IMAGE_URL` | 场景图片 URL | - |
| `AUDIO_URL` | 参考音频 URL | - |
| `VIDEO_URL` | 参考视频 URL | - |

---

## 使用示例

### 示例 1：测试默认配置

```bash
./test-seedance-simple.sh --execute
```

### 示例 2：测试 Action 格式

```bash
ASSET_API_FORMAT=action ./test-seedance-simple.sh --execute
```

### 示例 3：自定义角色图和场景图

```bash
ROLE_IMAGE_URL="https://example.com/role.png" \
IMAGE_URL="https://example.com/scene.png" \
./test-seedance-simple.sh --execute
```

### 示例 4：测试高清模型

```bash
MODEL="doubao-seedance-2-0-hd" \
RESOLUTION="720p" \
DURATION=6 \
./test-seedance-simple.sh --execute
```

### 示例 5：测试不同的 API 服务器

```bash
NEWAPI_BASE_URL="http://localhost:3000" \
NEWAPI_API_KEY="sk-your-api-key" \
./test-seedance-simple.sh --execute
```

### 示例 6：完整自定义配置

```bash
NEWAPI_BASE_URL="http://localhost:3000" \
NEWAPI_API_KEY="sk-your-key" \
MODEL="doubao-seedance-2-0-hd" \
ASSET_API_FORMAT="action" \
PROMPT="一位女性微笑着向镜头挥手" \
DURATION=5 \
RESOLUTION="720p" \
RATIO="9:16" \
ROLE_IMAGE_URL="https://example.com/role.png" \
./test-seedance-simple.sh --execute
```

---

## 测试流程

脚本会按以下步骤执行：

1. **Step 0：上传角色图到资产库**
   - 如果指定了 `ROLE_IMAGE_URL`，自动上传到资产库
   - 等待素材激活（10-30秒）
   - 获取 `asset://xxx` 引用

2. **Step 1：提交视频生成任务**
   - 调用 `POST /v1/video/generations`
   - 提交提示词、素材引用、参数等
   - 获取 `task_id`

3. **Step 2：轮询任务状态**
   - 调用 `GET /v1/videos/{task_id}`
   - 每 10 秒轮询一次
   - 等待状态变为 `Succeed`（1-3分钟）

4. **Step 3：下载视频（可选）**
   - 获取视频 URL
   - 显示完成信息

---

## 注意事项

1. **素材激活时间**：
   - 首次上传素材可能需要 10-30 秒
   - 如果超时，稍后重试

2. **视频生成时间**：
   - 4 秒视频：约 1-2 分钟
   - 6 秒视频：约 2-3 分钟
   - 具体时间取决于服务器负载

3. **接口格式选择**：
   - **推荐使用 `restful` 格式**（默认）
   - 只有在对接旧版 API 时才使用 `action` 格式

4. **网络要求**：
   - 需要能访问 `NEWAPI_BASE_URL`
   - 需要能访问素材 URL（图片、音频等）

5. **依赖工具**：
   - `curl` - HTTP 请求
   - `jq` - JSON 解析
   - `bc` - 计算（可选）

---

## 故障排查

### 问题 1：素材上传失败

**现象**：
```
[ERROR] 上传素材失败，未获取到 asset_id
```

**原因**：
- 网络问题
- 素材 URL 无法访问
- API 密钥错误

**解决**：
1. 检查网络连接
2. 测试素材 URL 是否可访问：`curl -I $ROLE_IMAGE_URL`
3. 验证 API 密钥是否正确

---

### 问题 2：素材激活超时

**现象**：
```
[ERROR] 素材激活超时，退出
```

**原因**：
- 素材处理时间过长
- 上游服务繁忙

**解决**：
1. 稍后重试
2. 检查上游服务状态
3. 查看后端日志

---

### 问题 3：视频生成失败

**现象**：
```
任务状态: Failed
```

**原因**：
- 提示词不合规
- 素材不符合要求
- 参数配置错误

**解决**：
1. 检查提示词是否合规
2. 确认素材已激活
3. 检查参数配置（分辨率、时长等）
4. 查看后端日志获取详细错误信息

---

## 高级用法

### 批量测试不同模型

```bash
for model in doubao-seedance-2-0-sd doubao-seedance-2-0-hd; do
  echo "Testing model: $model"
  MODEL="$model" ./test-seedance-simple.sh --execute
  sleep 10
done
```

### 对比两种接口格式

```bash
# 测试 RESTful 格式
echo "=== Testing RESTful format ==="
ASSET_API_FORMAT=restful ./test-seedance-simple.sh --execute

# 测试 Action 格式
echo "=== Testing Action format ==="
ASSET_API_FORMAT=action ./test-seedance-simple.sh --execute
```

### 保存测试日志

```bash
./test-seedance-simple.sh --execute 2>&1 | tee test-$(date +%Y%m%d-%H%M%S).log
```

---

## 相关文档

- [ARK-VIDEO-API 文档](docs/咪咕/ARK-VIDEO-API.zh-CN.md)
- [KWJM 素材库文档](docs/kwjm/素材.md)
- [渠道配置说明](docs/channel-config.md)

---

## 更新日志

### 2026-09-05
- ✅ 增加 `--execute` 参数，默认显示帮助信息
- ✅ 增加 `ASSET_API_FORMAT` 参数，支持 RESTful 和 Action 两种格式
- ✅ 优化帮助信息，详细说明使用方法和示例
- ✅ 完善错误处理和状态提示

### 2026-01-28
- 初始版本发布
