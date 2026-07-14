#!/bin/bash
# ============================================================
# Seedance 2.0 计费对比测试脚本（火山方舟原生接口格式）
# 对应文档: docs/咪咕/ARK-VIDEO-API.zh-CN.md
# 接口路径:
#   创建: POST /api/v3/contents/generations/tasks
#   查询: GET  /api/v3/contents/generations/tasks/{task_id}
# ============================================================

# ---------- 配置区（按需修改）----------
NEWAPI_BASE_URL="${NEWAPI_BASE_URL:-http://book2:3000}"
NEWAPI_API_KEY="${NEWAPI_API_KEY:-sk-60oOqvQYb8vziFfg2hPHlTKW3X80Pc6sIBDC5EFHCY0sn5NY}"

UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-https://sd.dawnloadai.com:8443}"
UPSTREAM_API_KEY="${UPSTREAM_API_KEY:-}"   # 上游 new-api 令牌（留空则跳过上游日志对比）

# 视频模型
MODEL="${MODEL:-doubao-seedance-2-0-260128}"

# 请求参数（Ark 原生格式，字段在根级）
PROMPT="${PROMPT:-人物面对镜头自然介绍产品}"
DURATION="${DURATION:-5}"           # integer，不是字符串
RESOLUTION="${RESOLUTION:-720p}"
RATIO="${RATIO:-16:9}"
GENERATE_AUDIO="${GENERATE_AUDIO:-false}"
WATERMARK="${WATERMARK:-false}"

# 参考图 URL（留空则纯文生视频）
IMAGE_URL="${IMAGE_URL:-https://static.horse-world.mints-id.com/rh/20260604204600/1780577160529_9921.png}"
# 参考视频 URL（留空则不传）
VIDEO_URL="${VIDEO_URL:-}"
# 参考音频 URL（留空则不传）
AUDIO_URL="${AUDIO_URL:-}"

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
  for cmd in curl jq bc; do
    command -v "$cmd" &>/dev/null || { log_error "$cmd 未安装"; exit 1; }
  done
}

# ---------- 积分转元（500000 积分 = 1 元）----------
quota_to_yuan() {
  local q="$1"
  if [ "$q" = "null" ] || [ "$q" = "-1" ] || [ -z "$q" ]; then echo "N/A"; return; fi
  echo "scale=6; $q / 500000" | bc 2>/dev/null || echo "N/A"
}

# ---------- 构建 content 数组 ----------
build_content_array() {
  # 始终包含 text
  local content
  content=$(jq -n --arg text "$PROMPT" '[{"type":"text","text":$text}]')

  # 追加 image_url（如果配置了）
  if [ -n "$IMAGE_URL" ]; then
    content=$(echo "$content" | jq \
      --arg url "$IMAGE_URL" \
      '. + [{"type":"image_url","image_url":{"url":$url}}]')
  fi

  # 追加 video_url（如果配置了）
  if [ -n "$VIDEO_URL" ]; then
    content=$(echo "$content" | jq \
      --arg url "$VIDEO_URL" \
      '. + [{"type":"video_url","video_url":{"url":$url}}]')
  fi

  # 追加 audio_url（如果配置了）
  if [ -n "$AUDIO_URL" ]; then
    content=$(echo "$content" | jq \
      --arg url "$AUDIO_URL" \
      '. + [{"type":"audio_url","audio_url":{"url":$url}}]')
  fi

  echo "$content"
}

