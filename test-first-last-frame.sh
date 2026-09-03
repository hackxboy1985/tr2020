#!/bin/bash
# ============================================================
# Seedance 首尾帧模式测试脚本
# 测试新旧两个接口对首尾帧的支持
#
# 用法:
#   ./test-first-last-frame.sh              # 测试两个接口
#   ./test-first-last-frame.sh --old        # 仅测试旧接口
#   ./test-first-last-frame.sh --new        # 仅测试新接口
# ============================================================

# ---------- 配置区 ----------
NEWAPI_BASE_URL="${NEWAPI_BASE_URL:-http://book2:3002}"
NEWAPI_API_KEY="${NEWAPI_API_KEY:-sk-60oOqvQYb8vziFfg2hPHlTKW3X80Pc6sIBDC5EFHCY0sn5NY}"

MODEL="${MODEL:-doubao-seedance-2-0-fast-260128}"
PROMPT="${PROMPT:-一个人在海边奔跑}"
DURATION="${DURATION:-5}"
RESOLUTION="${RESOLUTION:-720p}"
RATIO="${RATIO:-16:9}"

# 首尾帧图片 URL
FIRST_FRAME_URL="${FIRST_FRAME_URL:-https://static.horse-world.mints-id.com/rh/20260604204600/1780577160529_9921.png}"
LAST_FRAME_URL="${LAST_FRAME_URL:-https://static.horse-world.mints-id.com/general/1/image/2026-06-18/1781744040935_290.png}"

# 解析命令行参数
TEST_OLD=true
TEST_NEW=true
while [[ $# -gt 0 ]]; do
  case $1 in
    --old)
      TEST_OLD=true
      TEST_NEW=false
      shift
      ;;
    --new)
      TEST_OLD=false
      TEST_NEW=true
      shift
      ;;
    *)
      echo "未知参数: $1"
      echo "用法: $0 [--old|--new]"
      exit 1
      ;;
  esac
done

# ---------- 颜色 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_sep()   { echo -e "${CYAN}============================================================${NC}"; }
log_title() { echo -e "\n${BOLD}${CYAN}$*${NC}"; log_sep; }

check_deps() {
  for cmd in curl jq; do
    command -v "$cmd" &>/dev/null || { log_error "$cmd 未安装"; exit 1; }
  done
}

