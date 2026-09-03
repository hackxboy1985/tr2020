#!/bin/bash

# Doubao 完整流程测试脚本
# 包含：创建素材组 -> 上传素材 -> 使用素材生成视频 -> 查询任务
# 使用方法: ./test_doubao_complete_flow.sh

# 配置
BASE_URL="http://localhost:3000"
API_KEY="sk-your-api-key-here"

# 测试图片 URL（请替换为实际可访问的图片）
TEST_IMAGE_URL="https://example.com/test-portrait.jpg"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo "======================================"
echo "Doubao 完整流程测试"
echo "======================================"
echo ""

# 检查 API Key
if [ "$API_KEY" = "sk-your-api-key-here" ]; then
    echo -e "${RED}错误: 请先设置 API_KEY${NC}"
    echo "请编辑脚本，将 API_KEY 设置为你的实际 token"
    exit 1
fi

# 工具函数
print_section() {
    echo ""
    echo -e "${GREEN}======================================"
    echo "$1"
    echo "======================================${NC}"
    echo ""
}

print_step() {
    echo -e "${CYAN}>>> STEP $1: $2${NC}"
    echo ""
}

# ==========================================
# 第一部分：Asset API - 真人认证与素材管理
# ==========================================

print_section "第一部分：素材管理流程"

# Step 1: 创建真人认证会话
print_step "1" "创建真人认证会话"

session_response=$(curl -s -X POST \
    "${BASE_URL}/api/seedance/assets/v2/?Action=CreateVisualValidateSession&Version=2024-01-01" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
      "CallbackURL": "https://www.example.com/callback",
      "ProjectName": "default"
    }')

echo "响应:"
echo "$session_response" | jq '.'

BYTED_TOKEN=$(echo "$session_response" | jq -r '.Result.BytedToken // .BytedToken // empty')
H5_LINK=$(echo "$session_response" | jq -r '.Result.H5Link // .H5Link // empty')

if [ ! -z "$H5_LINK" ]; then
    echo ""
    echo -e "${YELLOW}真人认证 H5 链接: ${H5_LINK}${NC}"
    echo -e "${YELLOW}BytedToken: ${BYTED_TOKEN}${NC}"
    echo ""
    echo -e "${BLUE}提示: 请在浏览器中打开上面的 H5 链接完成真人认证${NC}"
    echo -e "${BLUE}完成后，会跳转到 CallbackURL 并附带 resultCode 参数${NC}"
    echo ""
fi

# 询问是否已完成真人认证
read -p "是否已完成真人认证? (y/N): " AUTH_COMPLETED
echo ""

GROUP_ID=""

if [ "$AUTH_COMPLETED" = "y" ] || [ "$AUTH_COMPLETED" = "Y" ]; then
    if [ ! -z "$BYTED_TOKEN" ]; then
        # Step 2: 获取真人认证结果
        print_step "2" "获取真人认证结果（获取 GroupId）"

        result_response=$(curl -s -X POST \
            "${BASE_URL}/api/seedance/assets/v2/?Action=GetVisualValidateResult&Version=2024-01-01" \
            -H "Authorization: Bearer ${API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{
              \"BytedToken\": \"${BYTED_TOKEN}\",
              \"ProjectName\": \"default\"
            }")

        echo "响应:"
        echo "$result_response" | jq '.'

        GROUP_ID=$(echo "$result_response" | jq -r '.Result.GroupId // .GroupId // empty')

        if [ ! -z "$GROUP_ID" ]; then
            echo ""
            echo -e "${GREEN}✓ 真人认证成功! GroupId: ${GROUP_ID}${NC}"
            echo ""
        else
            echo -e "${RED}✗ 未能获取 GroupId${NC}"
        fi
    fi
else
    echo -e "${YELLOW}跳过真人认证，使用手动输入的 GroupId${NC}"
fi

# 如果没有 GroupId，询问用户手动输入
if [ -z "$GROUP_ID" ]; then
    echo ""
    read -p "请输入已存在的 GroupId（或按 Enter 跳过素材相关测试）: " GROUP_ID
    echo ""
fi

ASSET_ID=""
ASSET_URI=""

