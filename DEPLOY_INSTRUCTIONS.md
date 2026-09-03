# 部署说明

## 问题描述
取消任务时，错误消息中暴露了上游任务ID（如`cgt-20260830221523-h4hgz`），用户不应该看到这些内部ID。

## 解决方案
已修改代码隐藏上游任务ID，返回用户友好的错误消息。

## 部署步骤

在远程服务器（book2: 112.126.109.72）上执行以下命令：

```bash
# 1. 进入项目目录
cd /root/new-api

# 2. 拉取最新代码
git fetch origin
git checkout dev_mints
git pull origin dev_mints

# 3. 重新构建并启动服务
docker compose -f docker-compose.dev.yml up -d --build new-api

# 4. 等待服务启动（约30-60秒）
sleep 30

# 5. 验证服务状态
curl http://localhost:3002/api/status | grep success
```

## 验证修复

部署后，执行测试脚本验证：

```bash
./test_doubao_official_api.sh --test-cancel
```

**预期结果：**
取消任务时，错误消息应该是：
```json
{
  "code": "task_not_cancellable",
  "message": "task is currently running, cannot be cancelled",
  "data": null
}
```

而不是包含上游任务ID的消息。

## 技术细节

### 修改的文件
- `relay/channel/task/doubao/adaptor.go`: 支持扁平和嵌套两种错误响应格式，并重写错误消息
- `controller/relay.go`: 添加调试日志

### 关键改动
adaptor的`CancelTask`方法现在会：
1. 解析上游返回的409错误响应（支持扁平和嵌套两种格式）
2. 识别错误码`InvalidAction.RunningTaskDeletion`
3. 返回友好消息"task is currently running, cannot be cancelled"，隐藏上游任务ID

## 调试日志

如果修复后问题仍存在，可以查看日志：

```bash
# 查看Docker容器日志
docker logs new-api-dev 2>&1 | grep -E "RelayTaskDelete|Cancel"

# 或查看文件日志
tail -100 /data/logs/oneapi-*.log | grep -E "RelayTaskDelete|Cancel"
```

应该能看到类似这样的日志：
```
[RelayTaskDelete] 函数被调用，Method=DELETE, Path=/api/v3/contents/generations/tasks/task_xxx
[Cancel] 开始解析上游错误响应
[Cancel] 扁平格式解析成功，错误码: InvalidAction.RunningTaskDeletion
[Cancel] 匹配到RunningTaskDeletion，返回重写消息
```
