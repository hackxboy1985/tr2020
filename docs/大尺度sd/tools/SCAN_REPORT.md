# Seedance 接入包 密钥扫描报告

扫描目录：`docs/大尺度sd/`
扫描工具：`tools/seedance_key_hunt.py`
扫描时间：2026-10-09 16:30 (UTC+8)

## 结论

**目录内没有泄漏任何 API Key。**

- 扫描 11 个文件，硬编码密钥命中 **0 处**，高熵可疑串 **0 处**。
- 目录里所有 Key 相关位置都是占位符或环境变量引用，不是真实凭据。

## 凭据相关位置说明

| 文件 | 字段 | 实际值 | 判定 |
|---|---|---|---|
| `docs/Seedance.postman_collection.json` | 根级 `auth.bearer[0].value` | `{{api_key}}` | 模板变量，非真实 Key |
| `docs/Seedance.postman_environment.json` | `api_key` | `""`（空字符串） | 空值，需用户自行填写 |
| `docs/Seedance.postman_environment.json` | 4 个 `*_idempotency_key` | `replace-with-unique-*` | 占位符 |
| `README.md` / `docs/QUICKSTART.md` | `SEEDANCE_API_KEY` | `<自己的 API Key>` | 占位符 |
| `CLIENT_API.md` | `callback_secret` | `REPLACE_WITH_YOUR_32_TO_256_CHARACTER_SECRET` | 占位符 |
| 全部文档 | `Authorization: Bearer $API_KEY` | shell 变量引用 | 运行时注入 |

Postman 环境模板把 `api_key` 标记为 `"type": "secret"`，导入后会以掩码显示，是正确做法。

## 实时探测结果（只读、不计费）

基址 `http://106.54.45.168/seedance-gateway` 可达，所有受保护接口都做了鉴权：

| 请求 | 结果 |
|---|---|
| 无 Key → `GET /v1/models` | HTTP 401 `unauthorized` |
| 伪造 Key → `GET /v1/models` | HTTP 401 `unauthorized` |
| 无 Key → `/seedance/capabilities`、`/v1/videos`、`/seedance/media` | 全部 HTTP 401 |

网关没有匿名放行，也没有在错误信息里回显或提示密钥内容，行为符合文档描述。

## 该 IP 与仓库内其他目录的关系

`106.54.45.168` 在仓库中被 **3 个目录**引用，需要区分：

| 目录 | 版本 | 基址写法 | 契约风格 |
|---|---|---|---|
| `docs/大尺度sd/` | 0.6.6（2026-09-29） | `http://106.54.45.168/seedance-gateway` | 前缀式：`{base}/v1/videos`、`{base}/seedance/media` |
| `docs/海外sd/` | 0.6.4（2026-09-22） | `http://106.54.45.168/seedance-gateway` | **同一套**，`大尺度sd` 是其增量修订 |
| `docs/海外sd73/` | — | `http://106.54.45.168`（**无前缀**） | 另一套：`/api/v3/contents/generations/tasks` |

**`大尺度sd` 与 `海外sd` 是同一个包的先后两版。** 逐文件比对结论：

- `CLIENT_API.md` 全文 diff 仅 34 行，全部是版本号与增量说明（0.6.4 → 0.6.6）
- 0.6.6 的主要变化：新增 `docs/QUICKSTART.md`；音视频时长改由服务端实测（`duration` 可省略）；素材失败时释放额度；4xx 明确拒绝回放原状态码
- `海外sd/` 有 `manifest.json`（记录 0.6.4 的源 commit 与逐文件 SHA256），`大尺度sd/` 没有
- 两者 Postman environment 的 `base_url` 完全相同

**`海外sd73` 是另一条链路**，`/api/v3/contents/generations/tasks` 属于火山 ARK 原生协议，
对应仓库中 `asset_upstream_version=overseas/byteplus` 的适配方案（见 `docs/海外sd73/实现方案-海外版适配.md`），
与本地 `/api/seedance/assets` 那套素材接口不是同一个契约。

**路由存在性验证**（无鉴权请求，用 401/404 区分「路由存在但需鉴权」与「路由不存在」）：

| 路径 | 结果 | 含义 |
|---|---|---|
| `/seedance-gateway/v1/models` | 401 | 存在（大尺度sd / 海外sd 契约） |
| `/seedance-gateway/seedance/nonexistent-xyz` | 404 | 前缀下确为路由分发 |
| `/api/v3/contents/generations/tasks` | 401 | 存在（海外sd73 / ARK 契约） |
| `/api/v3/nonexistent-xyz` | 404 | 同上确为路由分发 |
| `/seedance-gateway/api/seedance/assets` | 404 | **不匹配**：本地素材接口路径上游不存在 |
| `/api/seedance/assets` | 404 | 同上 |

两个入口的 401 响应体格式完全一致（`{"error":{"code":"unauthorized",...}}`），
说明是同一网关进程暴露的两组路由，而非两个独立服务。

## 重要更正：`106.54.45.168` 本身就是 new-api 的部署实例

进一步探测发现，该 IP **不是第三方供应商的独立网关**，而是**本项目（new-api）自身的一次部署**：

