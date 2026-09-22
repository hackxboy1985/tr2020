# 保存与下载周期（0.6.4）

## 生成视频：完成后保存 7 天

生成完成后，平台自动保存成片。保存期从 `completed_at` 起计算 **604800 秒（7 天）**，不会因查询、下载或重启重新计时。多输出任务会逐个保存。

查询 `GET /v1/videos/{id}`，结合以下字段判断：

| 字段 / 状态 | 含义与处理 |
|---|---|
| `status=completed` | 视频生成完成；保存可能还在进行 |
| `storage.status=not_ready` | 尚未生成成功 |
| `storage.status=pending` | 正在等待或进行自动保存；可稍后查询 |
| `storage.status=ready` | 全部输出已保存；可在七天期限内通过认证接口下载 |
| `storage.status=unavailable` | 自动保存暂不可用，平台会重试；原内容尚有效时仍可下载。若持续出现，请保留任务 ID 联系平台，不重复付费生成 |
| `storage.status=expired` | 七天保存期已结束 |
| `storage.retention_seconds` | 固定为 `604800` |
| `storage.expires_at` | 以完成时间计算的目标保存到期时间；pending/unavailable 时不表示文件已成功保存 |
| `expires_at` | ready 时为七天到期时间；尚未保存完成时为已知原内容到期时间或 null；到期后仍保留七天截止时间 |

示例：

```json
{
  "id": "task_example",
  "status": "completed",
  "completed_at": 1789952460,
  "expires_at": 1790557260,
  "storage": {
    "status": "ready",
    "retention_seconds": 604800,
    "expires_at": 1790557260
  },
  "url": "/v1/videos/task_example/content"
}
```

下载地址拼接 `API_BASE`，使用所属用户的有效 Bearer Key。支持 GET、HEAD 和单段 Range；不能把该地址当作免认证的分享链接。查询和下载仍执行用户、模型权限及任务隐藏检查。

```bash
curl "$API_BASE/v1/videos/$TASK_ID" -H "Authorization: Bearer $API_KEY"
curl -I "$API_BASE/v1/videos/$TASK_ID/content" -H "Authorization: Bearer $API_KEY"
curl "$API_BASE/v1/videos/$TASK_ID/content" \
  -H "Authorization: Bearer $API_KEY" --output video.mp4
```

七天到期后下载返回 **HTTP 410，`content_expired`**，平台周期清理保存的成片。任务记录仍可查询，`status` 仍表示原生成结果，不改为 failed。需要长期保存的客户应在到期前下载至自己的存储。

已保存的成片不再依赖原内容链接有效期。升级前已经失效、且没有成功保存的文件不能自动恢复；历史任务是否可用以实际 `storage` 状态和下载结果为准，不因升级统一延长七天。删除任务仍是终态隐藏，不取消生成、不触发退款，也不重新计算保存时间。

回调通知中的 `storage` 是通知生成时的状态；如为 pending，请继续查询任务。回调重试正文保持不变。

## 参考素材：容量不变，读取签名有效 7 天

- 单文件上传上限：**64 MiB**。
- 每用户参考素材累计上限：**512 MiB（536870912 字节）**；同一用户多个 Key 共用该容量。
- 上传文件供服务端读取的签名链接：从签发起有效 **7 天（604800 秒）**。旧的 24 小时链接保持原到期时间，不会自动续期。
- 签名链接到期不会自动删除已上传的素材文件，也不等于 `asset://task_…` 素材 ID 到期。是否可继续用于生成，请查询素材状态并做目标模型预检查。
- 持有签名链接者在有效期内可读取对应文件；不要分享或记录到公开日志。客户通常使用素材 ID，无需管理内部读取链接。
- 生成视频的七天保存空间与上述参考素材容量分别计算。

当前限制可以通过 `GET /seedance/capabilities` 查看：

```json
{
  "tasks": {"video_retention_seconds":604800},
  "limits": {
    "max_upload_bytes":67108864,
    "user_storage_bytes":536870912,
    "upload_link_ttl_seconds":604800,
    "base64_image_bytes":2097152
  }
}
```

示例仅列出相关字段；实际响应还包括模型及其他能力。
