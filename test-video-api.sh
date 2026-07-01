#!/bin/bash
# 视频生成系统 API 测试脚本

set -e

# 配置
API_BASE_URL="${API_BASE_URL:-http://localhost:3000}"
USER_TOKEN="${USER_TOKEN:-}"  # 需要先登录获取token

echo "=========================================="
echo "视频生成系统 API 测试"
echo "=========================================="
echo "API地址: $API_BASE_URL"
echo ""

if [ -z "$USER_TOKEN" ]; then
    echo "⚠️  警告: 未设置 USER_TOKEN"
    echo "请先登录系统获取token，然后:"
    echo "  export USER_TOKEN='your_token_here'"
    echo "  bash test-video-api.sh"
    echo ""
    echo "或者提供token作为参数:"
    echo "  bash test-video-api.sh your_token_here"
    echo ""

    if [ -n "$1" ]; then
        USER_TOKEN="$1"
        echo "✓ 使用提供的token"
    else
        exit 1
    fi
fi

echo "✓ 使用 Token: ${USER_TOKEN:0:20}..."
echo ""

# 1. 创建视频项目
echo "1️⃣  测试: 创建视频项目"
echo "----------------------------------------"
CREATE_RESPONSE=$(curl -s -X POST "$API_BASE_URL/api/video-generation/create" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_img_url": "https://example.com/test-product.jpg",
    "brand": "测试品牌",
    "product_name": "测试产品",
    "tagline": "这是一个测试宣传语",
    "selling_points": "卖点1\n卖点2\n卖点3",
    "prompt": "创建一个30秒的产品展示视频，展示产品的核心功能",
    "vtype": "产品展示",
    "duration": 30,
    "resolution": "2K",
    "whstr": "16:9",
    "channel_type": "platform"
  }')

echo "$CREATE_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$CREATE_RESPONSE"
echo ""

# 提取project_id
PROJECT_ID=$(echo "$CREATE_RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('data', {}).get('project_id', ''))" 2>/dev/null || echo "")

if [ -z "$PROJECT_ID" ]; then
    echo "❌ 创建项目失败，无法获取project_id"
    exit 1
fi

echo "✓ 项目创建成功，ID: $PROJECT_ID"
echo ""

# 2. 获取项目详情
echo "2️⃣  测试: 获取项目详情"
echo "----------------------------------------"
sleep 1
DETAIL_RESPONSE=$(curl -s -X GET "$API_BASE_URL/api/video-generation/projects/$PROJECT_ID" \
  -H "Authorization: Bearer $USER_TOKEN")

echo "$DETAIL_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$DETAIL_RESPONSE"
echo ""

# 3. 获取项目列表
echo "3️⃣  测试: 获取项目列表"
echo "----------------------------------------"
LIST_RESPONSE=$(curl -s -X GET "$API_BASE_URL/api/video-generation/projects?page=1&page_size=10" \
  -H "Authorization: Bearer $USER_TOKEN")

echo "$LIST_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$LIST_RESPONSE"
echo ""

# 4. 模拟Webhook回调（需要管理员权限或签名）
echo "4️⃣  测试: 模拟Webhook回调"
echo "----------------------------------------"
echo "注意: 这个测试需要正确的签名，通常由上游平台调用"
echo "跳过webhook测试（需要在上游平台配置后自动触发）"
echo ""

# 5. 删除项目（可选）
echo "5️⃣  测试: 删除项目 (输入 'yes' 确认)"
echo "----------------------------------------"
read -p "是否删除测试项目? (yes/no): " CONFIRM

if [ "$CONFIRM" = "yes" ]; then
    DELETE_RESPONSE=$(curl -s -X DELETE "$API_BASE_URL/api/video-generation/projects/$PROJECT_ID" \
      -H "Authorization: Bearer $USER_TOKEN")

    echo "$DELETE_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$DELETE_RESPONSE"
    echo "✓ 项目已删除"
else
    echo "⏭️  跳过删除，项目ID: $PROJECT_ID"
fi
echo ""

echo "=========================================="
echo "✓ API测试完成"
echo "=========================================="
echo ""
echo "测试的项目ID: $PROJECT_ID"
echo ""
echo "下一步:"
echo "1. 在上游平台配置Webhook地址:"
echo "   - Coze: $API_BASE_URL/api/video-generation/webhook/coze"
echo "   - Platform: $API_BASE_URL/api/video-generation/webhook/platform"
echo ""
echo "2. 触发视频生成后，webhook会自动更新项目状态"
echo ""
echo "3. 通过详情接口查询项目进度:"
echo "   curl -H 'Authorization: Bearer \$TOKEN' \\"
echo "     $API_BASE_URL/api/video-generation/projects/$PROJECT_ID"
