# GPT 生图服务 - 客户交付说明

> 版本：v1.0（2026-07-31）
> 服务：GPT 图片生成 API（异步生图服务）

## 快速开始

**1. 服务地址**

```
http://p.1010token.com/gpt/api/v1
```

**2. 准备 API Key**

联系客服开通账号并获取 API Key（格式：`wk_` 开头的字符串）。API Key 已绑定所属账号，账号需有余额方可生图。

**3. 调用流程（三步）**

```text
① 创建任务  POST /generate            → 返回 task_id
② 轮询状态  GET  /client/tasks/{id}   → 直到 succeeded / failed
③ 下载图片  GET  /files/{date}/{file}  → 使用 result_url + API Key
```

**4. 运行示例代码（Python）**

```powershell
pip install requests
# 将代码中的 API_KEY 替换为你的 key 后运行
python code/client_demo.py
```

## 目录结构

```
给客户资料/
├── README.md              # 本说明
├── API接口说明.md          # 完整接口文档（鉴权/接口/错误码/计费）
└── code/
    ├── client_demo.py     # Python 完整调用示例（创建→轮询→下载→查余额）
    └── requirements.txt   # Python 依赖
```

## 重要说明

| 项目 | 说明 |
|------|------|
| 异步模式 | 任务创建后立即返回，实际生图由后台 Worker 完成，需轮询等待（通常 1~3 分钟） |
| 计费 | 生图**成功后扣费**；余额不足时创建任务直接失败（`insufficient_balance`） |
| 鉴权 | 所有业务接口需携带 `Authorization: Bearer <API_KEY>` |
| 图片下载 | `result_url` 需带 API Key 访问，仅限本 Key 创建的任务图片 |
| 限流 | 支持 IP 白名单（可选）；按次限流规则后续开放 |

## 联系方式

- 联系人：（待填写）
- 电话 / 微信：（待填写）
- 邮箱：（待填写）