# ---------- 提交任务（旧接口 /v1/video/generations）----------
submit_old_api() {
  log_title "旧接口：POST /v1/video/generations（OpenAI 格式 + Ark content）"

  local body
  body=$(jq -n \
    --arg model "$MODEL" \
    --arg prompt "$PROMPT" \
    --argjson duration "$DURATION" \
    --arg resolution "$RESOLUTION" \
    --arg ratio "$RATIO" \
    --arg first_frame "$FIRST_FRAME_URL" \
    --arg last_frame "$LAST_FRAME_URL" \
    '{
      model: $model,
      content: [
        {type: "text", text: $prompt},
        {type: "image_url", image_url: {url: $first_frame}, role: "first_frame"},
        {type: "image_url", image_url: {url: $last_frame}, role: "last_frame"}
      ],
      duration: $duration,
      resolution: $resolution,
      ratio: $ratio
    }')

  log_info "请求体:"
  echo "$body" | jq '.'
  echo ""

  local tmpfile; tmpfile=$(mktemp)
  local http_code
  http_code=$(curl -s -o "$tmpfile" -w "%{http_code}" \
    -X POST "${NEWAPI_BASE_URL}/v1/video/generations" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body")

  local response; response=$(cat "$tmpfile"); rm -f "$tmpfile"

  log_info "HTTP ${http_code}"
  echo "$response" | jq '.'
  echo ""

  if [ "$http_code" != "200" ]; then
    log_error "提交失败"
    return 1
  fi

  OLD_TASK_ID=$(echo "$response" | jq -r '.id // .task_id // empty' 2>/dev/null)

  if [ -z "$OLD_TASK_ID" ]; then
    log_error "未获取到 task_id"
    return 1
  fi

  log_ok "task_id = ${OLD_TASK_ID}"
  return 0
}

# ---------- 提交任务（新接口 /api/v3/contents/generations/tasks）----------
submit_new_api() {
  log_title "新接口：POST /api/v3/contents/generations/tasks（Doubao 官方格式）"

  local body
  body=$(jq -n \
    --arg model "$MODEL" \
    --arg prompt "$PROMPT" \
    --argjson duration "$DURATION" \
    --arg resolution "$RESOLUTION" \
    --arg ratio "$RATIO" \
    --arg first_frame "$FIRST_FRAME_URL" \
    --arg last_frame "$LAST_FRAME_URL" \
    '{
      model: $model,
      content: [
        {type: "text", text: $prompt},
        {type: "image_url", image_url: {url: $first_frame}, role: "first_frame"},
        {type: "image_url", image_url: {url: $last_frame}, role: "last_frame"}
      ],
      duration: $duration,
      resolution: $resolution,
      ratio: $ratio
    }')

  log_info "请求体:"
  echo "$body" | jq '.'
  echo ""

  local tmpfile; tmpfile=$(mktemp)
  local http_code
  http_code=$(curl -s -o "$tmpfile" -w "%{http_code}" \
    -X POST "${NEWAPI_BASE_URL}/api/v3/contents/generations/tasks" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body")

  local response; response=$(cat "$tmpfile"); rm -f "$tmpfile"

  log_info "HTTP ${http_code}"
  echo "$response" | jq '.'
  echo ""

  if [ "$http_code" != "200" ]; then
    log_error "提交失败"
    return 1
  fi

  NEW_TASK_ID=$(echo "$response" | jq -r '.id // empty' 2>/dev/null)

  if [ -z "$NEW_TASK_ID" ]; then
    log_error "未获取到 task_id"
    return 1
  fi

  log_ok "task_id = ${NEW_TASK_ID}"
  return 0
}

# ---------- 轮询任务状态 ----------
poll_task() {
  local task_id="$1"
  local api_path="$2"
  local max_retries="${3:-40}"
  local interval="${4:-10}"
  local retry=0

  log_info "开始轮询 task_id=${task_id}，API=${api_path}，间隔 ${interval}s"

  while [ $retry -lt $max_retries ]; do
    retry=$((retry + 1))
    sleep "$interval"

    local tmpfile; tmpfile=$(mktemp)
    curl -s -o "$tmpfile" \
      -X GET "${NEWAPI_BASE_URL}${api_path}/${task_id}" \
      -H "Authorization: Bearer ${NEWAPI_API_KEY}"
    local response; response=$(cat "$tmpfile"); rm -f "$tmpfile"

    local status
    status=$(echo "$response" | jq -r '.status // "unknown"' 2>/dev/null)

    log_info "#${retry} 状态: ${status}"

    case "$status" in
      "succeeded"|"completed"|"success")
        log_ok "任务完成"
        echo "$response" | jq '.'

        local video_url
        video_url=$(echo "$response" | jq -r '.content.video_url // .video_url // .url // .metadata.url // "N/A"' 2>/dev/null)

        if [ "$video_url" != "N/A" ] && [ -n "$video_url" ]; then
          log_ok "视频地址: ${video_url}"
        fi

        return 0
        ;;
      "failed"|"error")
        log_error "任务失败"
        echo "$response" | jq '.error // .'
        return 1
        ;;
    esac
  done

  log_warn "轮询超时"
  return 2
}

# ============================================================
# 主流程
# ============================================================

check_deps

echo ""
log_sep
echo -e "${BOLD}  Seedance 首尾帧模式测试${NC}"
log_sep
echo -e "  Base URL:      ${NEWAPI_BASE_URL}"
echo -e "  模型:          ${MODEL}"
echo -e "  Prompt:        ${PROMPT}"
echo -e "  时长/分辨率:   ${DURATION}s / ${RESOLUTION} / ${RATIO}"
echo -e "  首帧图片:      ${FIRST_FRAME_URL}"
echo -e "  尾帧图片:      ${LAST_FRAME_URL}"
log_sep

OLD_TASK_ID=""
NEW_TASK_ID=""

# ---------- 测试旧接口 ----------
if [ "$TEST_OLD" = true ]; then
  if submit_old_api; then
    log_title "旧接口 · 轮询任务状态"
    poll_task "$OLD_TASK_ID" "/v1/videos" 40 10
  else
    log_error "旧接口提交失败"
  fi
fi

# ---------- 测试新接口 ----------
if [ "$TEST_NEW" = true ]; then
  if submit_new_api; then
    log_title "新接口 · 轮询任务状态"
    poll_task "$NEW_TASK_ID" "/api/v3/contents/generations/tasks" 40 10
  else
    log_error "新接口提交失败"
  fi
fi

# ---------- 总结 ----------
log_title "测试总结"

echo ""
if [ -n "$OLD_TASK_ID" ]; then
  echo -e "  旧接口 task_id: ${OLD_TASK_ID}"
fi

if [ -n "$NEW_TASK_ID" ]; then
  echo -e "  新接口 task_id: ${NEW_TASK_ID}"
fi

echo ""
log_sep
log_ok "测试完成"
log_sep
echo ""
