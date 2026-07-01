# 生产环境 Docker 部署指南

## 概述

本项目已经包含完整的 Docker 部署配置，支持快速启动和生产环境部署。

## 现有配置文件

- `Dockerfile` - 生产环境镜像构建文件（多阶段构建）
- `docker-compose.yml` - 服务编排配置（应用 + MySQL/PostgreSQL + Redis）
- `start.sh` - 一键启动脚本（适合开发/测试）
- `build-docker.sh` - 增强构建脚本（适合生产部署）

## 部署方案选择

### 方案 1: 使用外部数据库（推荐生产环境）

**适用场景**: 生产环境已有独立的 MySQL 和 Redis 服务

**步骤**:

1. **修改 docker-compose.yml**

```yaml
services:
  new-api:
    image: new-api:latest
    environment:
      # 修改为你的生产数据库连接
      - SQL_DSN=root:YOUR_PASSWORD@tcp(your-mysql-host:3306)/new-api
      
      # 修改为你的生产 Redis 连接
      - REDIS_CONN_STRING=redis://:YOUR_PASSWORD@your-redis-host:6379
      
      # 多节点部署必须设置（修改为随机字符串）
      - SESSION_SECRET=your-random-secret-key-change-me
      
      # 节点名称（多节点部署时区分节点）
      - NODE_NAME=new-api-production-node-1
      
      - TZ=Asia/Shanghai
      - ERROR_LOG_ENABLED=true
      - BATCH_UPDATE_ENABLED=true
```

2. **构建并启动**

```bash
# 方式 A: 使用 start.sh（最简单）
./start.sh

# 方式 B: 手动构建
docker build -t new-api:latest .
docker-compose up -d

# 查看日志
docker-compose logs -f new-api
```

### 方案 2: 使用 Docker Compose 完整服务栈

**适用场景**: 需要一套完整的服务（应用 + 数据库 + Redis）

**步骤**:

1. **修改 docker-compose.yml**

取消注释以下部分：

```yaml
services:
  new-api:
    depends_on:
      - redis
      - mysql  # 或 postgres

  redis:
    image: redis:latest
    container_name: redis
    restart: always
    command: ["redis-server", "--requirepass", "YOUR_REDIS_PASSWORD"]  # 修改密码
    networks:
      - new-api-network

  mysql:
    image: mysql:8.2
    container_name: mysql
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: YOUR_MYSQL_PASSWORD  # 修改密码
      MYSQL_DATABASE: new-api
    volumes:
      - mysql_data:/var/lib/mysql
    networks:
      - new-api-network

volumes:
  mysql_data:  # 取消注释
```

2. **修改应用环境变量**

```yaml
services:
  new-api:
    environment:
      - SQL_DSN=root:YOUR_MYSQL_PASSWORD@tcp(mysql:3306)/new-api
      - REDIS_CONN_STRING=redis://:YOUR_REDIS_PASSWORD@redis:6379
      - SESSION_SECRET=your-random-secret-key-change-me
```

3. **启动服务**

```bash
./start.sh
```

### 方案 3: 推送到镜像仓库（CI/CD 部署）

**适用场景**: 需要推送到私有镜像仓库或实现自动化部署

**步骤**:

```bash
# 构建并推送到镜像仓库
./build-docker.sh \
  --push \
  --tag-latest \
  --registry your-registry.example.com

# 在生产服务器上拉取并运行
docker pull your-registry.example.com/new-api:latest
docker-compose up -d
```

## 构建脚本对比

| 特性 | start.sh | build-docker.sh |
|------|----------|-----------------|
| **用途** | 本地开发/快速测试 | 生产环境部署 |
| **版本管理** | 固定 latest | 基于 git 自动生成版本号 |
| **多架构支持** | ❌ | ✅ AMD64 + ARM64 |
| **推送到仓库** | ❌ | ✅ |
| **构建参数** | 固定 | 灵活配置 |
| **适用场景** | 开发测试 | 生产部署、CI/CD |

### build-docker.sh 使用示例

```bash
# 查看帮助
./build-docker.sh --help

# 本地构建测试
./build-docker.sh

# 生产环境单架构（AMD64）
./build-docker.sh \
  --platform linux/amd64 \
  --push \
  --tag-latest \
  --registry registry.example.com

# 生产环境多架构（AMD64 + ARM64）
./build-docker.sh \
  --multi-arch \
  --push \
  --tag-latest \
  --registry registry.example.com

# 阿里云容器镜像服务
./build-docker.sh \
  --push \
  --registry registry.cn-hangzhou.aliyuncs.com/your-namespace
```

## 生产环境必改配置清单

