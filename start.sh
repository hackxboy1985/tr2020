#!/bin/bash
set -e

echo "=== docker build ==="
docker build . -t new-api

echo "=== docker-compose up ==="
docker-compose up -d

echo "=== 启动完成 ==="
docker-compose logs -f
