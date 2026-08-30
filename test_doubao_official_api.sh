#!/bin/bash

# Doubao 官方 API 测试脚本
# 完整测试流程：分组 → 素材 → 视频任务 → 查询 → 取消
#
# 使用方法:
#   ./test_doubao_official_api.sh                 # 执行完整测试流程
#   ./test_doubao_official_api.sh --query <id>    # 查询指定任务状态
#   ./test_doubao_official_api.sh --query all     # 查询任务列表
#   ./test_doubao_official_api.sh --cancel <id>   # 取消指定任务
#   ./test_doubao_official_api.sh --test-cancel   # 仅测试：创建任务并立即取消

# 配置
BASE_URL="http://book2:3002"
API_KEY="sk-60oOqvQYb8vziFfg2hPHlTKW3X80Pc6sIBDC5EFHCY0sn5NY"
GROUP_NAME="doubao-test-group"
TEST_IMAGE_URL="https://static.horse-world.mints-id.com/gy/general/1/image/2026-07-08/1783497394035_4147.png"
#TEST_IMAGE_URL="https://images.unsplash.com/photo-1514888286974-6c03e2ca1dba?w=800"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

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

print_test() {
    echo -e "${BLUE}> $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${CYAN}ℹ $1${NC}"
}

print_request() {
    local method="$1"
    local url="$2"
    local body="$3"

    echo ""
    echo -e "${CYAN}━━━ 请求详情 ━━━${NC}"
    echo -e "${YELLOW}Method:${NC} ${method}"
    echo -e "${YELLOW}URL:${NC} ${url}"
    echo -e "${YELLOW}Headers:${NC}"
    echo "  Authorization: Bearer ${API_KEY:0:10}..."
    echo "  Content-Type: application/json"
    if [ -n "$body" ]; then
        echo -e "${YELLOW}Body:${NC}"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    fi
    echo -e "${CYAN}━━━━━━━━━━━━━━━${NC}"
    echo ""
}

print_response() {
    local response="$1"
    local http_status="$2"

    echo ""
    echo -e "${CYAN}━━━ 响应详情 ━━━${NC}"
    if [ -n "$http_status" ]; then
        echo -e "${YELLOW}HTTP Status:${NC} ${http_status}"
    fi
    echo -e "${YELLOW}Body:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo -e "${CYAN}━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# 验证 doubao 官方格式
validate_official_format() {
    local response="$1"
    local type="$2"  # task_single, task_list, task_create

    case $type in
        "task_create")
            # 创建任务响应：必须有 id 字段
            if echo "$response" | jq -e '.id' >/dev/null 2>&1; then
                print_success "格式校验通过: 包含 id 字段"
                return 0
            else
                print_error "格式校验失败: 缺少 id 字段"
                return 1
            fi
            ;;
        "task_single")
            # 单个任务响应：必须有 id, model, status 字段
            local has_id=$(echo "$response" | jq -e '.id' >/dev/null 2>&1 && echo "yes" || echo "no")
            local has_model=$(echo "$response" | jq -e '.model' >/dev/null 2>&1 && echo "yes" || echo "no")
            local has_status=$(echo "$response" | jq -e '.status' >/dev/null 2>&1 && echo "yes" || echo "no")

            if [ "$has_id" = "yes" ] && [ "$has_model" = "yes" ] && [ "$has_status" = "yes" ]; then
                print_success "格式校验通过: 包含 id, model, status 字段"

                # 检查状态值是否为官方值
                local status=$(echo "$response" | jq -r '.status')
                case $status in
                    queued|running|succeeded|failed|cancelled)
                        print_success "状态值校验通过: ${status}"
                        ;;
                    *)
                        print_error "状态值非官方格式: ${status}"
                        ;;
                esac
                return 0
            else
                print_error "格式校验失败: 缺少必要字段 (id=${has_id}, model=${has_model}, status=${has_status})"
                return 1
            fi
            ;;
        "task_list")
            # 任务列表响应：必须有 items 和 total 字段
            local has_items=$(echo "$response" | jq -e '.items' >/dev/null 2>&1 && echo "yes" || echo "no")
            local has_total=$(echo "$response" | jq -e '.total' >/dev/null 2>&1 && echo "yes" || echo "no")

            if [ "$has_items" = "yes" ] && [ "$has_total" = "yes" ]; then
                print_success "格式校验通过: 包含 items 和 total 字段"

                # 检查 items 是否为数组
                if echo "$response" | jq -e '.items | type == "array"' >/dev/null 2>&1; then
                    print_success "items 类型校验通过: 数组"
                else
                    print_error "items 类型错误: 不是数组"
                fi
                return 0
            else
                print_error "格式校验失败: 缺少必要字段 (items=${has_items}, total=${has_total})"
                return 1
            fi
            ;;
    esac
}