### 1. 数据库连接 (SQL_DSN)

```bash
# MySQL
SQL_DSN=root:YOUR_PASSWORD@tcp(mysql-host:3306)/new-api

# PostgreSQL
SQL_DSN=postgresql://user:YOUR_PASSWORD@postgres-host:5432/new-api?sslmode=disable
```

### 2. Redis 连接 (REDIS_CONN_STRING)

```bash
# 带密码
REDIS_CONN_STRING=redis://:YOUR_PASSWORD@redis-host:6379

# 不带密码
REDIS_CONN_STRING=redis://redis-host:6379
```

### 3. 会话密钥 (SESSION_SECRET)

```bash
# 多节点部署必须设置，使用随机字符串
SESSION_SECRET=your-random-secret-key-change-me

# 生成随机密钥示例
openssl rand -base64 32
```

### 4. 节点名称 (NODE_NAME)

```bash
# 用于审计日志中标识节点身份
NODE_NAME=new-api-production-node-1
```

### 5. 其他可选配置

```bash
# 时区
TZ=Asia/Shanghai

# 错误日志
ERROR_LOG_ENABLED=true

# 批量更新
BATCH_UPDATE_ENABLED=true

# 流模式超时（秒）
STREAMING_TIMEOUT=300

# HTTP 空闲连接超时（秒）
RELAY_IDLE_CONN_TIMEOUT=90

# 数据库同步频率（秒）
SYNC_FREQUENCY=60

# Google Analytics
GOOGLE_ANALYTICS_ID=G-XXXXXXXXXX

# Umami 统计
UMAMI_WEBSITE_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
UMAMI_SCRIPT_URL=https://analytics.umami.is/script.js
```

## 数据持久化

### 目录挂载

```yaml
volumes:
  - ./data:/data        # 数据目录（SQLite 数据库文件、缓存等）
  - ./logs:/app/logs    # 日志目录
```

### 目录说明

```
./data/              # 数据目录
  ├── data.db        # SQLite 数据库文件（如果使用 SQLite）
  ├── cache/         # 缓存文件
  └── uploads/       # 上传文件

./logs/              # 日志目录
  ├── access.log     # 访问日志
  └── error.log      # 错误日志
```

## 常用操作

### 启动服务

```bash
# 使用 start.sh
./start.sh

# 或手动启动
docker-compose up -d
```

### 查看日志

```bash
# 实时查看所有服务日志
docker-compose logs -f

# 只看应用日志
docker-compose logs -f new-api

# 查看最近 100 行
docker-compose logs --tail 100 new-api
```

### 重启服务

```bash
# 重启所有服务
docker-compose restart

# 只重启应用
docker-compose restart new-api
```

### 停止服务

```bash
# 停止所有服务
docker-compose down

# 停止并删除数据卷（危险操作）
docker-compose down -v
```

### 更新部署

```bash
# 1. 重新构建镜像
docker build -t new-api:latest .

# 2. 重启服务
docker-compose up -d

# 或者使用 start.sh 一键更新
./start.sh
```

### 进入容器调试

```bash
# 进入应用容器
docker exec -it new-api sh

# 查看环境变量
docker exec new-api env

# 检查健康状态
docker exec new-api wget -qO- http://localhost:3000/api/status
```

### 查看容器资源使用

```bash
# 实时查看资源使用
docker stats new-api

# 查看容器详情
docker inspect new-api
```

## 健康检查

容器已配置健康检查：

```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -q -O - http://localhost:3000/api/status | grep -o '\"success\":\\s*true' || exit 1"]
  interval: 30s
  timeout: 10s
  retries: 3
```

查看健康状态：

```bash
docker ps
# 查看 STATUS 列的 (healthy) 标识
```

## 数据库迁移

### 首次启动

首次启动时，系统会自动创建所有表，包括：

- 用户相关表
- Token 相关表
- 渠道相关表
- 日志相关表
- **视频项目表 (video_projects)** - 新增的视频生成 API 表

### 验证表是否创建

```bash
# MySQL
docker exec -it mysql mysql -u root -p new-api -e "SHOW TABLES LIKE 'video_projects';"

# PostgreSQL
docker exec -it postgres psql -U root -d new-api -c "\dt video_projects"

# SQLite（进入容器）
docker exec -it new-api sh
sqlite3 /data/data.db ".tables"
```

## 生产环境最佳实践

### 1. 使用反向代理（Nginx）

```nginx
upstream new_api {
    server 127.0.0.1:3000;
    keepalive 64;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    client_max_body_size 50M;

    location / {
        proxy_pass http://new_api;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
}
```

