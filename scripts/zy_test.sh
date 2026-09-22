#!/usr/bin/env bash
# ============================================================
# zy 渠道异步生图 下游调用测试脚本
#
# 走 new-api 对外接口，验证 zy 渠道（ChannelTypeZy = 61）的
# 「提交 → 轮询 → 取图」完整链路。
#
#   提交: POST /v1/images/generations   （同步返回 task_id）
#   查询: GET  /v1/images/tasks/{id}    （异步轮询取结果）
#
# 用法:
#   ./zy_test.sh                                  # 文生图（默认参数）
#   ./zy_test.sh -p "一只猫" -m gpt-image-2.5-flare -r 16:9 -s 2K
#   ./zy_test.sh -m gpt-image-2 -S 1280x720       # 用像素 size
#   ./zy_test.sh -i https://x.com/a.png           # 图生图（可多次）
#   ./zy_test.sh --query task_xxxx                # 只查询已有任务
#   ./zy_test.sh --dry-run                        # 只打印报文不发送
#
# 环境变量（也可用命令行参数覆盖）:
#   NEWAPI_BASE_URL   默认 http://127.0.0.1:3000
#   NEWAPI_API_KEY    必填，new-api 的令牌（sk-xxx）
# ============================================================

set -uo pipefail

# ---------- 默认配置 ----------
NEWAPI_BASE_URL="${NEWAPI_BASE_URL:-http://www.luluai.cc}"
NEWAPI_API_KEY="${NEWAPI_API_KEY:-sk-UJkSn1Bxs2Jynb8pBj3iIpbfiXjVVnlsQQeMq8gh33EfBV37}"

MODEL="${MODEL:-gpt-image-2.5-flare}"
PROMPT="${PROMPT:-一只橘猫坐在窗台上，阳光洒落，写实摄影风格}"
ASPECT_RATIO="${ASPECT_RATIO:-16:9}"   # 与 SIZE 二选一，优先
SIZE="${SIZE:-}"                   # 比例串(16:9)或像素(1280x720)
IMAGE_SIZE="${IMAGE_SIZE:-}"       # 档位 1K/2K/4K（等价 RESOLUTION）
RESOLUTION="${RESOLUTION:-4K}"     # 档位 1K/2K/4K（等价 RESOLUTION）
QUALITY="${QUALITY:-}"

MAX_RETRIES="${MAX_RETRIES:-60}"
POLL_INTERVAL="${POLL_INTERVAL:-5}"
OUTPUT_DIR="${OUTPUT_DIR:-}"

DRY_RUN=false
QUERY_MODE=false
QUERY_TASK_ID=""

IMAGES=()

# ---------- 颜色输出 ----------
if [ -t 1 ]; then
  RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
  CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
else
  RED=''; GREEN=''; YELLOW=''; CYAN=''; BOLD=''; NC=''
fi

log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
log_title() { echo -e "\n${BOLD}${CYAN}$*${NC}"; echo -e "${CYAN}------------------------------------------------------------${NC}"; }

usage() {
  # 打印文件头部的注释块（第 2 行到 `set -uo` 之前）
  sed -n '2,/^set -uo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

# ---------- 参数解析 ----------
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--prompt)       PROMPT="$2"; shift 2 ;;
    -m|--model)        MODEL="$2"; shift 2 ;;
    -r|--aspect-ratio) ASPECT_RATIO="$2"; shift 2 ;;
    -S|--size)         SIZE="$2"; shift 2 ;;
    -s|--image-size)   IMAGE_SIZE="$2"; shift 2 ;;
    -R|--resolution)   RESOLUTION="$2"; shift 2 ;;
    -q|--quality)      QUALITY="$2"; shift 2 ;;
    -i|--image)        IMAGES+=("$2"); shift 2 ;;
    -u|--base-url)     NEWAPI_BASE_URL="$2"; shift 2 ;;
    -k|--api-key)      NEWAPI_API_KEY="$2"; shift 2 ;;
    -o|--output-dir)   OUTPUT_DIR="$2"; shift 2 ;;
    --max-retries)     MAX_RETRIES="$2"; shift 2 ;;
    --interval)        POLL_INTERVAL="$2"; shift 2 ;;
    --query)           QUERY_MODE=true; QUERY_TASK_ID="$2"; shift 2 ;;
    --dry-run)         DRY_RUN=true; shift ;;
    -h|--help)         usage 0 ;;
    *) log_error "未知参数: $1"; usage 1 ;;
  esac
