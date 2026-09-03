# 取消任务错误消息修复总结

## 问题描述

当尝试取消正在运行的Doubao视频任务时，API返回的错误消息包含上游任务ID：

```json
{
  "code": "task_not_cancellable",
  "message": "Cannot delete task `cgt-20260830221523-h4hgz` because it is currently running. Request id: ...",
  "data": null
}
```

**问题：** 上游任务ID（`cgt-20260830221523-h4hgz`）暴露了内部实现细节，不应该展示给最终用户。

## 根本原因

1. **上游API返回的错误格式**：咪咕视频API返回的409错误可能是扁平格式而非嵌套格式
2. **原代码只支持嵌套格式**：只尝试解析 `{"error": {"code": "...", "message": "..."}}`
3. **解析失败后透传原始消息**：当JSON解析失败时，直接将上游的完整错误消息返回给用户

## 修复方案

### 修改文件
`relay/channel/task/doubao/adaptor.go` - `CancelTask` 方法

### 关键改动

1. **支持两种错误响应格式**
   - 扁平格式：`{"code": "...", "message": "...", "type": "..."}`
   - 嵌套格式：`{"error": {"code": "...", "message": "...", "type": "..."}}`

2. **错误消息重写逻辑**
   ```go
   // 根据错误码返回用户友好的消息
   switch errorCode {
   case "InvalidAction.RunningTaskDeletion":
       return errors.New("task is currently running, cannot be cancelled")
   default:
       return errors.New("task cannot be cancelled at this time")
   }
   ```

3. **添加调试日志**
   - 记录JSON解析过程
   - 记录错误码匹配情况
   - 便于后续排查问题

### 预期结果

修复后，用户看到的错误消息将是：

```json
{
  "code": "task_not_cancellable",
  "message": "task is currently running, cannot be cancelled",
  "data": null
}
```

**不再包含上游任务ID。**

## 部署状态

✅ **代码已提交到 `dev_mints` 分支**

⚠️ **需要在远程服务器部署** - 测试环境运行在远程服务器 book2 (112.126.109.72)

### 部署命令

SSH到远程服务器后执行：

```bash
cd /root/new-api
git checkout dev_mints
git pull origin dev_mints
docker compose -f docker-compose.dev.yml up -d --build new-api
```

### 验证命令

```bash
./test_doubao_official_api.sh --test-cancel
```

检查返回的错误消息中是否还包含上游任务ID。

## 技术细节

### 为什么本地调试没有生效？

调试过程中发现：
- 测试URL `http://book2:3002` 指向远程服务器
- 本地的代码修改和Docker构建都在本地机器
- 远程服务器运行的是旧版本代码

这就是为什么：
- 添加的日志没有输出
- 错误消息没有改变
- 多次重新构建Docker都没有效果

### 学到的教训

1. 在开始调试前确认测试环境的位置（本地/远程）
2. 检查配置文件中的URL和主机名
3. 确保代码部署到正确的环境

## 相关提交

1. `fix: 支持扁平和嵌套两种错误响应格式，隐藏上游任务ID`
2. `debug: 添加详细的调试日志`  
3. `docs: 添加部署说明文档`

## 后续工作

1. 在远程服务器部署代码
2. 运行测试验证修复效果
3. 如果问题仍存在，查看调试日志定位原因
4. 清理调试日志代码（可选）

---

**文档创建时间：** 2026-08-30 22:18

**分支：** dev_mints

**状态：** 待部署到远程服务器
