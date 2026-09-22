# 素材预检查、标签与视频字段补充（0.6.4）

此文描述 0.6.4 接口契约。原 NewAPI 视频协议、模型名称、价格及用户所有权规则保持不变。上线状态以发布验收记录为准。

## 素材标签

`POST /seedance/media` 的 import、update 接受 `tags`。最多 20 项，每项 1–64 字符，去首尾空格、去重。空数组清除标签；省略保留原值。标签仅存于本平台，不转发给生成服务。

```json
{"action":"import","url":"https://example.com/reference.jpg","media_type":"image","title":"产品图","group_id":"group_example","tags":["新品","秋季"]}
```

```json
{"action":"update","id":"task_example","tags":["新品"]}
```

导入、上传、单条、批量、列表均返回 `tags` 数组。列表支持 `GET /seedance/media?tag=新品` 或 POST `{"action":"list","tag":"新品"}`，匹配完整标签，可与组、类型、状态筛选组合。上传后可用 update 添加标签。历史无标签记录为 `[]`。

## 素材与目标模型预检查

使用正常 Bearer Key 调用 `POST /seedance/media/preflight`，不会创建任务、扣费或迁移素材。

```json
{"asset_id":"task_example","model":"seedance-2.0-720p","media_type":"image","role":"reference_image"}
```

`media_type`、`role` 可省略。外部用户、隐藏/删除素材返回 404；Key 无目标模型或素材权限返回 403。

返回 `asset_id/model/compatibility/checks/production_verified`。各检查项使用 `supported/unsupported/unverified` 三态；明确不满足任何已知条件时整体为 unsupported。所有本地条件通过仍不等于模型实际接受，真实参考能力未验证时整体为 unverified。不会公开内部通道、模型或凭据。

检查项包括权限、素材就绪、类型、参考角色、当前通道的目标模型启用情况及真实模型能力。单素材检查不证明多素材组合、视频/音频时长或全部角色组合可用。检查与实际提交之间配置可能变化；正式提交会重新校验权限、就绪、已知类型和通道条件，多通道引用被拒绝。真实生成仍以宿主及底层能力校验为准。

能力目录的 `models[].reference_support` 标为 unverified，提供 `preflight` 路径；`media.tags/preflight` 标识接口已实现，不是模型能力验收证明。

## 视频字段与回调

已接受的时长、尺寸、比例和创建协议写入持久存储。明确尺寸可恢复比例；明确分辨率及比例可计算请求尺寸。历史任务从已存参数恢复，未知值保持 null。这些字段不是视频文件实测结果。

0.6.4 在全部输出自动保存完成后返回七天保存期限：`expires_at = completed_at + 604800`，`storage.status=ready`。正在保存或保存暂不可用时，顶层 expires_at 仍为已知内容链接到期时间或 null；详见 [保存与下载周期](RETENTION.md)。

本修复之后新建 `/v1/videos` 的回调示意：

```json
{"id":"evt_task_example","type":"video.completed","version":"video.v1","data":{"id":"task_example","object":"video","status":"completed"}}
```

示意省略了其他 Video 字段；实际 data 使用与 GET /v1/videos/{id} 相同投影。旧路径及历史任务保留 `video.succeeded/video.failed` 和旧 data。已持久化事件重试保持相同正文，原签名及去重机制不变。

## 稳定错误分类

HTTP 错误为 `error.code/message/type/param`，固定公开提示，不透传底层错误、地址或标识。

| code | 含义与处理 |
|---|---|
| asset_not_ready | 等待素材处理成功 |
| media_type_mismatch | 更正引用类型 |
| model_incompatible | 当前素材通道不满足目标模型条件 |
| asset_channel_conflict | 当前素材不能混在同一次请求使用 |
| invalid_tags | 修正标签数量及长度 |
| content_expired | 保存期已结束，或存在可靠内容到期证据；使用已下载文件或联系平台，不自动重提付费生成 |
| content_unavailable | 文件不可用；不推断一定过期 |
| invalid_range | 修正字节范围，保留 HTTP 416 |
| download_service_unavailable | 下载服务异常；无法凭该错误确认文件已过期 |
| service_unavailable / rate_limited | 稍后重试；生成提交结果不明时勿盲目重复提交 |

错误分类不会恢复已失效文件。超过七天的长期保存、跨通道自动注册、客户 Action/PascalCase 格式兼容、真实全模型验证不在本批已实现能力中。

## 0.6.3 下载补丁

真实视频参考验收发现：部分签名媒体允许 GET，却拒绝 HEAD。内容接口现以已授权 GET 获取真实响应头，对 HEAD 客户端立即停止接收文件体，保留实际 Content-Length/Range 等安全字段。无需重新生成或额外扣费，账号隔离与鉴权规则不变。
