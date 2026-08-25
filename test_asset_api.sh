#!/bin/bash

# Seedance Asset API 测试脚本
# 使用方法: ./test_asset_api.sh

# 配置
BASE_URL="http://localhost:3000"
API_KEY="sk-your-api-key-here"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "======================================"
echo "Seedance Asset API 测试脚本"
echo "======================================"
echo ""

# 检查 API Key
if [ "$API_KEY" = "sk-your-api-key-here" ]; then
    echo -e "${RED}错误: 请先设置 API_KEY${NC}"
    echo "请编辑脚本，将 API_KEY 设置为你的实际 token"
    exit 1
fi

# 测试函数
test_action() {
    local action=$1
    local data=$2
    local description=$3

    echo -e "${YELLOW}测试: $description${NC}"
    echo "Action: $action"
    echo "请求体:"
    echo "$data" | jq '.' 2>/dev/null || echo "$data"
    echo ""

    response=$(curl -s -X POST \
        "${BASE_URL}/api/seedance/assets/v2/?Action=${action}&Version=2024-01-01" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$data")

    echo "响应:"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo ""
    echo "--------------------------------------"
    echo ""
}

# 1. 测试创建真人认证会话
echo -e "${GREEN}=== 1. 真人认证接口 ===${NC}"
echo ""

test_action "CreateVisualValidateSession" \
'{
  "CallbackURL": "https://www.example.com/callback",
  "ProjectName": "default"
}' \
"创建真人认证会话"

# 保存 BytedToken（需要手动从响应中提取）
echo -e "${YELLOW}提示: 请记录上面返回的 BytedToken，稍后会用到${NC}"
echo ""
read -p "请输入 BytedToken (或按 Enter 跳过): " BYTED_TOKEN
echo ""

if [ ! -z "$BYTED_TOKEN" ]; then
    test_action "GetVisualValidateResult" \
    "{
      \"BytedToken\": \"${BYTED_TOKEN}\",
      \"ProjectName\": \"default\"
    }" \
    "获取真人认证结果"
fi

# 2. 测试 Asset Group 管理
echo -e "${GREEN}=== 2. Asset Group 管理接口 ===${NC}"
echo ""

test_action "CreateAssetGroup" \
'{
  "Name": "test_group_001",
  "Title": "测试素材组",
  "Description": "用于 API 测试的素材组",
  "GroupType": "LivenessFace",
  "ProjectName": "default"
}' \
"创建素材组"

# 保存 GroupId（需要手动从响应中提取）
echo -e "${YELLOW}提示: 请记录上面返回的 GroupId${NC}"
echo ""
read -p "请输入 GroupId (或按 Enter 跳过): " GROUP_ID
echo ""

if [ ! -z "$GROUP_ID" ]; then
    test_action "GetAssetGroup" \
    "{
      \"Id\": \"${GROUP_ID}\",
      \"ProjectName\": \"default\"
    }" \
    "查询素材组详情"

    test_action "UpdateAssetGroup" \
    "{
      \"Id\": \"${GROUP_ID}\",
      \"Name\": \"test_group_001_updated\",
      \"Description\": \"更新后的描述\",
      \"ProjectName\": \"default\"
    }" \
    "更新素材组信息"
fi

test_action "ListAssetGroups" \
'{
  "Filter": {
    "GroupType": "LivenessFace"
  },
  "PageNumber": 1,
  "PageSize": 10,
  "ProjectName": "default"
}' \
"查询素材组列表"

# 3. 测试 Asset 管理
echo -e "${GREEN}=== 3. Asset 管理接口 ===${NC}"
echo ""

if [ ! -z "$GROUP_ID" ]; then
    test_action "CreateAsset" \
    "{
      \"GroupId\": \"${GROUP_ID}\",
      \"URL\": \"https://example.com/test-image.jpg\",
      \"AssetType\": \"Image\",
      \"Name\": \"测试图片\",
      \"ProjectName\": \"default\"
    }" \
    "创建素材资产"

    echo -e "${YELLOW}提示: 请记录上面返回的 Asset ID${NC}"
    echo ""
    read -p "请输入 Asset ID (或按 Enter 跳过): " ASSET_ID
    echo ""

    if [ ! -z "$ASSET_ID" ]; then
        test_action "GetAsset" \
        "{
          \"Id\": \"${ASSET_ID}\",
          \"ProjectName\": \"default\"
        }" \
        "查询素材详情"

        test_action "UpdateAsset" \
        "{
          \"Id\": \"${ASSET_ID}\",
          \"Name\": \"更新后的图片\",
          \"ProjectName\": \"default\"
        }" \
        "更新素材信息"
    fi

    test_action "ListAssets" \
    "{
      \"Filter\": {
        \"GroupIds\": [\"${GROUP_ID}\"],
        \"GroupType\": \"LivenessFace\"
      },
      \"PageNumber\": 1,
      \"PageSize\": 10,
      \"ProjectName\": \"default\"
    }" \
    "查询素材列表"

    # 删除操作（可选）
    echo ""
    read -p "是否要删除测试资源? (y/N): " DELETE_CONFIRM
    echo ""

    if [ "$DELETE_CONFIRM" = "y" ] || [ "$DELETE_CONFIRM" = "Y" ]; then
        if [ ! -z "$ASSET_ID" ]; then
            test_action "DeleteAsset" \
            "{
              \"Id\": \"${ASSET_ID}\",
              \"ProjectName\": \"default\"
            }" \
            "删除素材"
        fi

        test_action "DeleteAssetGroup" \
        "{
          \"Id\": \"${GROUP_ID}\",
          \"ProjectName\": \"default\"
        }" \
        "删除素材组"
    fi
fi

echo ""
echo -e "${GREEN}======================================"
echo "测试完成!"
echo "======================================${NC}"
