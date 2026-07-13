# 人脸认证与真人素材调用 Demo

## 1. 准备参数

人脸认证、素材库和视频生成使用同一个 NewAPI API Key。调用方不需要传入 `ProjectName`、AccessKey ID 或 Secret Access Key，Gateway 会根据 API Key 所属用户自动选择项目配置。

```bash
export GATEWAY_BASE_URL='https://sd.dawnloadai.com:9444'
export NEWAPI_TOKEN='sk-替换为客户自己的令牌'
export FACE_IMAGE_URL='https://example.com/public/face.jpg'
```

`FACE_IMAGE_URL` 必须是火山引擎能够直接下载的公网 HTTPS 地址，不能使用本地文件路径、内网地址或需要登录的 URL。

## 2. 创建人脸认证任务

```bash
CREATE_RESPONSE=$(curl -fsS -X POST \
  "$GATEWAY_BASE_URL/api/seedance/face-verifications" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{}')

VERIFICATION_ID=$(printf '%s' "$CREATE_RESPONSE" | jq -r '.verification_id')
H5_URL=$(printf '%s' "$CREATE_RESPONSE" | jq -r '.h5_url')

printf 'verification_id=%s\nh5_url=%s\n' "$VERIFICATION_ID" "$H5_URL"
```

前端应跳转或展示 `h5_url`，由终端用户按页面提示完成人脸认证。不要在用户完成 H5 流程前调用素材创建接口。

需要认证结束后跳回客户页面时，创建任务可以传入：

```json
{
  "return_url": "https://customer.example.com/face-result"
}
```

Gateway 跳转时只会附加 `verification_id` 和 `status`。

## 3. 轮询认证结果

```bash
while true; do
  VERIFY_RESPONSE=$(curl -fsS \
    "$GATEWAY_BASE_URL/api/seedance/face-verifications/$VERIFICATION_ID" \
    -H "Authorization: Bearer $NEWAPI_TOKEN")

  STATUS=$(printf '%s' "$VERIFY_RESPONSE" | jq -r '.status')
  printf 'face status=%s\n' "$STATUS"

  case "$STATUS" in
    verified)
      GROUP_ID=$(printf '%s' "$VERIFY_RESPONSE" | jq -r '.group_id')
      break
      ;;
    failed|expired)
      printf '%s\n' "$VERIFY_RESPONSE" | jq .
      exit 1
      ;;
  esac
  sleep 3
done

printf 'group_id=%s\n' "$GROUP_ID"
```

查询认证任务必须继续使用创建任务时同一用户的 API Key。其他用户的 API Key 查询时返回 `404`。

## 4. 向真人组添加素材

```bash
ASSET_RESPONSE=$(curl -fsS -X POST \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg groupId "$GROUP_ID" \
    --arg url "$FACE_IMAGE_URL" \
    '{GroupId:$groupId,URL:$url,AssetType:"Image",Name:"verified-face"}')")

ASSET_ID=$(printf '%s' "$ASSET_RESPONSE" | jq -r '.Result.Id')
printf 'asset_id=%s\n' "$ASSET_ID"
```

`AssetType` 可以是 `Image`、`Video` 或 `Audio`。文件内容必须属于已完成人脸认证的本人，并满足上游素材要求。

## 5. 等待素材可用

```bash
while true; do
  ASSET_STATUS_RESPONSE=$(curl -fsS \
    "$GATEWAY_BASE_URL/api/seedance/proxy/assets/$ASSET_ID" \
    -H "Authorization: Bearer $NEWAPI_TOKEN")

  ASSET_STATUS=$(printf '%s' "$ASSET_STATUS_RESPONSE" | jq -r '.Result.Status')
  printf 'asset status=%s\n' "$ASSET_STATUS"

  case "$ASSET_STATUS" in
    Active)
      break
      ;;
    Failed)
      printf '%s\n' "$ASSET_STATUS_RESPONSE" | jq .
      exit 1
      ;;
  esac
  sleep 3
done

printf '视频生成引用值：asset://%s\n' "$ASSET_ID"
```

素材状态变为 `Active` 后，才可以在 NewAPI 视频生成请求中使用 `asset://<asset_id>`。

## 状态说明

| 对象 | 状态 | 说明 |
| --- | --- | --- |
| 人脸认证 | `waiting_user` | 等待用户完成 H5 认证 |
| 人脸认证 | `callback_received` | Gateway 已收到认证回调 |
| 人脸认证 | `resolving` | 正在获取人脸组结果 |
| 人脸认证 | `verified` | 成功，可读取 `group_id` |
| 人脸认证 | `failed` | 认证失败 |
| 人脸认证 | `expired` | 认证任务已过期，需要重新创建 |
| 素材 | `Processing` | 上游正在预处理 |
| 素材 | `Active` | 素材可用于视频生成 |
| 素材 | `Failed` | 素材处理失败 |