# ---------- 提交视频任务（Ark 原生格式）----------
submit_task() {
  local content_array
  content_array=$(build_content_array)

  local body
  body=$(jq -n \
    --arg model "$MODEL" \
    --arg resolution "$RESOLUTION" \
    --arg ratio "$RATIO" \
    --argjson duration "$DURATION" \
    --argjson generate_audio "$GENERATE_AUDIO" \
    --argjson watermark "$WATERMARK" \
    --argjson content "$content_array" \
    '{
      model: $model,
      content: $content,
      resolution: $resolution,
      ratio: $ratio,
      duration: $duration,
      generate_audio: $generate_audio,
      watermark: $watermark
    }')

  log_info "提交请求体（Ark 原生格式）:"
  echo "$body" | jq '.'
  echo ""

  local tmpfile; tmpfile=$(mktemp)
  HTTP_CODE=$(curl -s -o "$tmpfile" -w "%{http_code}" \
    -X POST "${NEWAPI_BASE_URL}/api/v3/contents/generations/tasks" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body")
  SUBMIT_BODY=$(cat "$tmpfile"); rm -f "$tmpfile"
}

# ---------- 轮询任务状态（Ark 原生查询路径）----------
poll_task() {
  local task_id="$1"
  local max_retries="${2:-40}"
  local interval="${3:-10}"
  local retry=0

  log_info "开始轮询 task_id=${task_id}，间隔 ${interval}s，最多 ${max_retries} 次"
  log_info "查询路径: GET /api/v3/contents/generations/tasks/${task_id}"

  while [ $retry -lt $max_retries ]; do
    retry=$((retry + 1))
    sleep "$interval"

    local tmpfile; tmpfile=$(mktemp)
    curl -s -o "$tmpfile" \
      -X GET "${NEWAPI_BASE_URL}/api/v3/contents/generations/tasks/${task_id}" \
      -H "Authorization: Bearer ${NEWAPI_API_KEY}"
    POLL_BODY=$(cat "$tmpfile"); rm -f "$tmpfile"

    local status progress
    status=$(echo "$POLL_BODY" | jq -r '.status // "unknown"' 2>/dev/null)
    progress=$(echo "$POLL_BODY" | jq -r '.progress // ""' 2>/dev/null)

    log_info "#${retry} 状态: ${status} ${progress}"

    case "$status" in
      "succeeded"|"completed"|"success")
        FINAL_TASK_BODY="$POLL_BODY"
        return 0
        ;;
      "failed"|"error")
        log_error "任务失败"
        echo "$POLL_BODY" | jq '.error // .'
        FINAL_TASK_BODY="$POLL_BODY"
        return 1
        ;;
    esac
  done

  log_warn "轮询超时，任务可能仍在处理"
  FINAL_TASK_BODY="$POLL_BODY"
  return 2
}

# ---------- 查询日志 ----------
query_log() {
  local base_url="$1"
  local token="$2"
  local start_ts="$3"
  local model_name="$4"

  if [ -z "$token" ] || [ -z "$base_url" ]; then
    echo ""
    return
  fi

  local end_ts
  end_ts=$(date +%s)

  curl -s \
    "${base_url}/api/log/self?type=1&model_name=${model_name}&start_timestamp=${start_ts}&end_timestamp=${end_ts}&p=1&page_size=5" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" 2>/dev/null
}

# ============================================================
# 主流程
# ============================================================

check_deps

echo ""
log_sep
echo -e "${BOLD}  Seedance 2.0 计费测试（Ark 原生接口格式）${NC}"
log_sep
echo -e "  本端 new-api:  ${NEWAPI_BASE_URL}"
[ -n "$UPSTREAM_BASE_URL" ] && echo -e "  上游 new-api:  ${UPSTREAM_BASE_URL}"
echo -e "  模型:          ${MODEL}"
echo -e "  Prompt:        ${PROMPT}"
echo -e "  时长/分辨率:   ${DURATION}s / ${RESOLUTION} / ${RATIO}"
echo -e "  音频生成:      ${GENERATE_AUDIO}  水印: ${WATERMARK}"
[ -n "$IMAGE_URL" ] && echo -e "  参考图:        ${IMAGE_URL}"
[ -n "$VIDEO_URL" ] && echo -e "  参考视频:      ${VIDEO_URL}"
[ -n "$AUDIO_URL" ] && echo -e "  参考音频:      ${AUDIO_URL}"
log_sep

# ---------- Step 1: 提交任务 ----------
log_title "Step 1 · 提交 Seedance 任务（POST /api/v3/contents/generations/tasks）"

SUBMIT_TS=$(date +%s)
submit_task

echo ""
log_info "HTTP ${HTTP_CODE}"
echo "$SUBMIT_BODY" | jq '.'

TASK_ID=$(echo "$SUBMIT_BODY" | jq -r '.id // .task_id // empty' 2>/dev/null)

if [ -z "$TASK_ID" ]; then
  log_error "未获取到 task_id，提交失败"
  exit 1
fi

log_ok "task_id = ${TASK_ID}"
log_info "提交时间戳 = ${SUBMIT_TS}"

# ---------- Step 2: 轮询等待完成 ----------
log_title "Step 2 · 轮询任务状态"

FINAL_TASK_BODY=""
poll_task "$TASK_ID" 40 10
POLL_RESULT=$?

FINISH_TS=$(date +%s)

if [ $POLL_RESULT -eq 1 ]; then
  log_error "任务失败，退出"
  exit 1
fi

echo ""
log_info "最终任务详情:"
echo "$FINAL_TASK_BODY" | jq '.'

# 视频地址在 content.video_url（Ark 原生响应格式）
VIDEO_URL_RESULT=$(echo "$FINAL_TASK_BODY" | jq -r '.content.video_url // .metadata.url // "N/A"' 2>/dev/null)