done

# ---------- 依赖检查 ----------
for cmd in curl jq; do
  command -v "$cmd" >/dev/null 2>&1 || { log_error "缺少依赖: $cmd，请先安装"; exit 1; }
done

if [ "$DRY_RUN" = false ] && [ -z "$NEWAPI_API_KEY" ]; then
  log_error "缺少 API Key，请设置 NEWAPI_API_KEY 或使用 -k sk-xxx"
  echo ""
  usage 1
fi

NEWAPI_BASE_URL="${NEWAPI_BASE_URL%/}"

# ---------- 构造请求体 ----------
build_body() {
  local jq_args=(--arg model "$MODEL" --arg prompt "$PROMPT")
  local filter='{model: $model, prompt: $prompt}'

  # aspect_ratio 优先，其次 size（与渠道校验顺序一致）
  if [ -n "$ASPECT_RATIO" ]; then
    jq_args+=(--arg aspect_ratio "$ASPECT_RATIO")
    filter+=' + {aspect_ratio: $aspect_ratio}'
  elif [ -n "$SIZE" ]; then
    jq_args+=(--arg size "$SIZE")
    filter+=' + {size: $size}'
  fi

  # image_size / resolution 等价，只传其一
  if [ -n "$IMAGE_SIZE" ]; then
    jq_args+=(--arg image_size "$IMAGE_SIZE")
    filter+=' + {image_size: $image_size}'
  elif [ -n "$RESOLUTION" ]; then
    jq_args+=(--arg resolution "$RESOLUTION")
    filter+=' + {resolution: $resolution}'
  fi

  if [ -n "$QUALITY" ]; then
    jq_args+=(--arg quality "$QUALITY")
    filter+=' + {quality: $quality}'
  fi

  # 参考图（图生图），最多 8 张
  if [ "${#IMAGES[@]}" -gt 0 ]; then
    local imgs_json
    imgs_json=$(printf '%s\n' "${IMAGES[@]}" | jq -R . | jq -s .)
    jq_args+=(--argjson images "$imgs_json")
    filter+=' + {images: $images}'
  fi

  filter+=' + {response_format: "url"}'

  jq -n "${jq_args[@]}" "$filter"
}

# ---------- 提交任务 ----------
submit_task() {
  local body="$1"

  log_title "1. 提交生图任务"
  log_info "POST ${NEWAPI_BASE_URL}/v1/images/generations"
  echo "$body" | jq '.'

  local resp http_code
  resp=$(curl -sS -w $'\n%{http_code}' \
    -X POST "${NEWAPI_BASE_URL}/v1/images/generations" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body" 2>&1) || { log_error "请求失败: $resp"; return 1; }

  http_code=$(printf '%s' "$resp" | tail -n1)
  resp=$(printf '%s' "$resp" | sed '$d')

  echo ""
  log_info "HTTP ${http_code}"
  log_info "响应:"
  printf '%s' "$resp" | jq '.' 2>/dev/null || printf '%s\n' "$resp"

  if [ "$http_code" != "200" ]; then
    log_error "提交失败（HTTP ${http_code}）"
    return 1
  fi

  SUBMIT_BODY="$resp"
  TASK_ID=$(printf '%s' "$resp" | jq -r '.id // empty')
  if [ -z "$TASK_ID" ]; then
    log_error "响应中没有 task id"
    return 1
  fi

  log_ok "任务已提交，task_id = ${TASK_ID}"
  return 0
}

# ---------- 查询任务 ----------
fetch_task() {
  local task_id="$1"

  local resp http_code
  resp=$(curl -sS -w $'\n%{http_code}' \
    -X GET "${NEWAPI_BASE_URL}/v1/images/tasks/${task_id}" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" 2>&1) || return 1

  http_code=$(printf '%s' "$resp" | tail -n1)
  resp=$(printf '%s' "$resp" | sed '$d')

  if [ "$http_code" != "200" ]; then
    log_error "查询失败（HTTP ${http_code}）: $(printf '%s' "$resp" | jq -c '.error // .' 2>/dev/null || printf '%s' "$resp")"
    return 1
  fi

  POLL_BODY="$resp"
  return 0
}

# ---------- 轮询 ----------
poll_task() {
  local task_id="$1"
  local n=0 status

  log_title "2. 轮询任务状态"
  log_info "GET ${NEWAPI_BASE_URL}/v1/images/tasks/${task_id}，间隔 ${POLL_INTERVAL}s，最多 ${MAX_RETRIES} 次"

  while [ "$n" -lt "$MAX_RETRIES" ]; do
    n=$((n + 1))
    sleep "$POLL_INTERVAL"

    fetch_task "$task_id" || { log_warn "#${n} 查询异常，继续重试"; continue; }

    status=$(printf '%s' "$POLL_BODY" | jq -r '.status // "unknown"')
    printf '  #%-3s status=%s\n' "$n" "$status"

    case "$status" in
      succeeded|success|completed)
        log_ok "生图完成"
        return 0
        ;;
      failed|error)
        log_error "生图失败"
        printf '%s' "$POLL_BODY" | jq '.error // .'
        return 1
        ;;
    esac
  done

  log_warn "轮询超时，任务可能仍在处理中（task_id=${task_id}）"
  return 2
}

