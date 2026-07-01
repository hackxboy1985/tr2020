#!/bin/bash
# 生产环境 Docker 镜像构建脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置
IMAGE_NAME="new-api"
REGISTRY="your-registry.example.com"  # 修改为你的镜像仓库地址
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# 打印信息
echo -e "${GREEN}=== 构建 Docker 镜像 ===${NC}"
echo "镜像名称: ${IMAGE_NAME}"
echo "版本标签: ${VERSION}"
echo "提交哈希: ${COMMIT_SHA}"
echo "构建时间: ${BUILD_DATE}"
echo ""

# 解析参数
PUSH=false
PLATFORM="linux/amd64"
TAG_LATEST=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --push)
      PUSH=true
      shift
      ;;
    --platform)
      PLATFORM="$2"
      shift 2
      ;;
    --multi-arch)
      PLATFORM="linux/amd64,linux/arm64"
      shift
      ;;
    --tag-latest)
      TAG_LATEST=true
      shift
      ;;
    --registry)
      REGISTRY="$2"
      shift 2
      ;;
    --help)
      echo "用法: $0 [选项]"
      echo ""
      echo "选项:"
      echo "  --push              构建后推送到镜像仓库"
      echo "  --platform PLATFORM 指定目标平台 (默认: linux/amd64)"
      echo "  --multi-arch        构建多架构镜像 (amd64 + arm64)"
      echo "  --tag-latest        同时打上 latest 标签"
      echo "  --registry REGISTRY 指定镜像仓库地址"
      echo "  --help              显示帮助信息"
      echo ""
      echo "示例:"
      echo "  $0                                    # 本地构建 amd64"
      echo "  $0 --push --tag-latest                # 构建并推送，打 latest 标签"
      echo "  $0 --multi-arch --push                # 多架构构建并推送"
      exit 0
      ;;
    *)
      echo -e "${RED}未知参数: $1${NC}"
      exit 1
      ;;
  esac
done

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

# 生成 VERSION 文件
echo "${VERSION}" > VERSION
echo -e "${GREEN}✓ 生成 VERSION 文件: ${VERSION}${NC}"

# 构建镜像标签
FULL_TAG="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
if [ "${TAG_LATEST}" = true ]; then
    TAGS="-t ${FULL_TAG} -t ${REGISTRY}/${IMAGE_NAME}:latest"
else
    TAGS="-t ${FULL_TAG}"
fi

# 构建参数
BUILD_ARGS="--build-arg VERSION=${VERSION} \
            --build-arg COMMIT_SHA=${COMMIT_SHA} \
            --build-arg BUILD_DATE=${BUILD_DATE} \
            --label org.opencontainers.image.version=${VERSION} \
            --label org.opencontainers.image.revision=${COMMIT_SHA} \
            --label org.opencontainers.image.created=${BUILD_DATE}"

echo -e "${YELLOW}开始构建...${NC}"

# 多架构构建需要 buildx
if [[ "${PLATFORM}" == *","* ]]; then
    echo -e "${YELLOW}多架构构建: ${PLATFORM}${NC}"

    # 确保 buildx 可用
    docker buildx version &> /dev/null || {
        echo -e "${RED}错误: Docker Buildx 不可用${NC}"
        exit 1
    }

    # 创建或使用 builder
    if ! docker buildx inspect new-api-builder &> /dev/null; then
        echo -e "${YELLOW}创建 buildx builder...${NC}"
        docker buildx create --name new-api-builder --use
    else
        docker buildx use new-api-builder
    fi

    # 构建命令
    BUILD_CMD="docker buildx build --platform ${PLATFORM} ${BUILD_ARGS} ${TAGS}"

    if [ "${PUSH}" = true ]; then
        BUILD_CMD="${BUILD_CMD} --push ."
    else
        BUILD_CMD="${BUILD_CMD} --load ."
        echo -e "${YELLOW}注意: 多架构镜像只能在推送时构建，已切换为 --push 模式${NC}"
        PUSH=true
    fi
else
    # 单架构构建
    echo -e "${YELLOW}单架构构建: ${PLATFORM}${NC}"
    BUILD_CMD="docker build --platform ${PLATFORM} ${BUILD_ARGS} ${TAGS} ."
fi

# 执行构建
echo -e "${YELLOW}执行: ${BUILD_CMD}${NC}"
eval ${BUILD_CMD}

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 镜像构建成功${NC}"
    echo ""
    echo "镜像标签:"
    echo "  ${FULL_TAG}"
    if [ "${TAG_LATEST}" = true ]; then
        echo "  ${REGISTRY}/${IMAGE_NAME}:latest"
    fi
else
    echo -e "${RED}✗ 镜像构建失败${NC}"
    exit 1
fi

# 推送镜像
if [ "${PUSH}" = true ] && [[ "${PLATFORM}" != *","* ]]; then
    echo ""
    echo -e "${YELLOW}推送镜像到仓库...${NC}"
    docker push ${FULL_TAG}

    if [ "${TAG_LATEST}" = true ]; then
        docker push ${REGISTRY}/${IMAGE_NAME}:latest
    fi

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 镜像推送成功${NC}"
    else
        echo -e "${RED}✗ 镜像推送失败${NC}"
        exit 1
    fi
fi

# 清理
rm -f VERSION

echo ""
echo -e "${GREEN}=== 构建完成 ===${NC}"
echo "下一步操作:"
echo "  1. 本地测试: docker run -p 3000:3000 ${FULL_TAG}"
echo "  2. 查看镜像: docker images | grep ${IMAGE_NAME}"
if [ "${PUSH}" = false ]; then
    echo "  3. 推送镜像: docker push ${FULL_TAG}"
fi
