# Seedance 2.0 素材图（资产）测试脚本使用说明

`test-seedance-asset.sh` —— 上传资产图到素材库，再用 `asset://` 引用该素材生成视频。

对应实现：`controller/seedance.go`、`router/seedance-router.go`、`relay/channel/task/doubao/adaptor.go`
对应文档：[docs/咪咕/seedance-asset-simple.md](docs/咪咕/seedance-asset-simple.md)、[docs/seedance/多上游下游接口路径对照表.md](docs/seedance/多上游下游接口路径对照表.md)

## 与 test-seedance-simple.sh 的区别

| | test-seedance-simple.sh | test-seedance-asset.sh |
|---|---|---|
| 重点 | 走通视频生成全链路 | **素材图上传 → 激活 → 引用生成** |
| 素材接口 | 可选（`ROLE_IMAGE_URL`） | **核心**，含独立 `--asset-only` 模式 |
| 引用方式 | 仅 Ark `content` 格式 | `images[]` 标准格式 + Ark `content` 格式 |
| 模型默认值 | `doubao-seedance-2-0` | 同 |

## 快速开始

```bash
# 1. 只测素材上传激活（不产生生成费用，推荐先跑这个）
NEWAPI_API_KEY=sk-xxx \
ASSET_IMAGE_URL="https://your-cdn.com/role.png" \
./test-seedance-asset.sh --asset-only

# 2. 完整流程：上传资产图 → 激活 → 引用生成视频（计费）
NEWAPI_API_KEY=sk-xxx \
ASSET_IMAGE_URL="https://your-cdn.com/role.png" \
./test-seedance-asset.sh --run
```

打开帮助：`./test-seedance-asset.sh`

## 命令一览

| 命令 | 作用 | 是否计费 |
|---|---|---|
| `--asset-only` | 上传素材并轮询到 `Active`，打印可复用的 `ASSET_REF` | 否 |
| `--run` | 上传 + 激活 + 提交视频任务 + 轮询 + 查扣费 | **是** |
| `--query-asset <id>` | 查询素材状态（支持 LocalId 数字 / `asset-xxx` / `asset://asset-xxx`） | 否 |
| `--list-assets` | 查询素材列表 | 否 |
| `--list-groups` | 查询素材组列表 | 否 |

## 完整流程说明

```
Step 1  POST /api/seedance/assets          上传资产图（按 URL 引用）
Step 2  GET  /api/seedance/assets/{id}     轮询直到 Status=Active
Step 3  POST /v1/video/generations         用 asset://<asset_id> 生成
Step 4  GET  /v1/videos/{task_id}          轮询任务
Step 5  GET  /api/log/self                 查询本端扣费
```

**重要：素材库按 URL 建索引，不上传文件字节。** `AssetType` 只告诉上游这是什么类型的素材，实际内容由上游去拉取 `URL`。因此：

- 资产图必须是**公网可访问的 HTTPS 地址**（脚本会校验 `http(s)://` 前缀）
- 内网地址、`localhost`、本地文件路径都无法被上游拉取
- 同一 URL 已有 `Active` 素材时，后端直接复用，不重复上传（幂等）

## 环境变量

### 必需

| 变量 | 说明 |
|---|---|
| `NEWAPI_API_KEY` | API 密钥，`sk-` 开头 |
| `ASSET_IMAGE_URL` | 资产图公网 URL（用 `ASSET_ID` 复用时可不填） |

### 素材相关

| 变量 | 默认值 | 说明 |
|---|---|---|
| `ASSET_TYPE` | `Image` | `Image` / `Video` / `Audio` |
| `ASSET_NAME` | `asset-test-<时间戳>` | 素材名称 |
| `ASSET_ID` | 空 | 直接复用已有上游素材 ID，**设置后跳过上传** |
| `ASSET_API_STYLE` | `images` | `images`=标准格式；`content`=Ark 原生格式 |
| `ASSET_POLL_MAX` | `40` | 素材激活最大轮询次数 |
| `ASSET_POLL_INTERVAL` | `3` | 素材轮询间隔（秒） |

### 视频相关

| 变量 | 默认值 |
|---|---|
| `NEWAPI_BASE_URL` | `http://127.0.0.1:3000` |
| `MODEL` | `doubao-seedance-2-0` |
| `PROMPT` | `女子人物面对镜头自然说话` |
| `DURATION` | `4` |
| `RESOLUTION` | `480p` |
| `RATIO` | `16:9` |
| `GENERATE_AUDIO` | `true` |
| `WATERMARK` | `false` |
| `TASK_POLL_MAX` / `TASK_POLL_INTERVAL` | `90` / `10` |
| `QUOTA_PER_YUAN` | `500000` |

## 两种引用格式

`ASSET_API_STYLE=images`（默认，标准格式）：

```json
{
  "model": "doubao-seedance-2-0",
  "prompt": "女子人物面对镜头自然说话",
  "seconds": "4",
  "images": ["asset://asset-mock-001"],
  "metadata": { "resolution": "480p", "ratio": "16:9", "generate_audio": true, "watermark": false }
}
```

`ASSET_API_STYLE=content`（Ark 原生格式）：

```json
{
  "model": "doubao-seedance-2-0",
  "content": [
    { "type": "text", "text": "女子人物面对镜头自然说话" },
    { "type": "image_url", "image_url": { "url": "asset://asset-mock-001" } }
  ],
  "resolution": "480p", "ratio": "16:9", "duration": 4,
  "generate_audio": true, "watermark": false
}
```

两种格式的 `asset://` 引用都会由 `relay/channel/task/doubao/adaptor.go` 的 `convertToRequestPayload`
转成上游 `content[].image_url`，并自动补 `role: "reference_image"`。

## 典型用法

```bash
# 复用已激活的素材，跳过重复上传
ASSET_ID="asset-20260923121923-gtqgb" NEWAPI_API_KEY=sk-xxx \
PROMPT="参考图中人物自然说话" ./test-seedance-asset.sh --run

# 换模型 / 换分辨率
MODEL="doubao-seedance-2-0-260128" RESOLUTION=720p DURATION=5 \
NEWAPI_API_KEY=sk-xxx ASSET_IMAGE_URL="https://cdn.example.com/r.png" \
./test-seedance-asset.sh --run

# 素材激活慢时，先单独跑 asset-only，拿到 ID 后再生成
./test-seedance-asset.sh --asset-only          # 输出 ASSET_REF
./test-seedance-asset.sh --query-asset asset-xxx
ASSET_ID=asset-xxx ./test-seedance-asset.sh --run
```

## 退出码

| 码 | 含义 |
|---|---|
| `0` | 成功 |
| `1` | 失败（参数错误、上传失败、素材 Failed、生成失败） |
| `2` | 轮询超时，任务可能仍在处理（用 `task_id` 复查） |

## 常见问题

**资产图必须是 HTTPS 公网地址**
脚本会拒绝 `http(s)://` 以外的输入。本地图片需先传到图床或对象存储。

**素材激活超时**
上游处理通常 10–30 秒。超时不是失败，用 `--query-asset <id>` 稍后复查；状态变 `Active` 后可直接 `ASSET_ID=<id> --run`。

**同一 URL 重复上传不报错**
后端对同 URL 且 `Active` 的素材直接返回已有记录（幂等）。要强制重传，需在接口层显式传 `Force: true`（脚本暂未暴露该参数）。

**`images` 与 `content` 不要混用**
两种引用格式对应不同的请求体结构，用 `ASSET_API_STYLE` 二选一。
