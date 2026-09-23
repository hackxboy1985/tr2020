#!/usr/bin/env bash

# Seedance Asset API 测试脚本
# 每个操作都可以单独执行，默认使用 Action 风格接口。
#
# 使用示例:
#   API_KEY=sk-xxx ./test_asset_api.sh get-asset asset-xxx
#   API_KEY=sk-xxx ./test_asset_api.sh list-assets
#   API_KEY=sk-xxx ./test_asset_api.sh upload-asset https://example.com/image.jpg [group_id]
#   API_KEY=sk-xxx ./test_asset_api.sh get-group group-xxx
#
# 可通过环境变量覆盖配置:
#   BASE_URL     默认 http://localhost:3000
#   API_KEY      必填，也可以作为 --api-key 参数传入
#   PROJECT_NAME 默认 default
#   VERSION      默认 2024-01-01

set -u

BASE_URL="${BASE_URL:-http://book2:3000}"
API_KEY="${API_KEY:-}"
PROJECT_NAME="${PROJECT_NAME:-default}"
VERSION="${VERSION:-2024-01-01}"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

usage() {
  cat <<'EOF'
Seedance Asset API 测试脚本

用法:
  API_KEY=<token> ./test_asset_api.sh <命令> [参数]

命令:
  get-asset <asset_id>                         查询资产详情
  list-assets [group_id]                       查询资产列表
  upload-asset <url> [group_id] [name]        上传/创建资产
  update-asset <asset_id> [name]               更新资产名称
  delete-asset <asset_id>                      删除资产
  get-group <group_id>                         查询资产组详情
  list-groups                                  查询资产组列表
  create-group [name] [title]                  创建资产组
  update-group <group_id> [name]               更新资产组
  delete-group <group_id>                      删除资产组
  help                                         显示帮助

环境变量:
  BASE_URL=http://localhost:3000
  API_KEY=sk-xxx
  PROJECT_NAME=default
  VERSION=2024-01-01

说明:
  每次请求都会打印请求 URL、请求报文和返回报文。
  参数中的特殊字符会通过 jq 安全编码，不要手工拼接 JSON。
EOF
}

print_json() {
  if command -v jq >/dev/null 2>&1; then
    jq '.' 2>/dev/null <<<"$1" || printf '%s\n' "$1"
  else
    printf '%s\n' "$1"
  fi
}

require_arg() {
  if [ -z "${2:-}" ]; then
    echo -e "${RED}错误: $1 缺少参数${NC}" >&2
    usage
    exit 1
  fi
}

request() {
  local action="$1"
  local body="$2"
  local url="${BASE_URL%/}/api/seedance/assets/v2/?Action=${action}&Version=${VERSION}"
  local response

  echo -e "${CYAN}===== ${action} =====${NC}"
  echo "请求 URL: POST ${url}"
  echo "请求报文:"
  print_json "$body"
  echo "返回报文:"
  response=$(curl -sS -X POST "$url" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body")
  print_json "$response"
  echo -e "${CYAN}====================${NC}"
}

if [ "$#" -eq 0 ]; then
  usage
  exit 1
fi

# 支持 API_KEY 作为全局参数传入: --api-key <token> <command> ...
if [ "${1:-}" = "--api-key" ]; then
  require_arg "--api-key" "${2:-}"
  API_KEY="$2"
  shift 2
fi

if [ -z "$API_KEY" ]; then
  echo -e "${RED}错误: 未设置 API_KEY${NC}" >&2
  echo "请使用 API_KEY=sk-xxx 或 --api-key sk-xxx" >&2
  exit 1
fi

command="${1:-help}"
shift || true

case "$command" in
  get-asset)
    require_arg "get-asset" "${1:-}"
    body=$(jq -n --arg id "$1" --arg project "$PROJECT_NAME" \
      '{Id: $id, ProjectName: $project}')
    request "GetAsset" "$body"
    ;;
  list-assets)
    group_id="${1:-}"
    if [ -n "$group_id" ]; then
      body=$(jq -n --arg group "$group_id" --arg project "$PROJECT_NAME" \
        '{Filter: {GroupIds: [$group]}, PageNumber: 1, PageSize: 10, ProjectName: $project}')
    else
      body=$(jq -n --arg project "$PROJECT_NAME" \
        '{PageNumber: 1, PageSize: 10, ProjectName: $project}')
    fi
    request "ListAssets" "$body"
    ;;
  upload-asset)
    require_arg "upload-asset" "${1:-}"
    asset_url="$1"
    group_id="${2:-}"
    asset_name="${3:-测试图片}"
    body=$(jq -n --arg url "$asset_url" --arg group "$group_id" \
      --arg name "$asset_name" --arg project "$PROJECT_NAME" \
      '{URL: $url, AssetType: "Image", Name: $name, ProjectName: $project} | if $group == "" then . else . + {GroupId: $group} end')
    request "CreateAsset" "$body"
    ;;
  update-asset)
    require_arg "update-asset" "${1:-}"
    asset_name="${2:-更新后的图片}"
    body=$(jq -n --arg id "$1" --arg name "$asset_name" --arg project "$PROJECT_NAME" \
      '{Id: $id, Name: $name, ProjectName: $project}')
    request "UpdateAsset" "$body"
    ;;
  delete-asset)
    require_arg "delete-asset" "${1:-}"
    body=$(jq -n --arg id "$1" --arg project "$PROJECT_NAME" \
      '{Id: $id, ProjectName: $project}')
    request "DeleteAsset" "$body"
    ;;
  get-group)
    require_arg "get-group" "${1:-}"
    body=$(jq -n --arg id "$1" --arg project "$PROJECT_NAME" \
      '{Id: $id, ProjectName: $project}')
    request "GetAssetGroup" "$body"
    ;;
  list-groups)
    body=$(jq -n --arg project "$PROJECT_NAME" \
      '{Filter: {GroupType: "AIGC"}, PageNumber: 1, PageSize: 10, ProjectName: $project}')
    request "ListAssetGroups" "$body"
    ;;
  create-group)
    name="${1:-test_group_001}"
    title="${2:-测试素材组}"
    body=$(jq -n --arg name "$name" --arg title "$title" --arg project "$PROJECT_NAME" \
      '{Name: $name, Title: $title, Description: "用于 API 测试的素材组", GroupType: "AIGC", ProjectName: $project}')
    request "CreateAssetGroup" "$body"
    ;;
  update-group)
    require_arg "update-group" "${1:-}"
    group_name="${2:-test_group_001_updated}"
    body=$(jq -n --arg id "$1" --arg name "$group_name" --arg project "$PROJECT_NAME" \
      '{Id: $id, Name: $name, Description: "更新后的描述", ProjectName: $project}')
    request "UpdateAssetGroup" "$body"
    ;;
  delete-group)
    require_arg "delete-group" "${1:-}"
    body=$(jq -n --arg id "$1" --arg project "$PROJECT_NAME" \
      '{Id: $id, ProjectName: $project}')
    request "DeleteAssetGroup" "$body"
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    echo -e "${RED}错误: 未知命令: ${command}${NC}" >&2
    usage
    exit 1
    ;;
esac
