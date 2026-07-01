# Docker 生产环境部署指南

## 快速开始

### 1. 构建 Docker 镜像

```bash
# 基本构建（本地测试）
./build-docker.sh

# 构建并推送到镜像仓库
./build-docker.sh --push --tag-latest --registry your-registry.example.com

# 多架构构建（AMD64 + ARM64）
./build-docker.sh --multi-arch --push --registry your-registry.example.com
```

### 2. 运行容器

```bash
# 使用 SQLite（开发/测试）
docker run -d \
  --name new-api \
  -p 3000:3000 \
  -v /data/new-api:/data \
  your-registry.example.com/new-api:latest

# 使用 MySQL
docker run -d \
  --name new-api \
  -p 3000:3000 \
  -e SQL_DSN="user:password@tcp(mysql-host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local" \
  -v /data/new-api:/data \
  your-registry.example.com/new-api:latest

# 使用 PostgreSQL
docker run -d \
  --name new-api \
  -p 3000:3000 \
  -e SQL_DSN="postgresql://user:password@postgres-host:5432/dbname?sslmode=disable" \
  -v /data/new-api:/data \
  your-registry.example.com/new-api:latest
```

## 构建选项详解

### build-docker.sh 参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `--push` | 构建后推送到镜像仓库 | `./build-docker.sh --push` |
| `--platform` | 指定目标平台 | `./build-docker.sh --platform linux/arm64` |
| `--multi-arch` | 构建多架构镜像（amd64+arm64） | `./build-docker.sh --multi-arch` |
| `--tag-latest` | 同时打上 latest 标签 | `./build-docker.sh --tag-latest` |
| `--registry` | 指定镜像仓库地址 | `./build-docker.sh --registry hub.docker.com/myorg` |
| `--help` | 显示帮助信息 | `./build-docker.sh --help` |

### 使用示例

```bash
# 场景 1: 本地开发测试
./build-docker.sh

# 场景 2: 生产环境单架构部署（AMD64）
./build-docker.sh \
  --platform linux/amd64 \
  --push \
  --tag-latest \
  --registry registry.example.com

# 场景 3: 生产环境多架构部署
./build-docker.sh \
  --multi-arch \
  --push \
  --tag-latest \
  --registry registry.example.com

# 场景 4: 只构建特定版本不推送
./build-docker.sh --platform linux/amd64

# 场景 5: 阿里云容器镜像服务
./build-docker.sh \
  --push \
  --registry registry.cn-hangzhou.aliyuncs.com/your-namespace
```

## 使用 Docker Compose 部署

### docker-compose.yml（完整示例）

```yaml
version: '3.8'

services:
  new-api:
    image: your-registry.example.com/new-api:latest
    container_name: new-api
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      # 数据库配置（MySQL）
      - SQL_DSN=root:password@tcp(mysql:3306)/new_api?charset=utf8mb4&parseTime=True&loc=Local
      
      # Redis 配置
      - REDIS_CONN_STRING=redis://redis:6379
      
      # 会话密钥（必须修改）
      - SESSION_SECRET=your-random-secret-key-change-me
      
      # 日志级别
      - LOG_LEVEL=info
      
      # 其他配置
      - PORT=3000
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/data
      - ./logs:/logs
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_started
    networks:
      - new-api-network
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/api/status"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  mysql:
    image: mysql:8.0
    container_name: new-api-mysql
    restart: unless-stopped
    environment:
      - MYSQL_ROOT_PASSWORD=your-mysql-root-password
      - MYSQL_DATABASE=new_api
      - MYSQL_USER=new_api_user
      - MYSQL_PASSWORD=your-mysql-password
      - TZ=Asia/Shanghai
    volumes:
      - mysql-data:/var/lib/mysql
      - ./mysql/my.cnf:/etc/mysql/conf.d/my.cnf:ro
    ports:
      - "3306:3306"
    networks:
      - new-api-network
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-p$$MYSQL_ROOT_PASSWORD"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: new-api-redis
    restart: unless-stopped
    command: redis-server --requirepass your-redis-password --appendonly yes
    volumes:
      - redis-data:/data
    ports:
      - "6379:6379"
    networks:
      - new-api-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3

networks:
  new-api-network:
    driver: bridge

volumes:
  mysql-data:
  redis-data:
```

### 启动服务

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f new-api

# 查看所有服务状态
docker-compose ps

# 停止服务
docker-compose down

# 停止并删除数据卷（危险操作）
docker-compose down -v
```

## 环境变量配置

### 必需环境变量

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `SQL_DSN` | 数据库连接字符串 | `user:pass@tcp(host:3306)/db?charset=utf8mb4` |
| `SESSION_SECRET` | 会话加密密钥 | `your-random-secret-key` |

### 可选环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PORT` | 服务监听端口 | `3000` |
| `REDIS_CONN_STRING` | Redis 连接字符串 | 空（不使用 Redis） |
| `LOG_LEVEL` | 日志级别 | `info` |
| `LOG_SQL_DSN` | 日志数据库（独立） | 空（使用主数据库） |
| `CHANNEL_UPDATE_FREQUENCY` | 渠道更新频率（秒） | `3600` |
| `ENABLE_QUOTA_CACHE` | 启用配额缓存 | `true` |
| `SAVE_PROMPT` | 保存提示词 | `false` |

