#!/bin/bash
# ============================================================
# 视频生成 API 测试脚本
# 端点: POST /api/video-generation/create (TokenOrUserAuth)
# 设计: docs/video-generation-design.md
# ============================================================
set -e

# ---------- 默认配置 ----------
DEFAULT_BASE_URL="http://open.mints-id.com"
DEFAULT_API_KEY="sk-ttNgtYmXn03HgbOPAwaqV5FFaPgOvMwWo9ijlRCc3C1JYiNq"

# 请求参数默认值
PRODUCT_NAME="小米24寸电脑显示器"
BRAND="小米"
TAGLINE=""
SELLING_POINTS=""
PROMPT="请生成一段高端tvb广告"
RESOLUTION="720p"
DURATION=15
WHSTR="9:16"
VTYPE="产品展示"
VTYPE_ADD="搞笑"
PLATFORM="抖音"
REGION="国内电商"
LANGUAGE="简体中文"
VIDEO_MODEL="alpha-flash"
MEDIA_URL="https://aikeep-1416285001.cos.ap-guangzhou.myqcloud.com/ai-tools-upload/1783404917781-1icoz8-20260707-md5"
ROLE_URL="https://aikeep-1416285001.cos.ap-guangzhou.myqcloud.com/ai-tools-upload/20260630/015f95c8fc716d8ee106f2dd212d01c6.jpg"

# 运行时变量
BASE_URL="${BASE_URL:-}"
API_KEY="${API_KEY:-}"

# ---------- 颜色 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_sep()   { echo -e "${CYAN}============================================================${NC}"; }

check_deps() {
  if ! command -v curl &>/dev/null; then log_error "curl 未安装"; exit 1; fi
  if command -v jq &>/dev/null; then HAS_JQ=true; else HAS_JQ=false; fi
}

fmt() { if $HAS_JQ; then jq '.' 2>/dev/null || cat; else cat; fi; return 0; }

MODE="${1:-run}"
PROJECT_ID_ARG="${2:-}"

# ---------- 交互 ----------
prompt_base_url() {
  if [ -n "$BASE_URL" ]; then echo; log_info "环境变量 BASE_URL: ${BASE_URL}"; return; fi
  echo ""
  echo -e "${CYAN}请输入服务地址（直接回车: ${DEFAULT_BASE_URL}）:${NC}"
  read -r u
  BASE_URL="${u:-$DEFAULT_BASE_URL}"
  log_info "服务地址: ${BASE_URL}"
}

prompt_api_key() {
  if [ -n "$API_KEY" ]; then log_info "环境变量 API_KEY 已设置"; return; fi
  echo ""
  echo -e "${CYAN}请输入 API Key（直接回车: 默认密钥）:${NC}"
  read -r k
  API_KEY="${k:-$DEFAULT_API_KEY}"
  log_info "API Key: ${API_KEY:0:10}...${API_KEY: -6}"
}

resolve_defaults() {
  BASE_URL="${BASE_URL:-$DEFAULT_BASE_URL}"
  API_KEY="${API_KEY:-$DEFAULT_API_KEY}"
}

# ---------- 请求 JSON ----------
build_request_json() {
  cat <<JSON
{
  "product_name": "${PRODUCT_NAME}",
  "brand": "${BRAND}",
  "tagline": "${TAGLINE}",
  "selling_points": "${SELLING_POINTS}",
  "prompt": "${PROMPT}",
  "resolution": "${RESOLUTION}",
  "duration": ${DURATION},
  "whstr": "${WHSTR}",
  "vtype": "${VTYPE}",
  "vtype_add": "${VTYPE_ADD}",
  "platform": "${PLATFORM}",
  "region": "${REGION}",
  "language": "${LANGUAGE}",
  "video_model": "${VIDEO_MODEL}",
  "mediaList": [
    {
      "mediaType": "PRODUCT",
      "mediaUrl": "${MEDIA_URL}",
      "sortOrder": 1
    },
    {
      "mediaType": "ROLE",
      "mediaUrl": "${ROLE_URL}",
      "sortOrder": 2
    }
  ]
}
JSON
}