### 2. 设置资源限制

```yaml
services:
  new-api:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 1G
```

### 3. 配置日志轮转

```yaml
services:
  new-api:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"
```

### 4. 定期备份

```bash
#!/bin/bash
# backup.sh - 数据库备份脚本

BACKUP_DIR="/backup/new-api"
DATE=$(date +%Y%m%d_%H%M%S)

# MySQL 备份
docker exec mysql mysqldump -u root -p${MYSQL_ROOT_PASSWORD} new-api > \
  ${BACKUP_DIR}/new_api_${DATE}.sql

# 压缩
gzip ${BACKUP_DIR}/new_api_${DATE}.sql

# 保留最近 7 天
find ${BACKUP_DIR} -name "*.sql.gz" -mtime +7 -delete
```

定时任务：

```bash
# crontab -e
0 2 * * * /opt/scripts/backup.sh
```

## CI/CD 集成

### GitHub Actions 示例

```yaml
# .github/workflows/deploy.yml
name: Deploy to Production

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Login to Registry
        uses: docker/login-action@v2
        with:
          registry: ${{ secrets.REGISTRY }}
          username: ${{ secrets.REGISTRY_USERNAME }}
          password: ${{ secrets.REGISTRY_PASSWORD }}
      
      - name: Build and Push
        run: |
          chmod +x build-docker.sh
          ./build-docker.sh --push --tag-latest --registry ${{ secrets.REGISTRY }}
      
      - name: Deploy to Production
        uses: appleboy/ssh-action@master
        with:
          host: ${{ secrets.PRODUCTION_HOST }}
          username: ${{ secrets.PRODUCTION_USER }}
          key: ${{ secrets.SSH_PRIVATE_KEY }}
          script: |
            cd /opt/new-api
            docker-compose pull
            docker-compose up -d
```

### GitLab CI 示例

```yaml
# .gitlab-ci.yml
stages:
  - build
  - deploy

build:
  stage: build
  script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - ./build-docker.sh --push --tag-latest --registry $CI_REGISTRY
  only:
    - main
    - tags

deploy:
  stage: deploy
  script:
    - ssh user@production-server "cd /opt/new-api && docker-compose pull && docker-compose up -d"
  only:
    - main
  when: manual
```

## 故障排查

### 问题 1: 容器启动失败

```bash
# 查看容器日志
docker logs new-api

# 检查端口占用
netstat -tuln | grep 3000

# 检查配置文件语法
docker-compose config
```

### 问题 2: 数据库连接失败

```bash
# 检查数据库环境变量
docker exec new-api env | grep SQL_DSN

# 测试数据库连接（MySQL）
docker exec mysql mysql -u root -p -e "SELECT 1"

# 测试网络连通性
docker exec new-api ping mysql-host
```

### 问题 3: Redis 连接失败

```bash
# 检查 Redis 环境变量
docker exec new-api env | grep REDIS

# 测试 Redis 连接
docker exec redis redis-cli ping
```

### 问题 4: 视频项目表未创建

```bash
# 重启容器触发迁移
docker-compose restart new-api

# 查看启动日志
docker-compose logs -f new-api | grep video_projects

# 手动进入数据库检查
docker exec -it mysql mysql -u root -p new-api
SHOW TABLES LIKE 'video_projects';
DESC video_projects;
```

## 安全建议

1. ✅ **修改所有默认密码** - 数据库、Redis、SESSION_SECRET
2. ✅ **使用环境变量文件** - 敏感信息不要提交到 Git
3. ✅ **启用 HTTPS** - 生产环境必须使用 HTTPS
4. ✅ **限制网络访问** - 只暴露必要的端口
5. ✅ **定期更新镜像** - 获取安全补丁
6. ✅ **启用防火墙** - 限制容器之间的网络访问
7. ✅ **备份策略** - 定期备份数据库和重要文件
8. ✅ **监控告警** - 配置容器和应用监控

## 相关文档

- [视频生成 API 文档](./video-generation-api.md)
- [详细 Docker 部署文档](./docker-deployment.md)
- [环境变量完整说明](../README.md#environment-variables)

## 快速参考

```bash
# 构建并启动（开发/测试）
./start.sh

# 构建并推送到仓库（生产）
./build-docker.sh --push --tag-latest --registry your-registry.example.com

# 查看日志
docker-compose logs -f new-api

# 重启服务
docker-compose restart new-api

# 进入容器
docker exec -it new-api sh

# 查看健康状态
docker ps
docker inspect new-api --format='{{.State.Health.Status}}'

# 备份数据库
docker exec mysql mysqldump -u root -p new-api > backup.sql
```
