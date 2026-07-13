# Seedance 素材库与人脸库接口

Base URL：

```text
https://sd.dawnloadai.com:9444
```

## 鉴权

使用与 NewAPI 视频生成接口相同的 API Token：

```http
Authorization: Bearer sk-<NewAPI Token>
Content-Type: application/json
```

Token 的启用状态、过期时间、额度和用户状态由 NewAPI 统一校验。素材库和人脸库请求不额外扣减视频生成额度。

Gateway 根据 Token 所属的 NewAPI 用户选择对应的人脸与素材项目。同一用户创建的多个 Token 共用该用户的项目配置；不同用户的数据和认证任务相互隔离。客户端不得提交或覆盖 `ProjectName` 和上游密钥。

## 功能范围

```markdown
vip65
```

本网关只提供素材库和人脸库访问，不提供视频生成、视频任务查询或模型调用接口。

对外开放的路径：

```text
/api/seedance/proxy/assets
/api/seedance/proxy/assets/*
/api/seedance/face-verifications
/api/seedance/face-verifications/{verification_id}
```

请求体和上游响应按 JSON 原样透传。客户端不需要、也不应传递 ProjectName、Ark API Key、Access Key ID 或 Secret Access Key。

## 素材库

### 素材组

```http
POST /api/seedance/proxy/assets/groups
GET /api/seedance/proxy/assets/groups
GET|PUT|PATCH|DELETE /api/seedance/proxy/assets/groups/{group_id}
```

### 素材

```http
POST /api/seedance/proxy/assets
GET /api/seedance/proxy/assets
GET|PUT|PATCH|DELETE /api/seedance/proxy/assets/{asset_id}
```

示例：

```bash
curl -X GET \
  'https://sd.dawnloadai.com:9444/api/seedance/proxy/assets/<asset_id>' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'
```

## 人脸库认证工作流

### 创建认证任务

```http
POST /api/seedance/face-verifications
```

请求体可选传入 `return_url`。响应包含 `verification_id`、`h5_url`、`status` 和 `expires_at`。

### 查询认证任务

```http
GET /api/seedance/face-verifications/{verification_id}
```

调用方不需要传递 `BytedToken`。Gateway 只会在收到 H5 成功回调后查询上游结果。

认证成功后响应中的 `group_id` 是该用户项目下的真人素材组 ID。调用方使用同一用户的 Token 调用 `POST /api/seedance/proxy/assets`，将图片、视频或音频加入该组。完整可执行示例见 [人脸认证与真人素材调用 Demo](./FACE-ASSET-DEMO.zh-CN.md)。

## 错误码

| HTTP 状态码 | 错误码                            | 说明                    |
| -------- | ------------------------------ | --------------------- |
| `401`    | `unauthorized`                 | Token 缺失、无效，或对应用户不可用  |
| `404`    | `unsupported_seedance_route`   | 路径不在素材库/人脸库白名单中       |
| `410`    | `face_workflow_required`       | 旧的原始人脸校验接口已禁用         |
| `413`    | `request_body_too_large`       | 请求体超过限制               |
| `502`    | `upstream_request_failed`      | 上游请求失败                |
| `503`    | `auth_backend_unavailable`     | NewAPI 鉴权服务暂时不可用      |
| `503`    | `user_upstream_not_configured` | Token 所属用户尚未配置人脸与素材项目 |
| `504`    | `upstream_timeout`             | 上游请求超时                |

上游业务响应保持原状态码和原响应体。

## 健康检查

```http
GET /healthz
```

健康检查无需 Token。
