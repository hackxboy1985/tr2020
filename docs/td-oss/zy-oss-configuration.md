# Td / Zy 对象存储配置补充

## Zy 渠道开关

每个 Zy 渠道独立配置，默认关闭，不能用 Td 开关代替：

```json
{
  "zy_oss_image_enabled": true,
  "zy_oss_video_enabled": false
}
```

可在 Zy 渠道配置的可视化界面切换，或编辑渠道 other JSON。所有渠道共用部署环境中的 `S3_*` 配置。

Zy 图片结果优先读取顶层 `image_url`，其次 `url`；视频结果优先读取 `video_url`，其次 `url`。相同地址去重，单结果任务只转存最高优先级地址。旧版 `metadata.image_url` / `metadata.image` 保留为图片兼容兜底。

图片响应可能带 `object=video` 和 `video_url` 别名，不能仅凭这些字段判为视频。图片字段、结果文件扩展名以及图片模型标记用于区分媒体；实时转存和失败补扫共用判据。

## MinIO 公网地址

公网基础地址为域名时，自动在对象键前追加桶名：

```text
S3_PUBLIC_BASE_URL=http://oss.luluai.cc
S3_BUCKET=hk-a

结果：http://oss.luluai.cc/hk-a/td/2026/10/16/task_x.png
```

兼容基础地址已经以桶名结束的旧配置，不重复追加桶名。Zy 共用 `S3_PATH_PREFIX`（默认 `td/`），目录前缀不代表渠道类型。

修改配置不会自动修复之前已持久化的错误 URL；本次没有批量改写历史任务或日志。

## 保持不变

- Td、Zy 各渠道的图片与视频开关互相独立。
- 最多尝试三次，每次转存失败保留记录。
- 原 URL 保留在任务私有字段及成功转存后的消费日志中。
- 开启转存且仍可重试时，图片/视频任务查询对外显示处理中，不返回结果 URL。生成成功状态和计费不变。
- 转存成功后返回完成和 OSS 地址；三次转存均失败后，返回完成并回退上游 URL。
- 未配置存储、渠道开关关闭或源地址已过期时不等待转存，保持原有行为。
- 直接访问视频内容代理时，转存中返回 HTTP 409 / result_transferring 和 Retry-After: 5，不提前回源上游。
- 同任务执行门禁为进程内互斥，不提供跨节点互斥。
