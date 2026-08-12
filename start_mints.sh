#!/bin/bash
set -e

echo "=== docker build ==="
docker build . -t new-api-mints

docker tag new-api-mints:latest registry.cn-beijing.aliyuncs.com/mints-prod/new-api-mints:latest
docker push registry.cn-beijing.aliyuncs.com/mints-prod/new-api-mints:latest

echo "=== docker-compose up ==="
docker-compose -f docker-compose-mints.yml up -d

echo "=== 启动完成 ==="
docker-compose logs -f