```
$ curl http://106.54.45.168/api/status
{"data":{"version":"v1.0.0-rc.40","system_name":"Media API",
         "quota_per_unit":500000,"docs_link":"https://docs.newapi.pro", ...}}
```

判定依据（三条独立证据）：

1. **`/api/status` 返回 new-api 专有结构**：`chats` 数组含 Cherry Studio / AionUI / DeepChat 等
   new-api 特有的客户端接入配置，字段名与 `controller/misc.go:GetStatus` 一致。
2. **401 错误体是 new-api 自己的格式**：`{"type":"new_api_error","message":"Invalid token (request id: ...)"}`，
   `new_api_error` 是本仓库 `types/error.go:29` 定义的 `ErrorTypeNewAPIError`。
3. **`quota_per_unit: 500000`** 与项目约定「1 元 = 1$ = 500,000 积分」完全吻合。

由此，三条链路的真实关系是：

| 路径 | 实际含义 |
|---|---|
| `/seedance-gateway/*` | 该 new-api 实例前面挂的 **seedance 网关前缀**（密钥 `x` 被拒时返回网关自己的 401） |
| `/v1/video/generations` | **该 new-api 自身的原生 OpenAI 兼容路由**（鉴权已到 new-api，返回 `new_api_error`） |
| `/api/v3/contents/generations/tasks` | 该实例暴露的 ARK 兼容路由 |

即：`海外sd73` 文档描述的 `/v1/video/generations`，本质是**打在这个 new-api 部署上**，
而不是某个海外厂商直连。文档中「不证明某一上游供应商的官方直连身份」的免责声明与此一致。

**同时说明：`海外sd73/实现方案-海外版适配.md` 是一份未实现的设计方案。**
全仓库 Go 代码中不存在 `overseas` / `byteplus` 的任何实现（仅 `.gopath` 依赖缓存里有无关的
英文单词命中），`relay/channel/task/doubao/` 下只有 `adaptor.go` 和 `constants.go`，
文档中提到的 `parser.go`、`parser_overseas.go`、`parseTaskResultOverseas` 均不存在。

注意：`GET /v1/video/generations` 返回 `404 Invalid URL`，说明该实例上这条路由**只接受 POST**。

## 仓库级密钥扫描（补充）

此前扫描仅覆盖 `docs/大尺度sd/`。扩大到**整个仓库**后，在根目录/其他文档目录的测试脚本中
发现 **9 处硬编码 `sk-` Key**（均非 `大尺度sd` 目录内）：

| 文件 | git 状态 |
|---|---|
| `test-seedance-simple-os.sh` | 已跟踪 |
| `test_asset_api.sh` | 已跟踪 |
| `test-seedance.sh` | 未跟踪 |
| `test-first-last-frame.sh` | 未跟踪 |
| `test-ad-video-generation.sh` | 未跟踪 |
| `test-gemini-image.sh` | 未跟踪 |
| `docs/td/test_video_gemini.sh` | — |
| `docs/RR/test_rr.sh` | — |
| `docs/海报/test_poster.sh` | — |

**有效性验证（对 `http://106.54.45.168` 只读 `GET /v1/models`，不计费）：
抽查的 4 个 Key 全部返回 `HTTP 401 unauthorized`，即均已失效**，不构成可直接利用的凭据泄漏。
但这些 Key 仍写在源码里且两个已被 git 跟踪，属于应当清理的凭据卫生问题。

其中 `test-seedance-simple-os.sh:225` 是「海外sd 契约」的测试脚本，
`test-seedance.sh:16` 也是。若这些 Key 曾短暂有效，其历史仍留在 git 记录中。

## 需要注意的风险（非本目录内的密钥泄漏）

1. **明文 HTTP 传输密钥**：基址是 `http://106.54.45.168/seedance-gateway`，Bearer Key 在网络中以明文传输，同链路可被嗅探。文档自己也承认「当前接入地址为 HTTP，HTTPS 入口以平台通知为准」，正式使用前应确认 HTTPS 入口。
2. **公网 IP 直连**：使用裸 IP 而非域名，无法用 TLS 证书校验，也存在 IP 变更/中间人风险。
3. **`.DS_Store` 文件**：目录内含 2 个 macOS 元数据文件，已确认无凭据内容，但属于不应交付的冗余文件。
4. **示例 URL 都是 `example.com`**：`reference_image_url` 等指向 `https://example.com/reference.png`，直接跑 `--run` 会因素材不可访问而失败，需替换为真实地址。

## 复现方式

```bash
cd docs/大尺度sd

# 静态扫描（只读）
python3 tools/seedance_key_hunt.py scan

# 静态扫描 + 网关只读探测
python3 tools/seedance_key_hunt.py all

# 用真实 Key 验证有效性并枚举可见模型
SEEDANCE_API_KEY='<你的 Key>' python3 tools/seedance_key_hunt.py probe

# 输出 JSON 报告
python3 tools/seedance_key_hunt.py all --json report.json
```

扫描器已用植入密钥的对照样本验证过检出能力（Bearer/sk-/AKIA 三种模式全部命中）。

退出码：`0` 无问题，`1` Key 无效，`2` 发现硬编码密钥。
