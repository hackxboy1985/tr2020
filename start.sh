#!/bin/bash
set -e

echo "=== docker build ==="
docker build . -t new-api

docker tag new-api:latest registry.cn-beijing.aliyuncs.com/mints-prod/new-api:latest
docker push registry.cn-beijing.aliyuncs.com/mints-prod/new-api:latest

echo "=== docker-compose up ==="
docker-compose up -d

echo "=== 启动完成 ==="
docker-compose logs -f
