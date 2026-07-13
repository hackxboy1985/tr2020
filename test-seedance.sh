#!/bin/bash
# ============================================================
# Seedance 视频生成计费对比测试脚本
# 对比本端 new-api 扣费日志 vs 上游 new-api 扣费日志
# ============================================================

# ---------- 配置区（按需修改）----------
NEWAPI_BASE_URL="${NEWAPI_BASE_URL:-http://book2:3000}"
NEWAPI_API_KEY="${NEWAPI_API_KEY:-sk-60oOqvQYb8vziFfg2hPHlTKW3X80Pc6sIBDC5EFHCY0sn5NY}"

UPSTREAM_BASE_URL="${UPSTREAM_BASE_URL:-https://sd.dawnloadai.com:8443}"
UPSTREAM_API_KEY="${UPSTREAM_API_KEY:-sk-KIzafhiGGLfG14AdRxLvvL76bEN7lQ70zjQ0UpNZQm0rKfPS}"   # 上游 new-api 令牌（留空则跳过上游日志对比）

# 视频模型
MODEL="${MODEL:-doubao-seedance-2-0-260128}"

# 请求参数
PROMPT="${PROMPT:-一只猫在雪地上奔跑，阳光明媚}"
DURATION="${DURATION:-4}"
RESOLUTION="${RESOLUTION:-720p}"
RATIO="${RATIO:-9:16}"
# 参考图 URL（留空则为文生视频，填写则为图生视频）
IMAGE_URL="${IMAGE_URL:-https://static.horse-world.mints-id.com/rh/20260604204600/1780577160529_9921.png}"





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

# ---------- 提交视频任务 ----------
submit_task() {
  local body
  if [ -n "$IMAGE_URL" ]; then
    body=$(jq -n \
      --arg model "$MODEL" \
      --arg prompt "$PROMPT" \
      --arg seconds "$DURATION" \
      --arg resolution "$RESOLUTION" \
      --arg ratio "$RATIO" \
      --arg image_url "$IMAGE_URL" \
      '{
        model: $model,
        prompt: $prompt,
        seconds: $seconds,
        images: [$image_url],
        metadata: { resolution: $resolution, ratio: $ratio }
      }')
  else
    body=$(jq -n \
      --arg model "$MODEL" \
      --arg prompt "$PROMPT" \
      --arg seconds "$DURATION" \
      --arg resolution "$RESOLUTION" \
      --arg ratio "$RATIO" \
      '{
        model: $model,
        prompt: $prompt,
        seconds: $seconds,
        metadata: { resolution: $resolution, ratio: $ratio }
      }')
  fi

  log_info "提交请求体:"
  echo "$body" | jq '.'
  echo ""

  local tmpfile; tmpfile=$(mktemp)
  HTTP_CODE=$(curl -s -o "$tmpfile" -w "%{http_code}" \
    -X POST "${NEWAPI_BASE_URL}/v1/video/generations" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body")
  SUBMIT_BODY=$(cat "$tmpfile"); rm -f "$tmpfile"
}

# ---------- 轮询任务状态 ----------
poll_task() {
  local task_id="$1"
  local max_retries="${2:-40}"
  local interval="${3:-10}"
  local retry=0

  log_info "开始轮询 task_id=${task_id}，间隔 ${interval}s，最多 ${max_retries} 次"

  while [ $retry -lt $max_retries ]; do
    retry=$((retry + 1))
    sleep "$interval"

    local tmpfile; tmpfile=$(mktemp)
    curl -s -o "$tmpfile" \
      -X GET "${NEWAPI_BASE_URL}/v1/video/generations/${task_id}" \
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

# ---------- 查询日志（按 model + 时间范围，取最近一条消费记录）----------
# 参数: base_url user_token start_ts model_name
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
    -H "Authorization: ${token}" \
    -H "Content-Type: application/json" 2>/dev/null
}

# ============================================================
# 主流程
# ============================================================

check_deps

echo ""
log_sep
echo -e "${BOLD}  Seedance 视频生成 · 计费对比测试${NC}"
log_sep
echo -e "  本端 new-api:  ${NEWAPI_BASE_URL}"
[ -n "$UPSTREAM_BASE_URL" ] && echo -e "  上游 new-api:  ${UPSTREAM_BASE_URL}"
echo -e "  模型:          ${MODEL}"
echo -e "  Prompt:        ${PROMPT}"
echo -e "  时长/分辨率:   ${DURATION}s / ${RESOLUTION} / ${RATIO}"
[ -n "$IMAGE_URL" ] && echo -e "  参考图:        ${IMAGE_URL}"
log_sep

# ---------- Step 1: 提交任务 ----------
log_title "Step 1 · 提交 Seedance 任务"

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

VIDEO_URL=$(echo "$FINAL_TASK_BODY" | jq -r '.video_url // .url // .metadata.url // "N/A"' 2>/dev/null)

# ---------- Step 3: 查询本端日志 ----------
log_title "Step 3 · 查询本端扣费日志"

sleep 3  # 等待日志落库

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
echo -e "  ${BOLD}[ 说明 ]${NC}"
echo -e "  - 分组倍率 1x 时，本端扣费应与上游一致"
echo -e "  - 1元 = 500,000积分，ModelRatio=23 对应 ¥46/M tokens"

if [ "$VIDEO_URL" != "N/A" ] && [ -n "$VIDEO_URL" ]; then
  echo ""
  log_ok "视频地址: ${VIDEO_URL}"
fi

echo ""
log_sep
log_ok "测试完成"
log_sep
echo ""
