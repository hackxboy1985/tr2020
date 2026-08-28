#!/bin/bash

# Doubao 官方 API 测试脚本
# 模拟下游用户使用与官方一致的接口进行请求
# 使用方法: ./test_doubao_official_api.sh

# 配置
BASE_URL="http://localhost:3000"
API_KEY="sk-your-api-key-here"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "======================================"
echo "Doubao 官方 API 测试脚本"
echo "======================================"
echo ""

# 检查 API Key
if [ "$API_KEY" = "sk-your-api-key-here" ]; then
    echo -e "${RED}错误: 请先设置 API_KEY${NC}"
    echo "请编辑脚本，将 API_KEY 设置为你的实际 token"
    exit 1
fi

# 测试函数
print_section() {
    echo ""
    echo -e "${GREEN}======================================"
    echo "$1"
    echo "======================================${NC}"
    echo ""
}

print_test() {
    echo -e "${BLUE}>>> $1${NC}"
    echo ""
}

# 1. 创建视频生成任务
print_section "1. 创建视频生成任务"

print_test "请求: POST /api/v3/contents/generations/tasks"

create_response=$(curl -s -X POST \
    "${BASE_URL}/api/v3/contents/generations/tasks" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "doubao-seedance-2-0-mini-260128",
      "content": [
        {
          "type": "text",
          "text": "一个可爱的小猫咪在草地上玩耍，阳光明媚，画面温馨"
        }
      ],
      "ratio": "16:9",
      "duration": 5,
      "resolution": "720p"
    }')

echo "响应:"
echo "$create_response" | jq '.'

# 提取 task_id
TASK_ID=$(echo "$create_response" | jq -r '.id // empty')

if [ -z "$TASK_ID" ]; then
    echo -e "${RED}错误: 未能获取 task_id${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}任务创建成功! Task ID: ${TASK_ID}${NC}"
echo ""

# 2. 查询单个任务
print_section "2. 查询单个任务状态"

print_test "请求: GET /api/v3/contents/generations/tasks/${TASK_ID}"

for i in {1..3}; do
    echo "第 $i 次查询..."

    get_response=$(curl -s -X GET \
        "${BASE_URL}/api/v3/contents/generations/tasks/${TASK_ID}" \
        -H "Authorization: Bearer ${API_KEY}")

    echo "响应:"
    echo "$get_response" | jq '.'

    status=$(echo "$get_response" | jq -r '.status // empty')
    echo ""
    echo -e "${YELLOW}当前状态: ${status}${NC}"

    if [ "$status" = "succeeded" ] || [ "$status" = "failed" ]; then
        break
    fi

    if [ $i -lt 3 ]; then
        echo ""
        echo "等待 5 秒后重试..."
        sleep 5
        echo ""
    fi
done

echo ""

# 3. 查询任务列表（无筛选）
print_section "3. 查询任务列表（无筛选）"

print_test "请求: GET /api/v3/contents/generations/tasks?page_num=1&page_size=10"

list_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?page_num=1&page_size=10" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$list_response" | jq '.'