# 子命令：查询任务状态
query_task() {
    local task_id="$1"

    if [ "$task_id" = "all" ]; then
        # 查询任务列表
        print_section "查询任务列表"

        local url="${BASE_URL}/api/v3/contents/generations/tasks?page_num=1&page_size=10"
        print_request "GET" "$url"

        response=$(curl -s -X GET "$url" \
            -H "Authorization: Bearer ${API_KEY}")

        print_response "$response"
        validate_official_format "$response" "task_list"

        total=$(echo "$response" | jq -r '.total // 0')
        echo ""
        echo -e "${YELLOW}总任务数: ${total}${NC}"
    else
        # 查询指定任务
        print_section "查询任务: ${task_id}"

        local url="${BASE_URL}/api/v3/contents/generations/tasks/${task_id}"
        print_request "GET" "$url"

        response=$(curl -s -X GET "$url" \
            -H "Authorization: Bearer ${API_KEY}")

        print_response "$response"
        validate_official_format "$response" "task_single"

        status=$(echo "$response" | jq -r '.status // "unknown"')
        echo ""
        echo -e "${YELLOW}任务状态: ${status}${NC}"
    fi

    exit 0
}

# 子命令：取消任务
cancel_task() {
    local task_id="$1"

    print_section "取消任务: ${task_id}"

    local cancel_url="${BASE_URL}/api/v3/contents/generations/tasks/${task_id}"
    print_request "DELETE" "$cancel_url"

    cancel_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$cancel_url" \
        -H "Authorization: Bearer ${API_KEY}")

    response_body=$(echo "$cancel_response" | sed -e 's/HTTP_STATUS\:.*//g')
    http_status=$(echo "$cancel_response" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

    print_response "$response_body" "$http_status"

    case $http_status in
        200)
            print_success "取消成功 (HTTP 200)"
            echo ""
            echo -e "${GREEN}任务已成功取消${NC}"
            ;;
        404)
            print_error "任务不存在 (HTTP 404)"
            ;;
        409)
            print_error "任务运行中，无法取消 (HTTP 409)"
            ;;
        400)
            print_error "任务状态不支持取消 (HTTP 400)"
            local code=$(echo "$response_body" | jq -r '.code // "unknown"')
            local message=$(echo "$response_body" | jq -r '.message // "unknown"')
            echo ""
            echo -e "${YELLOW}错误代码: ${code}${NC}"
            echo -e "${YELLOW}错误信息: ${message}${NC}"
            ;;
        *)
            print_error "未预期的状态码: ${http_status}"
            ;;
    esac

    exit 0
}

# 处理命令行参数
if [ "$1" = "--query" ]; then
    if [ -z "$2" ]; then
        echo -e "${RED}错误: --query 需要指定任务 ID 或 'all'${NC}"
        echo "用法: $0 --query <task_id>"
        echo "      $0 --query all"
        exit 1
    fi
    query_task "$2"
fi

if [ "$1" = "--cancel" ]; then
    if [ -z "$2" ]; then
        echo -e "${RED}错误: --cancel 需要指定任务 ID${NC}"
        echo "用法: $0 --cancel <task_id>"
        exit 1
    fi
    cancel_task "$2"
fi

