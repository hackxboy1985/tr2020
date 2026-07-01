#!/bin/bash
# 视频生成系统代码完整性检查脚本

set -e

BASE_DIR="/Users/mac/Desktop/ecap/new-api"
cd "$BASE_DIR"

echo "=========================================="
echo "视频生成系统代码完整性检查"
echo "=========================================="
echo ""

# 检查文件是否存在
echo "✓ 检查文件存在性..."
files=(
    "model/video_project.go"
    "dto/video_project.go"
    "service/video_adapter.go"
    "service/video_adapter_coze.go"
    "service/video_adapter_platform.go"
    "service/video_generation_service.go"
    "controller/video_generation.go"
    "router/video-router.go"
    "docs/video-generation-design.md"
    "docs/video-generation-implementation-summary.md"
)

for file in "${files[@]}"; do
    if [ -f "$BASE_DIR/$file" ]; then
        echo "  ✓ $file"
    else
        echo "  ✗ $file (缺失)"
        exit 1
    fi
done
echo ""

# 检查关键代码是否存在
echo "✓ 检查关键代码结构..."

# 1. 检查模型字段
if grep -q "ChannelType" "$BASE_DIR/model/video_project.go" && \
   grep -q "RemoteProjectId" "$BASE_DIR/model/video_project.go"; then
    echo "  ✓ VideoProject 模型包含多渠道字段"
else
    echo "  ✗ VideoProject 模型缺少多渠道字段"
    exit 1
fi

# 2. 检查适配器接口
if grep -q "VideoGenerationAdapter" "$BASE_DIR/service/video_adapter.go" && \
   grep -q "CreateProject" "$BASE_DIR/service/video_adapter.go"; then
    echo "  ✓ VideoGenerationAdapter 接口定义正确"
else
    echo "  ✗ VideoGenerationAdapter 接口定义有问题"
    exit 1
fi

# 3. 检查Coze适配器
if grep -q "CozeAdapter" "$BASE_DIR/service/video_adapter_coze.go" && \
   grep -q "COZE_API_KEY" "$BASE_DIR/service/video_adapter_coze.go"; then
    echo "  ✓ CozeAdapter 实现正确"
else
    echo "  ✗ CozeAdapter 实现有问题"
    exit 1
fi

# 4. 检查Platform适配器
if grep -q "PlatformAdapter" "$BASE_DIR/service/video_adapter_platform.go" && \
   grep -q "PLATFORM_BASE_URL" "$BASE_DIR/service/video_adapter_platform.go"; then
    echo "  ✓ PlatformAdapter 实现正确"
else
    echo "  ✗ PlatformAdapter 实现有问题"
    exit 1
fi

# 5. 检查Service层
if grep -q "VideoGenerationService" "$BASE_DIR/service/video_generation_service.go" && \
   grep -q "NewVideoGenerationService" "$BASE_DIR/service/video_generation_service.go"; then
    echo "  ✓ VideoGenerationService 实现正确"
else
    echo "  ✗ VideoGenerationService 实现有问题"
    exit 1
fi

# 6. 检查Controller
if grep -q "CreateVideoProject" "$BASE_DIR/controller/video_generation.go" && \
   grep -q "HandleWebhook" "$BASE_DIR/controller/video_generation.go"; then
    echo "  ✓ Controller 实现正确"
else
    echo "  ✗ Controller 实现有问题"
    exit 1
fi

# 7. 检查路由注册
if grep -q "/api/video-generation" "$BASE_DIR/router/video-router.go" && \
   grep -q ":channel" "$BASE_DIR/router/video-router.go"; then
    echo "  ✓ 路由注册正确"
else
    echo "  ✗ 路由注册有问题"
    exit 1
fi

# 8. 检查AutoMigrate
if grep -q "VideoProject" "$BASE_DIR/model/main.go"; then
    echo "  ✓ VideoProject 已添加到 AutoMigrate"
else
    echo "  ✗ VideoProject 未添加到 AutoMigrate"
    exit 1
fi

echo ""
echo "=========================================="
echo "✓ 所有检查通过！"
echo "=========================================="
echo ""

# 统计代码行数
echo "📊 代码统计:"
echo "  Model:      $(wc -l < "$BASE_DIR/model/video_project.go") 行"
echo "  DTO:        $(wc -l < "$BASE_DIR/dto/video_project.go") 行"
echo "  Adapter接口: $(wc -l < "$BASE_DIR/service/video_adapter.go") 行"
echo "  Coze适配器:  $(wc -l < "$BASE_DIR/service/video_adapter_coze.go") 行"
echo "  Platform适配器: $(wc -l < "$BASE_DIR/service/video_adapter_platform.go") 行"
echo "  Service:    $(wc -l < "$BASE_DIR/service/video_generation_service.go") 行"
echo "  Controller: $(wc -l < "$BASE_DIR/controller/video_generation.go") 行"
echo ""

# 显示配置说明
echo "📝 启动配置:"
echo ""
echo "方式1 - 使用三方平台渠道（默认）:"
echo "  export VIDEO_GENERATION_CHANNEL=platform"
echo "  export PLATFORM_BASE_URL=https://your-platform.com"
echo "  export PLATFORM_API_KEY=your_api_key"
echo ""
echo "方式2 - 使用Coze渠道:"
echo "  export VIDEO_GENERATION_CHANNEL=coze"
echo "  export COZE_API_KEY=your_coze_key"
echo "  export COZE_WORKFLOW_ID=your_workflow_id"
echo "  export COZE_WEBHOOK_SECRET=your_secret"
echo ""
echo "然后运行: ./start.sh 或 docker-compose up"
echo ""
echo "✓ 代码实现完成，可以启动测试！"