# ---------- Step 3: 查询本端日志 ----------
log_title "Step 3 · 查询本端扣费日志"

sleep 3

NEWAPI_LOG_RESP=$(query_log "$NEWAPI_BASE_URL" "Bearer $NEWAPI_API_KEY" "$SUBMIT_TS" "$MODEL")

if [ -z "$NEWAPI_LOG_RESP" ]; then
  log_warn "未配置 NEWAPI_API_KEY，跳过本端日志查询"
  NEWAPI_QUOTA="N/A"
else
  log_info "本端最近日志条目:"
  echo "$NEWAPI_LOG_RESP" | jq '.data.items[0] // .data[0] // "无日志"'
  NEWAPI_QUOTA=$(echo "$NEWAPI_LOG_RESP" | jq -r '(.data.items[0].quota // .data[0].quota // "N/A") | tostring' 2>/dev/null)
  NEWAPI_YUAN=$(quota_to_yuan "$NEWAPI_QUOTA")
  log_ok "本端实际扣费: ${NEWAPI_QUOTA} 积分 ≈ ${NEWAPI_YUAN} 元"
fi

# ---------- Step 4: 查询上游日志 ----------
UPSTREAM_QUOTA="N/A"
if [ -n "$UPSTREAM_BASE_URL" ] && [ -n "$UPSTREAM_API_KEY" ]; then
  log_title "Step 4 · 查询上游扣费日志"

  sleep 2
  UPSTREAM_LOG_RESP=$(query_log "$UPSTREAM_BASE_URL" "Bearer $UPSTREAM_API_KEY" "$SUBMIT_TS" "$MODEL")
  log_info "上游最近日志条目:"
  echo "$UPSTREAM_LOG_RESP" | jq '.data.items[0] // .data[0] // "无日志"'
  UPSTREAM_QUOTA=$(echo "$UPSTREAM_LOG_RESP" | jq -r '(.data.items[0].quota // .data[0].quota // "N/A") | tostring' 2>/dev/null)
  UPSTREAM_YUAN=$(quota_to_yuan "$UPSTREAM_QUOTA")
  log_ok "上游实际扣费: ${UPSTREAM_QUOTA} 积分 ≈ ${UPSTREAM_YUAN} 元"
fi

# ---------- Step 5: 对比报告 ----------
log_title "Step 5 · 计费对比报告"

echo ""
echo -e "  task_id:       ${TASK_ID}"
echo -e "  耗时:          $((FINISH_TS - SUBMIT_TS))s"
echo ""

[ "$NEWAPI_QUOTA" != "N/A" ] && \
  echo -e "  ${BOLD}本端扣费:${NC}  ${NEWAPI_QUOTA} 积分 ≈ ${NEWAPI_YUAN} 元"

[ "$UPSTREAM_QUOTA" != "N/A" ] && \
  echo -e "  ${BOLD}上游扣费:${NC}  ${UPSTREAM_QUOTA} 积分 ≈ ${UPSTREAM_YUAN} 元"

if [ "$NEWAPI_QUOTA" != "N/A" ] && [ "$UPSTREAM_QUOTA" != "N/A" ]; then
  DIFF=$(echo "$NEWAPI_QUOTA - $UPSTREAM_QUOTA" | bc 2>/dev/null)
  ABS_DIFF="${DIFF#-}"
  DIFF_YUAN=$(quota_to_yuan "$ABS_DIFF")
  echo ""
  if [ "$DIFF" = "0" ]; then
    echo -e "  ${GREEN}✓ 扣费一致${NC}"
  elif [ "${DIFF:0:1}" = "-" ]; then
    echo -e "  ${YELLOW}⚠ 本端少扣 ${ABS_DIFF} 积分 ≈ ${DIFF_YUAN} 元${NC}"
  else
    echo -e "  ${YELLOW}⚠ 本端多扣 ${DIFF} 积分 ≈ ${DIFF_YUAN} 元${NC}"
  fi
fi

echo ""
echo -e "  ${BOLD}[ 接口路径对比 ]${NC}"
echo -e "  本脚本 (Ark 原生): POST /api/v3/contents/generations/tasks"
echo -e "  旧脚本 (兼容接口): POST /v1/video/generations"

if [ "$VIDEO_URL_RESULT" != "N/A" ] && [ -n "$VIDEO_URL_RESULT" ]; then
  echo ""
  log_ok "视频地址 (content.video_url): ${VIDEO_URL_RESULT}"
fi

echo ""
log_sep
log_ok "测试完成"
log_sep
echo ""
