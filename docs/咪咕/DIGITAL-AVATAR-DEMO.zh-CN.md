# 数字人形象库调用 Demo

数字人或虚拟形象使用 `GroupType=AIGC`，不需要真人 H5 活体认证。形象库、视频生成共用同一个 NewAPI Token，调用方不传递 `ProjectName`、Ark API Key、Access Key ID 或 Secret Access Key。

## 1. 准备参数

本示例依赖 `curl` 和 `jq`。

```bash
export GATEWAY_BASE_URL='https://sd.dawnloadai.com:9444'
export NEWAPI_BASE_URL='https://sd.dawnloadai.com:8443'
export NEWAPI_TOKEN='sk-替换为客户自己的令牌'
export AVATAR_IMAGE_URL='https://example.com/public/avatar.jpg'
```

`AVATAR_IMAGE_URL` 必须是火山引擎能够直接下载的公网 HTTPS 地址，不能是本地路径、内网地址或需要登录的 URL。

## 2. 创建数字人形象组

```bash
GROUP_RESPONSE=$(curl -fsS -X POST \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets/groups" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "Name": "avatar-001",
    "Description": "品牌数字人形象",
    "GroupType": "AIGC"
  }')

GROUP_ID=$(printf '%s' "$GROUP_RESPONSE" | jq -r '.Result.Id')
printf 'group_id=%s\n' "$GROUP_ID"
```

`GroupType` 必须为 `AIGC`。响应中的 `Result.Id` 是该数字人的形象组 ID。

## 3. 添加形象素材

```bash
ASSET_RESPONSE=$(curl -fsS -X POST \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg groupId "$GROUP_ID" \
    --arg url "$AVATAR_IMAGE_URL" \
    '{GroupId:$groupId,URL:$url,AssetType:"Image",Name:"avatar-front"}')")

ASSET_ID=$(printf '%s' "$ASSET_RESPONSE" | jq -r '.Result.Id')
printf 'asset_id=%s\n' "$ASSET_ID"
```

`AssetType` 支持 `Image`、`Video` 和 `Audio`。创建成功只表示进入预处理队列，暂时不能直接用于生成视频。

## 4. 等待素材可用

```bash
while true; do
  ASSET_RESPONSE=$(curl -fsS \
    "$GATEWAY_BASE_URL/api/seedance/proxy/assets/$ASSET_ID" \
    -H "Authorization: Bearer $NEWAPI_TOKEN")

  STATUS=$(printf '%s' "$ASSET_RESPONSE" | jq -r '.Result.Status')
  printf 'asset status=%s\n' "$STATUS"

  case "$STATUS" in
    Active)
      break
      ;;
    Failed)
      printf '%s\n' "$ASSET_RESPONSE" | jq .
      exit 1
      ;;
  esac
  sleep 3
done
```

常见状态：

| 状态 | 说明 |
| --- | --- |
| `Processing` | 正在预处理和审核 |
| `Active` | 已可用于 Seedance 视频生成 |
| `Failed` | 处理失败，查看响应中的错误信息 |

## 5. 在 Seedance 2.0 中引用

素材可用后的引用值为：

```bash
printf 'asset://%s\n' "$ASSET_ID"
```

创建视频时将其放入 `images`：

```json
{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "数字人面对镜头自然介绍产品",
  "seconds": "5",
  "images": [
    "asset://asset-xxxxxxxx"
  ],
  "metadata": {
    "resolution": "720p",
    "ratio": "16:9"
  }
}
```

视频生成请求发送到 `$NEWAPI_BASE_URL/v1/video/generations`，不发送到素材 Gateway：

```bash
curl -fsS -X POST \
  "$NEWAPI_BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -nc \
    --arg asset "asset://$ASSET_ID" \
    '{
      model:"doubao-seedance-2-0-260128",
      prompt:"数字人面对镜头自然介绍产品",
      seconds:"5",
      images:[$asset],
      metadata:{resolution:"720p",ratio:"16:9"}
    }')" | jq .
```

## 6. 查询与清理

查询 AIGC 形象组和素材：

```bash
curl -fsS \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets/groups?GroupType=AIGC&PageNumber=1&PageSize=20" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" | jq .

curl -fsS \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets?GroupType=AIGC&PageNumber=1&PageSize=20" \
  -H "Authorization: Bearer $NEWAPI_TOKEN" | jq .
```

不再使用时，先删除素材，再删除形象组：

```bash
curl -fsS -X DELETE \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets/$ASSET_ID" \
  -H "Authorization: Bearer $NEWAPI_TOKEN"

curl -fsS -X DELETE \
  "$GATEWAY_BASE_URL/api/seedance/proxy/assets/groups/$GROUP_ID" \
  -H "Authorization: Bearer $NEWAPI_TOKEN"
```

## 与真人人像库的区别

| 类型 | `GroupType` | 是否需要 H5 活体认证 | 形象组来源 |
| --- | --- | --- | --- |
| 数字人/虚拟形象 | `AIGC` | 否 | 调用创建形象组接口 |
| 真实人物 | `LivenessFace` | 是 | 活体认证成功后由上游返回 |