# ---------- 输出结果 ----------
show_result() {
  log_title "3. 结果"

  local status
  status=$(printf '%s' "$POLL_BODY" | jq -r '.status // "unknown"')

  printf '%s' "$POLL_BODY" | jq '.'

  if [ "$status" != "succeeded" ] && [ "$status" != "success" ] && [ "$status" != "completed" ]; then
    return 0
  fi

  local urls
  urls=$(printf '%s' "$POLL_BODY" | jq -r '.result.data[]?.url // empty')
  if [ -z "$urls" ]; then
    log_warn "响应中没有图片 URL"
    return 0
  fi

  echo ""
  log_ok "图片地址:"
  local idx=0
  while IFS= read -r u; do
    [ -z "$u" ] && continue
    idx=$((idx + 1))
    echo "  ${idx}. ${u}"
  done <<< "$urls"

  # 可选下载
  if [ -n "$OUTPUT_DIR" ]; then
    mkdir -p "$OUTPUT_DIR"
    local i=0
    while IFS= read -r u; do
      [ -z "$u" ] && continue
      i=$((i + 1))
      local ext out
      ext="${u%%\?*}"
      ext="${ext##*.}"
      case "$ext" in
        png|jpg|jpeg|webp|gif) ;;
        *) ext="png" ;;
      esac
      out="${OUTPUT_DIR}/${TASK_ID}_${i}.${ext}"
      if curl -sS -L -o "$out" "$u"; then
        log_ok "已下载: ${out}"
      else
        log_error "下载失败: ${u}"
      fi
    done <<< "$urls"
  fi
}

# ============================================================
# 主流程
# ============================================================
main() {
  log_title "zy 渠道异步生图测试"
  log_info "new-api: ${NEWAPI_BASE_URL}"
  log_info "model:   ${MODEL}"

  # --query 模式：只查已有任务
  if [ "$QUERY_MODE" = true ]; then
    TASK_ID="$QUERY_TASK_ID"
    log_info "查询已有任务: ${TASK_ID}"
    if fetch_task "$TASK_ID"; then
      show_result
    else
      exit 1
    fi
    # 若仍在处理中，进入轮询
    local st
    st=$(printf '%s' "$POLL_BODY" | jq -r '.status // "unknown"')
    if [ "$st" = "processing" ]; then
      poll_task "$TASK_ID" && show_result
    fi
    exit 0
  fi

  local body
  body=$(build_body)

  if [ "$DRY_RUN" = true ]; then
    log_title "请求体（--dry-run，未发送）"
    echo "$body" | jq '.'
    exit 0
  fi

  submit_task "$body" || exit 1

  local rc=0
  poll_task "$TASK_ID" || rc=$?
  show_result
  exit "$rc"
}

main