# ========================================
# 仅测试创建和取消任务
# ========================================
if [ "$1" = "--test-cancel" ]; then
    print_section "测试：创建任务并立即取消"

    print_test "1. 创建视频任务"

    create_url="${BASE_URL}/api/v3/contents/generations/tasks"
    create_body=$(cat <<EOF
{
  "model": "doubao-seedance-2-0-mini-260615",
  "content": [
    {
      "type": "text",
      "text": "测试取消任务"
    }
  ],
  "duration": 4
}
EOF
)

    print_request "POST" "$create_url" "$create_body"

    create_response=$(curl -s -X POST "$create_url" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$create_body")

    print_response "$create_response"

    TASK_ID=$(echo "$create_response" | jq -r '.id // empty')

    if [ -z "$TASK_ID" ]; then
        print_error "任务创建失败"
        exit 1
    fi

    print_success "任务创建成功! Task ID: ${TASK_ID}"
    echo ""

    print_test "2. 立即取消任务"

    cancel_url="${BASE_URL}/api/v3/contents/generations/tasks/${TASK_ID}"
    print_request "DELETE" "$cancel_url"

    cancel_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$cancel_url" \
        -H "Authorization: Bearer ${API_KEY}")

    response_body=$(echo "$cancel_response" | sed -e 's/HTTP_STATUS\:.*//g')
    http_status=$(echo "$cancel_response" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

    echo -e "${CYAN}━━━ 响应详情 ━━━${NC}"
    echo "Status: ${http_status}"
    echo "Body:"
    echo "$response_body" | jq '.' 2>/dev/null || echo "$response_body"
    echo -e "${CYAN}━━━━━━━━━━━━━━━${NC}"
    echo ""

    if [ "$http_status" = "200" ]; then
        print_success "任务取消成功!"
    else
        print_error "任务取消失败! HTTP ${http_status}"
        echo "$response_body"
    fi

    exit 0
fi

# ========================================
# 完整测试流程
# ========================================

echo "======================================"
echo "Doubao 官方 API 完整测试流程"
echo "======================================"
echo ""
print_info "测试流程: 分组 → 素材 → 视频任务 → 查询 → 取消"
echo ""

# 1. 创建或获取分组
print_section "1. 获取或创建测试分组"

# 先查询分组列表，看是否已存在
print_test "查询分组: ${GROUP_NAME}"

list_url="${BASE_URL}/api/seedance/assets/v2/?Action=ListAssetGroups&Version=2024-01-01"
list_body=$(cat <<EOF
{
  "Filter": {
    "GroupType": "AIGC"
  },
  "PageNumber": 1,
  "PageSize": 100,
  "ProjectName": "default"
}
EOF
)

list_response=$(curl -s -X POST "$list_url" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$list_body")

# 调试：打印查询响应
echo ""
echo -e "${YELLOW}━━━ 查询分组响应 ━━━${NC}"
echo "$list_response" | jq '.' 2>/dev/null || echo "$list_response"
echo -e "${YELLOW}━━━━━━━━━━━━━━━${NC}"
echo ""

# 从列表中查找匹配的分组
GROUP_ID=$(echo "$list_response" | jq -r --arg name "$GROUP_NAME" '.Result.Items[]? | select(.Name == $name) | .Id')

if [ -n "$GROUP_ID" ]; then
    print_success "分组已存在，跳过创建! Group ID: ${GROUP_ID}"
    echo ""
else
    print_info "分组不存在，开始创建..."
    echo ""

    print_test "创建 AIGC 素材组: ${GROUP_NAME}"

    group_url="${BASE_URL}/api/seedance/assets/v2/?Action=CreateAssetGroup&Version=2024-01-01"
    group_body=$(cat <<EOF
{
  "Name": "${GROUP_NAME}",
  "Title": "Doubao 测试素材组",
  "Description": "用于 Doubao 官方 API 测试的素材组",
  "GroupType": "AIGC",
  "ProjectName": "default"
}
EOF
)

    print_request "POST" "$group_url" "$group_body"

    group_response=$(curl -s -X POST "$group_url" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$group_body")

    print_response "$group_response"

    GROUP_ID=$(echo "$group_response" | jq -r '.Result.Id // empty')

    if [ -z "$GROUP_ID" ]; then
        print_error "分组创建失败，跳过素材上传"
        echo ""
    else
        print_success "分组创建成功! Group ID: ${GROUP_ID}"
        echo ""
    fi
fi

# 2. 上传素材
print_section "2. 上传图片素材"

if [ -z "$GROUP_ID" ]; then
    print_info "没有 Group ID，跳过素材上传，直接使用外部图片 URL"
    IMAGE_URL="${TEST_IMAGE_URL}"
    echo ""
else
    print_test "上传测试图片到素材库"

    asset_url="${BASE_URL}/api/seedance/assets/v2/?Action=CreateAsset&Version=2024-01-01"
    asset_body=$(cat <<EOF
{
  "GroupId": "${GROUP_ID}",
  "URL": "${TEST_IMAGE_URL}",
  "AssetType": "Image",
  "Name": "test-image-$(date +%s).jpg",
  "ProjectName": "default"
}
EOF
)

    print_request "POST" "$asset_url" "$asset_body"

    asset_response=$(curl -s -X POST "$asset_url" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$asset_body")

    print_response "$asset_response"

    ASSET_ID=$(echo "$asset_response" | jq -r '.Result.Id // empty')

    if [ -z "$ASSET_ID" ]; then
        print_error "素材上传失败，使用外部图片 URL"
        IMAGE_URL="${TEST_IMAGE_URL}"
    else
        print_success "素材上传成功! Asset ID: ${ASSET_ID}"

        # 等待素材状态变为 Active
        print_test "等待素材处理完成..."
        MAX_RETRIES=30
        RETRY_COUNT=0
        ASSET_STATUS=""

        while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
            get_asset_response=$(curl -s -X POST "${BASE_URL}/api/seedance/assets/v2/?Action=GetAsset&Version=2024-01-01" \
                -H "Authorization: Bearer ${API_KEY}" \
                -H "Content-Type: application/json" \
                -d "{\"AssetId\": \"${ASSET_ID}\"}")

            ASSET_STATUS=$(echo "$get_asset_response" | jq -r '.Result.Status // empty')

            if [ "$ASSET_STATUS" = "Active" ]; then
                print_success "素材处理完成，状态: Active"
                break
            elif [ "$ASSET_STATUS" = "Failed" ]; then
                print_error "素材处理失败"
                IMAGE_URL="${TEST_IMAGE_URL}"
                break
            else
                echo "  状态: ${ASSET_STATUS:-Processing}，等待中... ($((RETRY_COUNT + 1))/$MAX_RETRIES)"
                sleep 2
            fi

            RETRY_COUNT=$((RETRY_COUNT + 1))
        done

        if [ "$ASSET_STATUS" = "Active" ]; then
            IMAGE_URL="asset://${ASSET_ID}"
        else
            print_error "素材处理超时或失败，使用外部图片 URL"
            IMAGE_URL="${TEST_IMAGE_URL}"
        fi
    fi
    echo ""
fi

# 3. 创建视频任务
print_section "3. 创建视频生成任务"

print_test "使用素材创建视频任务"

create_url="${BASE_URL}/api/v3/contents/generations/tasks"
create_body=$(cat <<EOF
{
  "model": "doubao-seedance-2-0-mini-260615",
  "content": [
    {
      "type": "text",
      "text": "女子身边一只可爱的小猫在草地上玩耍，阳光明媚"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "${IMAGE_URL}"
      },
      "role": "reference_image"
    }
  ],
  "resolution": "480p",
  "ratio": "16:9",
  "duration": 4
}
EOF
)

print_request "POST" "$create_url" "$create_body"

create_response=$(curl -s -X POST "$create_url" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$create_body")

print_response "$create_response"
validate_official_format "$create_response" "task_create"

TASK_ID=$(echo "$create_response" | jq -r '.id // empty')

if [ -z "$TASK_ID" ]; then
    print_error "任务创建失败"
    exit 1
fi

print_success "任务创建成功! Task ID: ${TASK_ID}"
echo ""

# 4. 查询任务状态（轮询）
print_section "4. 查询任务状态"

print_info "开始轮询任务状态（最多等待 60 秒）..."
echo ""

for i in {1..12}; do
    print_test "第 ${i} 次查询 (${i}0秒)"

    query_url="${BASE_URL}/api/v3/contents/generations/tasks/${TASK_ID}"

    if [ $i -eq 1 ]; then
        # 第一次查询时打印请求详情
        print_request "GET" "$query_url"
    else
        echo -e "${CYAN}━━━ 请求 ━━━${NC} GET ${query_url}"
    fi

    query_response=$(curl -s -X GET "$query_url" \
        -H "Authorization: Bearer ${API_KEY}")

    status=$(echo "$query_response" | jq -r '.status // "unknown"')

    if [ $i -eq 1 ]; then
        # 第一次查询时打印完整响应
        print_response "$query_response"
    else
        # 其他查询也打印响应体，方便调试
        echo -e "${CYAN}━━━ 响应详情 ━━━${NC}"
        echo "Body:"
        echo "$query_response" | jq '.'
        echo -e "${CYAN}━━━━━━━━━━━━━━━${NC}"
        echo ""
    fi

    echo "当前状态: ${status}"

    if [ "$status" = "succeeded" ]; then
        print_success "任务完成！"
        echo ""
        print_response "$query_response"
        validate_official_format "$query_response" "task_single"

        video_url=$(echo "$query_response" | jq -r '.content.video_url // empty')
        if [ -n "$video_url" ]; then
            echo ""
            print_success "视频 URL: ${video_url}"
        fi
        break
    elif [ "$status" = "failed" ]; then
        print_error "任务失败"
        print_response "$query_response"
        validate_official_format "$query_response" "task_single"
        break
    elif [ "$status" = "cancelled" ]; then
        print_error "任务已取消"
        print_response "$query_response"
        break
    fi

    if [ $i -lt 12 ]; then
        echo "等待 10 秒后继续..."
        sleep 10
    fi
done

echo ""

# 5. 查询任务列表
print_section "5. 查询任务列表"

print_test "查询最近的任务列表"

list_url="${BASE_URL}/api/v3/contents/generations/tasks?page_num=1&page_size=5"
print_request "GET" "$list_url"

list_response=$(curl -s -X GET "$list_url" \
    -H "Authorization: Bearer ${API_KEY}")

print_response "$list_response"
validate_official_format "$list_response" "task_list"

total=$(echo "$list_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}总任务数: ${total}${NC}"
echo ""

# 6. 测试筛选
print_section "6. 测试任务列表筛选"

print_test "按模型筛选"

filter_url="${BASE_URL}/api/v3/contents/generations/tasks?filter.model=doubao-seedance-2-0-mini-260615&page_size=5"
print_request "GET" "$filter_url"

filter_response=$(curl -s -X GET "$filter_url" \
    -H "Authorization: Bearer ${API_KEY}")

print_response "$filter_response"
validate_official_format "$filter_response" "task_list"

filter_count=$(echo "$filter_response" | jq -r '.total // 0')
print_info "该模型的任务数: ${filter_count}"
echo ""

# 7. 测试取消任务
print_section "7. 测试取消任务接口"

print_test "创建一个新任务用于测试取消"

cancel_create_url="${BASE_URL}/api/v3/contents/generations/tasks"
cancel_create_body=$(cat <<EOF
{
  "model": "doubao-seedance-2-0-mini-260615",
  "content": [
    {
      "type": "text",
      "text": "测试取消任务"
    }
  ],
  "duration": 4
}
EOF
)

print_request "POST" "$cancel_create_url" "$cancel_create_body"

cancel_test_response=$(curl -s -X POST "$cancel_create_url" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$cancel_create_body")

print_response "$cancel_test_response"

CANCEL_TASK_ID=$(echo "$cancel_test_response" | jq -r '.id // empty')

if [ -n "$CANCEL_TASK_ID" ]; then
    print_success "测试任务创建成功: ${CANCEL_TASK_ID}"
    echo ""

    print_test "立即尝试取消任务"

    cancel_url="${BASE_URL}/api/v3/contents/generations/tasks/${CANCEL_TASK_ID}"
    print_request "DELETE" "$cancel_url"

    cancel_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE "$cancel_url" \
        -H "Authorization: Bearer ${API_KEY}")

    response_body=$(echo "$cancel_response" | sed -e 's/HTTP_STATUS\:.*//g')
    http_status=$(echo "$cancel_response" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

    print_response "$response_body" "$http_status"

    case $http_status in
        200)
            print_success "取消成功 (HTTP 200)"
            ;;
        404)
            print_info "任务不存在 (HTTP 404)"
            ;;
        409)
            print_info "任务运行中，无法取消 (HTTP 409)"
            ;;
        400)
            print_info "任务状态不支持取消 (HTTP 400)"
            ;;
        *)
            print_error "未预期的状态码: ${http_status}"
            ;;
    esac
else
    print_error "无法创建测试任务"
fi

echo ""

# 8. 测试总结
print_section "测试完成"

echo -e "${GREEN}已完成以下测试:${NC}"
echo "  ✓ 素材上传"
echo "  ✓ 创建视频任务"
echo "  ✓ 查询任务状态"
echo "  ✓ 查询任务列表"
echo "  ✓ 任务列表筛选"
echo "  ✓ 取消任务接口"
echo ""
print_info "主任务 ID: ${TASK_ID}"
echo ""
print_info "查询任务: $0 --query ${TASK_ID}"
print_info "查询列表: $0 --query all"
echo ""