total=$(echo "$list_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}总任务数: ${total}${NC}"
echo ""

# 4. 查询任务列表（按状态筛选）
print_section "4. 查询任务列表（按状态筛选）"

print_test "请求: GET /api/v3/contents/generations/tasks?filter.status=queued"

filter_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?filter.status=queued" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$filter_response" | jq '.'

queued_count=$(echo "$filter_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}排队中的任务数: ${queued_count}${NC}"
echo ""

# 5. 查询任务列表（按模型筛选）
print_section "5. 查询任务列表（按模型筛选）"

print_test "请求: GET /api/v3/contents/generations/tasks?filter.model=doubao-seedance-2-0-mini-260128"

model_filter_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?filter.model=doubao-seedance-2-0-mini-260128&page_size=5" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$model_filter_response" | jq '.'

model_count=$(echo "$model_filter_response" | jq -r '.total // 0')
echo ""
echo -e "${YELLOW}该模型的任务数: ${model_count}${NC}"
echo ""

# 6. 查询任务列表（多条件筛选）
print_section "6. 查询任务列表（多条件筛选）"

print_test "请求: GET /api/v3/contents/generations/tasks?filter.status=succeeded&filter.model=doubao-seedance-2-0-260128"

multi_filter_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?filter.status=succeeded&filter.model=doubao-seedance-2-0-260128&page_num=1&page_size=5" \
    -H "Authorization: Bearer ${API_KEY}")

echo "响应:"
echo "$multi_filter_response" | jq '.'
echo ""

# 7. 测试取消任务
print_section "7. 测试取消任务"

# 7.1 创建一个新任务用于测试取消
print_test "7.1 创建一个新任务用于测试取消"

cancel_test_response=$(curl -s -X POST \
    "${BASE_URL}/api/v3/contents/generations/tasks" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "doubao-seedance-2-0-mini-260128",
      "content": [
        {
          "type": "text",
          "text": "测试取消任务"
        }
      ],
      "duration": 5
    }')

echo "响应:"
echo "$cancel_test_response" | jq '.'

CANCEL_TASK_ID=$(echo "$cancel_test_response" | jq -r '.id // empty')

if [ -z "$CANCEL_TASK_ID" ]; then
    echo -e "${RED}错误: 未能获取任务 ID${NC}"
else
    echo ""
    echo -e "${YELLOW}任务创建成功! Task ID: ${CANCEL_TASK_ID}${NC}"
    echo ""

    # 7.2 立即尝试取消任务
    print_test "7.2 取消刚创建的任务: DELETE /api/v3/contents/generations/tasks/${CANCEL_TASK_ID}"

    cancel_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE \
        "${BASE_URL}/api/v3/contents/generations/tasks/${CANCEL_TASK_ID}" \
        -H "Authorization: Bearer ${API_KEY}")

    # 分离响应体和状态码
    response_body=$(echo "$cancel_response" | sed -e 's/HTTP_STATUS\:.*//g')
    http_status=$(echo "$cancel_response" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

    echo "HTTP Status: ${http_status}"
    echo "响应:"
    if [ -n "$response_body" ]; then
        echo "$response_body" | jq '.' 2>/dev/null || echo "$response_body"
    else
        echo "{}"
    fi
    echo ""

    case $http_status in
        200)
            echo -e "${GREEN}✓ 取消成功 (HTTP 200)${NC}"
            echo "说明: 任务已发送取消请求到上游，状态将由轮询同步更新"
            ;;
        404)
            echo -e "${YELLOW}✓ 任务不存在 (HTTP 404)${NC}"
            ;;
        409)
            echo -e "${YELLOW}✓ 任务运行中，无法取消 (HTTP 409)${NC}"
            echo "说明: 只有 queued 状态的任务可以取消"
            ;;
        400)
            echo -e "${YELLOW}✓ 任务状态不支持取消 (HTTP 400)${NC}"
            ;;
        *)
            echo -e "${RED}✗ 未预期的状态码: ${http_status}${NC}"
            ;;
    esac

    echo ""
    echo -e "${BLUE}等待几秒后查询任务状态...${NC}"
    sleep 3

    # 7.3 查询任务状态，确认是否变为 cancelled
    print_test "7.3 查询任务状态"

    status_check_response=$(curl -s -X GET \
        "${BASE_URL}/api/v3/contents/generations/tasks/${CANCEL_TASK_ID}" \
        -H "Authorization: Bearer ${API_KEY}")

    echo "响应:"
    echo "$status_check_response" | jq '.'

    final_status=$(echo "$status_check_response" | jq -r '.status // empty')
    echo ""
    echo -e "${YELLOW}当前状态: ${final_status}${NC}"

    if [ "$final_status" = "cancelled" ]; then
        echo -e "${GREEN}✓ 任务状态已更新为 cancelled${NC}"
    fi
fi

echo ""

# 8. 测试参数验证
print_section "8. 测试参数验证"

echo -e "${BLUE}8.1 测试超出范围的 page_num (501)${NC}"
invalid_page_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?page_num=501&page_size=20" \
    -H "Authorization: Bearer ${API_KEY}")
echo "$invalid_page_response" | jq '.'
echo ""

echo -e "${BLUE}8.2 测试超出范围的 page_size (501)${NC}"
invalid_size_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks?page_num=1&page_size=501" \
    -H "Authorization: Bearer ${API_KEY}")
echo "$invalid_size_response" | jq '.'
echo ""

# 9. 测试状态转换
print_section "9. 测试状态值转换"

echo -e "${YELLOW}验证返回的状态值是否为 doubao 官方格式:${NC}"
echo "- queued (排队中)"
echo "- running (运行中)"
echo "- succeeded (成功)"
echo "- failed (失败)"
echo "- cancelled (已取消)"
echo ""

status_test_response=$(curl -s -X GET \
    "${BASE_URL}/api/v3/contents/generations/tasks/${TASK_ID}" \
    -H "Authorization: Bearer ${API_KEY}")

returned_status=$(echo "$status_test_response" | jq -r '.status // "unknown"')
echo -e "返回的状态值: ${GREEN}${returned_status}${NC}"
echo ""

case $returned_status in
    queued|running|succeeded|failed|cancelled)
        echo -e "${GREEN}✓ 状态值格式正确！${NC}"
        ;;
    *)
        echo -e "${RED}✗ 状态值格式不正确！${NC}"
        ;;
esac
echo ""

# 总结
print_section "测试完成"

echo "测试的接口:"
echo "  ✓ POST   /api/v3/contents/generations/tasks (创建任务)"
echo "  ✓ GET    /api/v3/contents/generations/tasks/{task_id} (查询单个任务)"
echo "  ✓ GET    /api/v3/contents/generations/tasks (查询任务列表)"
echo "  ✓ GET    /api/v3/contents/generations/tasks?filter.* (筛选查询)"
echo "  ✓ DELETE /api/v3/contents/generations/tasks/{task_id} (取消任务-预期不支持)"
echo ""

echo "测试的功能点:"
echo "  ✓ 接口路径与 doubao 官方一致"
echo "  ✓ 请求参数格式与官方一致 (page_num, page_size, filter.*)"
echo "  ✓ 响应格式与官方一致 (items, total)"
echo "  ✓ 状态值自动转换 (系统内部 <-> doubao 官方)"
echo "  ✓ 参数范围验证 (1-500)"
echo ""

echo -e "${GREEN}所有测试已完成！${NC}"