# ---------- API 调用 ----------
call_api() {
  local method="$1" path="$2" data="$3"

  local tmpfile
  tmpfile=$(mktemp)
  if [ -n "$data" ]; then
    HTTP_CODE=$(curl -s -o "$tmpfile" -w "%{http_code}" -X "$method" "${BASE_URL}${path}" \
      -H "Authorization: Bearer ${API_KEY}" \
      -H 'Content-Type: application/json' \
      -d "$data")
  else
    HTTP_CODE=$(curl -s -o "$tmpfile" -w "%{http_code}" -X "$method" "${BASE_URL}${path}" \
      -H "Authorization: Bearer ${API_KEY}")
  fi
  BODY=$(cat "$tmpfile")
  rm -f "$tmpfile"
  echo "$BODY" | fmt
  echo ""
  log_info "HTTP ${HTTP_CODE}"
}

# ---------- list ----------
do_list() {
  resolve_defaults
  echo ""
  log_sep
  log_info "当前配置参数"
  log_sep
  echo ""
  echo -e "  ${CYAN}服务地址${NC}       ${BASE_URL}"
  echo -e "  ${CYAN}接口路径${NC}       POST ${BASE_URL}/api/video-generation/create"
  echo -e "  ${CYAN}鉴权方式${NC}       Bearer Token (或 Session Cookie)"
  echo ""
  echo -e "  ${CYAN}product_name${NC}   ${PRODUCT_NAME}"
  echo -e "  ${CYAN}brand${NC}          ${BRAND}"
  echo -e "  ${CYAN}tagline${NC}        ${TAGLINE}"
  echo -e "  ${CYAN}selling_points${NC} ${SELLING_POINTS}"
  echo -e "  ${CYAN}prompt${NC}         ${PROMPT}"
  echo -e "  ${CYAN}resolution${NC}     ${RESOLUTION}"
  echo -e "  ${CYAN}duration${NC}       ${DURATION}"
  echo -e "  ${CYAN}whstr${NC}          ${WHSTR}"
  echo -e "  ${CYAN}vtype${NC}          ${VTYPE}"
  echo -e "  ${CYAN}vtype_add${NC}      ${VTYPE_ADD}"
  echo -e "  ${CYAN}platform${NC}       ${PLATFORM}"
  echo -e "  ${CYAN}region${NC}         ${REGION}"
  echo -e "  ${CYAN}language${NC}       ${LANGUAGE}"
  echo -e "  ${CYAN}video_model${NC}    ${VIDEO_MODEL}"
  echo -e "  ${CYAN}mediaUrl${NC}       ${MEDIA_URL}"
  echo -e "  ${CYAN}roleUrl${NC}        ${ROLE_URL}"
  echo ""
  log_sep
  log_info "完整请求 JSON"
  log_sep
  echo ""
  build_request_json | fmt
  echo ""
  echo -e "${CYAN}cURL 命令:${NC}"
  echo "  curl -s -X POST '${BASE_URL}/api/video-generation/create' \\"
  echo "    -H 'Authorization: Bearer ${API_KEY}' \\"
  echo "    -H 'Content-Type: application/json' \\"
  echo "    -d '\$(上面 JSON 内容)' | jq"
  echo ""
}

# ---------- query ----------
do_query() {
  check_deps
  resolve_defaults

  local pid="${PROJECT_ID_ARG:-}"
  if [ -z "$pid" ]; then
    echo ""
    echo -e "${CYAN}请输入 project_id:${NC}"
    read -r pid
  fi

  if [ -z "$pid" ]; then
    log_error "project_id 不能为空"
    exit 1
  fi

  log_sep
  log_info "查询视频项目 project_id=${pid}"
  log_sep

  call_api "GET" "/api/video-generation/projects/${pid}"

  STATUS=$(echo "$BODY" | jq -r '.data.status // "unknown"' 2>/dev/null)
  VIDEO_URL=$(echo "$BODY" | jq -r '.data.first_video_url // empty' 2>/dev/null)
  ERROR_MSG=$(echo "$BODY" | jq -r '.data.error_msg // empty' 2>/dev/null)

  case "$STATUS" in
    "SUCCESS") log_ok "已完成"; [ -n "$VIDEO_URL" ] && log_ok "视频地址: ${VIDEO_URL}" ;;
    "FAILED")  log_error "生成失败: ${ERROR_MSG}" ;;
    *)         log_info "当前状态: ${STATUS}" ;;
  esac
}