## 数据持久化

### 目录说明

```
/data/              # 主数据目录
  ├── data.db       # SQLite 数据库文件（如果使用 SQLite）
  ├── cache/        # 缓存文件
  └── uploads/      # 上传文件

/logs/              # 日志目录
  ├── access.log    # 访问日志
  └── error.log     # 错误日志
```

### Volume 挂载建议

```bash
# 生产环境推荐挂载
docker run -d \
  -v /data/new-api/data:/data \
  -v /data/new-api/logs:/logs \
  your-registry.example.com/new-api:latest
```

## 生产环境最佳实践

### 1. 资源限制

```yaml
services:
  new-api:
    # ...
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 1G
```

### 2. 健康检查

```yaml
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/api/status"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### 3. 日志轮转

```yaml
services:
  new-api:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"
```

### 4. 使用反向代理（Nginx）

```nginx
upstream new_api {
    server 127.0.0.1:3000;
    keepalive 64;
}

server {
    listen 80;
    server_name api.example.com;

    # HTTPS 重定向
    return 301 https://$server_name$request_uri;
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
        
        # 长连接支持
        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
}
```

### 5. 备份策略

```bash
#!/bin/bash
# 数据库备份脚本

BACKUP_DIR="/backup/new-api"
DATE=$(date +%Y%m%d_%H%M%S)

# MySQL 备份
docker exec new-api-mysql mysqldump -u root -p${MYSQL_ROOT_PASSWORD} new_api > \
  ${BACKUP_DIR}/new_api_${DATE}.sql

# 压缩
gzip ${BACKUP_DIR}/new_api_${DATE}.sql

# 保留最近 7 天的备份
find ${BACKUP_DIR} -name "*.sql.gz" -mtime +7 -delete
```

## 监控与调试

### 查看容器日志

```bash
# 实时查看日志
docker logs -f new-api

# 查看最近 100 行
docker logs --tail 100 new-api

# 查看错误日志
docker exec new-api cat /logs/error.log
```

### 进入容器调试

```bash
# 进入容器
docker exec -it new-api sh

# 检查数据库连接
docker exec new-api wget -qO- http://localhost:3000/api/status
```

### 性能监控

```bash
# 容器资源使用
docker stats new-api

# 查看进程
docker top new-api
```

## 更新部署

```bash
# 1. 拉取新镜像
docker pull your-registry.example.com/new-api:latest

# 2. 停止旧容器
docker stop new-api

# 3. 删除旧容器
docker rm new-api

# 4. 启动新容器（使用相同配置）
docker run -d \
  --name new-api \
  -p 3000:3000 \
  -e SQL_DSN="..." \
  -v /data/new-api:/data \
  your-registry.example.com/new-api:latest

# 或使用 Docker Compose 更新
docker-compose pull
docker-compose up -d
```

## 故障排查

### 问题 1: 容器启动失败

```bash
# 查看容器日志
docker logs new-api

# 检查端口占用
netstat -tuln | grep 3000

# 检查数据库连接
docker exec new-api ping mysql-host
```

### 问题 2: 数据库连接失败

```bash
# 检查数据库环境变量
docker exec new-api env | grep SQL_DSN

# 测试数据库连接
docker exec new-api-mysql mysql -u root -p -e "SELECT 1"
```

### 问题 3: 视频生成 API 表未创建

```bash
# 进入容器检查数据库
docker exec -it new-api-mysql mysql -u root -p new_api

# 查看表结构
SHOW TABLES LIKE 'video_projects';
DESC video_projects;

# 如果表不存在，需要重启服务触发迁移
docker restart new-api
```

## 安全建议

1. **修改默认密码**: 确保修改所有默认密码（MySQL、Redis、SESSION_SECRET）
2. **使用环境变量文件**: 敏感信息使用 `.env` 文件，不要提交到 Git
3. **限制网络访问**: 只暴露必要的端口
4. **定期更新**: 定期更新镜像以获取安全补丁
5. **使用 HTTPS**: 生产环境必须使用 HTTPS
6. **启用防火墙**: 限制容器之间的网络访问

## CI/CD 集成

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
```

### GitHub Actions 示例

```yaml
# .github/workflows/docker.yml
name: Build and Push Docker Image

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2
      
      - name: Login to Registry
        uses: docker/login-action@v2
        with:
          registry: ${{ secrets.REGISTRY }}
          username: ${{ secrets.REGISTRY_USERNAME }}
          password: ${{ secrets.REGISTRY_PASSWORD }}
      
      - name: Build and Push
        run: |
          chmod +x build-docker.sh
          ./build-docker.sh --multi-arch --push --tag-latest --registry ${{ secrets.REGISTRY }}
```

## 相关文档

- [视频生成 API 文档](./video-generation-api.md)
- [环境变量配置](../README.md#environment-variables)
- [数据库迁移指南](./database-migration.md)