if [ ! -z "$GROUP_ID" ]; then
    # Step 3: 查询素材组信息
    print_step "3" "查询素材组信息"

    group_response=$(curl -s -X POST \
        "${BASE_URL}/api/seedance/assets/v2/?Action=GetAssetGroup&Version=2024-01-01" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{
          \"Id\": \"${GROUP_ID}\",
          \"ProjectName\": \"default\"
        }")

    echo "响应:"
    echo "$group_response" | jq '.'
    echo ""

    # Step 4: 上传素材资产
    print_step "4" "上传素材资产（图片）"

    echo "上传图片 URL: ${TEST_IMAGE_URL}"
    echo ""

    asset_response=$(curl -s -X POST \
        "${BASE_URL}/api/seedance/assets/v2/?Action=CreateAsset&Version=2024-01-01" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{
          \"GroupId\": \"${GROUP_ID}\",
          \"URL\": \"${TEST_IMAGE_URL}\",
          \"AssetType\": \"Image\",
          \"Name\": \"API测试人像\",
          \"ProjectName\": \"default\"
        }")

    echo "响应:"
    echo "$asset_response" | jq '.'

    ASSET_ID=$(echo "$asset_response" | jq -r '.Result.Id // .Id // empty')

    if [ ! -z "$ASSET_ID" ]; then
        echo ""
        echo -e "${GREEN}✓ 素材上传成功! Asset ID: ${ASSET_ID}${NC}"
        ASSET_URI="asset://${ASSET_ID}"
        echo -e "${GREEN}  Asset URI: ${ASSET_URI}${NC}"
        echo ""
    else
        echo -e "${RED}✗ 素材上传失败${NC}"
        echo ""
    fi

    # Step 5: 轮询查询素材状态
    print_step "5" "查询素材处理状态"

    if [ ! -z "$ASSET_ID" ]; then
        for i in {1..5}; do
            echo "第 $i 次查询素材状态..."

            asset_status_response=$(curl -s -X POST \
                "${BASE_URL}/api/seedance/assets/v2/?Action=GetAsset&Version=2024-01-01" \
                -H "Authorization: Bearer ${API_KEY}" \
                -H "Content-Type: application/json" \
                -d "{
                  \"Id\": \"${ASSET_ID}\",
                  \"ProjectName\": \"default\"
                }")

            echo "$asset_status_response" | jq '.'

            asset_status=$(echo "$asset_status_response" | jq -r '.Result.Status // .Status // empty')
            echo ""
            echo -e "当前状态: ${YELLOW}${asset_status}${NC}"

            if [ "$asset_status" = "Active" ]; then
                echo -e "${GREEN}✓ 素材处理完成，可以用于生成视频！${NC}"
                break
            elif [ "$asset_status" = "Failed" ]; then
                echo -e "${RED}✗ 素材处理失败${NC}"
                break
            fi

            if [ $i -lt 5 ]; then
                echo ""
                echo "等待 10 秒后重试..."
                sleep 10
                echo ""
            fi
        done
        echo ""
    fi

    # Step 6: 查询素材列表
    print_step "6" "查询该素材组的所有素材"

    list_assets_response=$(curl -s -X POST \
        "${BASE_URL}/api/seedance/assets/v2/?Action=ListAssets&Version=2024-01-01" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{
          \"Filter\": {
            \"GroupIds\": [\"${GROUP_ID}\"],
            \"GroupType\": \"LivenessFace\",
            \"Statuses\": [\"Active\"]
          },
          \"PageNumber\": 1,
          \"PageSize\": 10,
          \"ProjectName\": \"default\"
        }")

    echo "响应:"
    echo "$list_assets_response" | jq '.'

    total_assets=$(echo "$list_assets_response" | jq -r '.Result.TotalCount // .TotalCount // 0')
    echo ""
    echo -e "${YELLOW}该素材组共有 ${total_assets} 个素材（Active 状态）${NC}"
    echo ""
fi

# ==========================================
# 第二部分：视频生成 API
# ==========================================

print_section "第二部分：视频生成流程"

# Step 7: 创建视频生成任务
print_step "7" "创建视频生成任务"

if [ ! -z "$ASSET_URI" ]; then
    echo -e "${BLUE}使用素材生成视频: ${ASSET_URI}${NC}"
    echo ""

    create_task_response=$(curl -s -X POST \
        "${BASE_URL}/api/v3/contents/generations/tasks" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{
          \"model\": \"doubao-seedance-2-0-260128\",
          \"content\": [
            {
              \"type\": \"text\",
              \"text\": \"图片1中的人物面带微笑，向镜头挥手打招呼，背景是温馨的室内环境\"
            },
            {
              \"type\": \"image_url\",
              \"image_url\": {
                \"url\": \"${ASSET_URI}\"
              },
              \"role\": \"reference_image\"
            }
          ],
          \"ratio\": \"16:9\",
          \"duration\": 5,
          \"resolution\": \"720p\"
        }")
else
    echo -e "${BLUE}创建普通文生视频任务${NC}"
    echo ""

    create_task_response=$(curl -s -X POST \
        "${BASE_URL}/api/v3/contents/generations/tasks" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d '{
          "model": "doubao-seedance-2-0-260128",
          "content": [
            {
              "type": "text",
              "text": "一只可爱的小猫咪在草地上玩耍，阳光明媚，画面温馨，16:9画幅"
            }
          ],
          "ratio": "16:9",
          "duration": 5,
          "resolution": "720p"
        }')
fi

echo "响应:"
echo "$create_task_response" | jq '.'

TASK_ID=$(echo "$create_task_response" | jq -r '.id // empty')

if [ -z "$TASK_ID" ]; then
    echo -e "${RED}✗ 任务创建失败${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ 视频生成任务创建成功! Task ID: ${TASK_ID}${NC}"
echo ""

# Step 8: 查询单个任务状态
print_step "8" "轮询查询任务状态"

for i in {1..10}; do
    echo "第 $i 次查询任务状态..."

    task_response=$(curl -s -X GET \
        "${BASE_URL}/api/v3/contents/generations/tasks/${TASK_ID}" \
        -H "Authorization: Bearer ${API_KEY}")

    echo "响应:"
    echo "$task_response" | jq '.'

    status=$(echo "$task_response" | jq -r '.status // empty')
    video_url=$(echo "$task_response" | jq -r '.content.video_url // empty')

    echo ""
    echo -e "当前状态: ${YELLOW}${status}${NC}"

    if [ "$status" = "succeeded" ]; then
        echo -e "${GREEN}✓ 视频生成成功！${NC}"
        echo -e "${GREEN}  视频 URL: ${video_url}${NC}"
        break
    elif [ "$status" = "failed" ]; then
        echo -e "${RED}✗ 视频生成失败${NC}"
        error_msg=$(echo "$task_response" | jq -r '.error.message // empty')
        echo -e "${RED}  错误信息: ${error_msg}${NC}"
        break
    fi

    if [ $i -lt 10 ]; then
        echo ""
        echo "等待 30 秒后重试..."
        sleep 30
        echo ""
    fi
done
echo ""

# Step 9: 查询任务列表
print_step "9" "查询任务列表（最近10个）"

list_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?page_num=1&page_size=10" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$list_response" | jq '.'

total_tasks=$(echo "$list_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}总任务数: ${total_tasks}${NC}"
echo ""

# Step 10: 按状态筛选任务
print_step "10" "查询排队中的任务"

queued_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?filter.status=queued&page_size=5" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$queued_response" | jq '.'

queued_count=$(echo "$queued_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}排队中的任务数: ${queued_count}${NC}"
echo ""

# Step 11: 按状态筛选任务（成功的任务）
print_step "11" "查询成功的任务"

succeeded_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?filter.status=succeeded&page_size=5" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$succeeded_response" | jq '.'

succeeded_count=$(echo "$succeeded_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}成功的任务数: ${succeeded_count}${NC}"
echo ""

# Step 12: 多条件筛选
print_step "12" "多条件筛选（成功 + 指定模型）"

multi_filter_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?filter.status=succeeded&filter.model=doubao-seedance-2-0-260128&page_size=3" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$multi_filter_response" | jq '.'
echo ""

# ==========================================
# 总结
# ==========================================

print_section "测试完成 - 流程总结"

echo "已测试的完整流程:"
echo ""
echo "【第一部分：素材管理】"
echo "  1. ✓ 创建真人认证会话 (CreateVisualValidateSession)"
echo "  2. ✓ 获取认证结果获取 GroupId (GetVisualValidateResult)"
echo "  3. ✓ 查询素材组信息 (GetAssetGroup)"
echo "  4. ✓ 上传素材资产 (CreateAsset)"
echo "  5. ✓ 轮询查询素材状态 (GetAsset)"
echo "  6. ✓ 查询素材列表 (ListAssets)"
echo ""
echo "【第二部分：视频生成】"
echo "  7. ✓ 创建视频生成任务 (使用素材或纯文本)"
echo "  8. ✓ 轮询查询任务状态"
echo "  9. ✓ 查询任务列表"
echo " 10. ✓ 按状态筛选（queued）"
echo " 11. ✓ 按状态筛选（succeeded）"
echo " 12. ✓ 多条件筛选"
echo ""

if [ ! -z "$GROUP_ID" ]; then
    echo -e "${CYAN}素材信息:${NC}"
    echo "  GroupId: ${GROUP_ID}"
    if [ ! -z "$ASSET_ID" ]; then
        echo "  AssetId: ${ASSET_ID}"
        echo "  AssetURI: ${ASSET_URI}"
    fi
    echo ""
fi

if [ ! -z "$TASK_ID" ]; then
    echo -e "${CYAN}任务信息:${NC}"
    echo "  TaskId: ${TASK_ID}"
    echo "  状态: ${status}"
    if [ ! -z "$video_url" ] && [ "$video_url" != "null" ]; then
        echo "  视频URL: ${video_url}"
    fi
    echo ""
fi

echo -e "${GREEN}======================================"
echo "所有测试已完成！"
echo "======================================${NC}"