# ---------- run ----------
do_run() {
  check_deps
  prompt_base_url
  prompt_api_key

  log_sep
  log_info "视频生成 API 测试"
  log_info "服务: ${BASE_URL}"
  log_sep

  # ---------- 1. 创建项目 ----------
  echo ""
  log_sep
  log_info "【1】创建视频项目"
  log_sep

  GENERATE_BODY=$(build_request_json)
  echo "$GENERATE_BODY" | fmt
  echo ""

  call_api "POST" "/api/video-generation/create" "$GENERATE_BODY"

  PROJECT_ID=$(echo "$BODY" | jq -r '.data.project_id // empty' 2>/dev/null)
  PROJECT_STATUS=$(echo "$BODY" | jq -r '.data.status // "unknown"' 2>/dev/null)

  if [ -z "$PROJECT_ID" ]; then
    log_error "未获取到 project_id"
    exit 1
  fi

  log_ok "项目创建成功! project_id = ${PROJECT_ID}, status = ${PROJECT_STATUS}"

  # ---------- 2. 轮询查询 ----------
  echo ""
  log_sep
  log_info "【2】轮询查询项目状态"
  log_sep

  MAX_RETRIES=10
  RETRY_INTERVAL=10
  retry=0

  while [ $retry -lt $MAX_RETRIES ]; do
    retry=$((retry + 1))
    echo ""
    log_info "查询 #${retry} (project_id=${PROJECT_ID})..."

    call_api "GET" "/api/video-generation/projects/${PROJECT_ID}"

    STATUS=$(echo "$BODY" | jq -r '.data.status // "unknown"' 2>/dev/null)
    VIDEO_URL=$(echo "$BODY" | jq -r '.data.first_video_url // empty' 2>/dev/null)
    ERROR_MSG=$(echo "$BODY" | jq -r '.data.error_msg // empty' 2>/dev/null)

    case "$STATUS" in
      "ONE_CLICK_GENERATED"|"COMPLETED"|"SUCCESS")
        log_ok "视频生成完成!"
        if [ -n "$VIDEO_URL" ]; then log_ok "视频地址: ${VIDEO_URL}"; fi
        break
        ;;
      "FAILED")
        log_error "生成失败: ${ERROR_MSG}"
        break
        ;;
      "CREATED"|"COZE_RUNNING"|"VIDEO_PROCESSING"|"VIDEO_PREPARING"|"VIDEO_CONCAT"|"PROCESSING")
        log_info "状态: ${STATUS}，${RETRY_INTERVAL}s 后重试..."
        sleep $RETRY_INTERVAL
        ;;
      *)
        log_warn "状态: ${STATUS}，${RETRY_INTERVAL}s 后重试..."
        sleep $RETRY_INTERVAL
        ;;
    esac
  done

  if [ $retry -ge $MAX_RETRIES ] && [ "$STATUS" != "ONE_CLICK_GENERATED" ] && [ "$STATUS" != "COMPLETED" ]; then
    log_warn "达到最大重试次数，任务可能仍在处理中"
    echo ""
    echo "手动查询:"
    echo "  curl -s -H 'Authorization: Bearer ${API_KEY}' '${BASE_URL}/api/video-generation/projects/${PROJECT_ID}' | jq"
  fi

  echo ""
  log_sep
  log_info "测试完成"
  log_sep
}

# ============================================================
case "$MODE" in
  list|--list|-l)   do_list  ;;
  query|--query|-q) do_query ;;
  *)                do_run   ;;
esac
