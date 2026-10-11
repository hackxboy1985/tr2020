#!/bin/bash
# ============================================================
# Seedance 2.0 素材图（资产）上传 + 引用生成 测试脚本
#
# 对应文档:
#   docs/咪咕/seedance-asset-simple.md
#   docs/seedance/多上游下游接口路径对照表.md
#
# 核心流程:
#   1. POST /api/seedance/assets          上传资产图（按 URL 引用，非文件字节）
#   2. GET  /api/seedance/assets/{id}     轮询直到 Status=Active
#   3. POST /v1/video/generations         用 asset://<asset_id> 生成视频
#   4. GET  /v1/videos/{task_id}          轮询任务
#
# 说明: 上游素材库只接受「公网可访问的 HTTPS 文件地址」，
#       如果手里只有本地图片文件，用 --upload-image 可先传到你自己的
#       图床/对象存储，再把公网 URL 交给素材库。
# ============================================================

set -uo pipefail

# ---------- 颜色 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_sep()   { echo -e "${CYAN}============================================================${NC}"; }
log_title() { echo -e "\n${BOLD}${CYAN}$*${NC}"; log_sep; }

# ---------- 帮助 ----------
show_help() {
  cat <<'EOF'

Seedance 2.0 素材图（资产）测试脚本
============================================================

用法:
  ./test-seedance-asset.sh --run                完整流程（上传资产图 → 激活 → 生成视频）
  ./test-seedance-asset.sh --asset-only         只测试资产上传 + 激活（不生成视频，不产生生成费用）
  ./test-seedance-asset.sh --query-asset <id>   查询素材状态（数字=LocalId，asset-xxx=上游ID）
  ./test-seedance-asset.sh --list-assets        查询素材列表
  ./test-seedance-asset.sh --list-groups        查询素材组列表

命令:
  --run            上传资产图并等待激活，然后用 asset:// 引用生成视频
  --asset-only     仅上传 + 轮询激活，打印可复用的 ASSET_REF 后退出
  --query-asset    查询指定素材状态
  --list-assets    查询素材列表
  --list-groups    查询素材组列表

必需参数:
  需要指定以上命令之一；不带参数则显示本帮助

关键环境变量:
  NEWAPI_BASE_URL   本地 new-api 地址 (默认: http://127.0.0.1:3000)
  NEWAPI_API_KEY    API 密钥 (必需，sk- 开头)
  ASSET_IMAGE_URL   资产图公网 URL（必需，HTTPS 可访问）
  ASSET_TYPE        素材类型 Image|Video|Audio (默认: Image)

可选环境变量:
  MODEL             视频模型 (默认: doubao-seedance-2-0)
  PROMPT            提示词 (默认: 女子人物面对镜头自然说话)
  DURATION          视频时长秒数 (默认: 4)
  RESOLUTION        分辨率 (默认: 480p)
  RATIO             宽高比 (默认: 16:9)
  GENERATE_AUDIO    是否生成音频 (默认: true)
  WATERMARK         是否加水印 (默认: false)
  ASSET_NAME        素材名称 (默认: asset-test-<时间戳>)
  ASSET_ID          直接复用已有上游素材 ID（设置后跳过上传）
  ASSET_API_STYLE   生成视频时的引用格式:
                      images  - 标准格式 images:[asset://xxx] (默认)
                      content - Ark 原生格式 content:[{type:image_url,...}]
  ASSET_POLL_MAX    素材激活最大轮询次数 (默认: 40)
  ASSET_POLL_INTERVAL 素材轮询间隔秒 (默认: 3)
  TASK_POLL_MAX     视频任务最大轮询次数 (默认: 90)
  TASK_POLL_INTERVAL 视频轮询间隔秒 (默认: 10)
  QUOTA_PER_YUAN    积分换算比 (默认: 500000)

使用示例:

  # 示例 1: 完整流程（上传资产图 → 激活 → 用资产图生成视频）
  NEWAPI_API_KEY=sk-xxx \
  ASSET_IMAGE_URL="https://your-cdn.com/role.png" \
  ./test-seedance-asset.sh --run

  # 示例 2: 只测资产上传激活（不花生成费用）
  NEWAPI_API_KEY=sk-xxx \
  ASSET_IMAGE_URL="https://your-cdn.com/role.png" \
  ./test-seedance-asset.sh --asset-only

  # 示例 3: 复用已激活的素材，直接用 asset:// 生成
  NEWAPI_API_KEY=sk-xxx \
  ASSET_ID="asset-20260923121923-gtqgb" \
  PROMPT="参考图中人物自然说话" \
  ./test-seedance-asset.sh --run

  # 示例 4: Ark 原生 content 格式引用资产图
  NEWAPI_API_KEY=sk-xxx \
  ASSET_IMAGE_URL="https://your-cdn.com/role.png" \
  ASSET_API_STYLE=content \
  ./test-seedance-asset.sh --run

  # 示例 5: 查询素材状态
  NEWAPI_API_KEY=sk-xxx ./test-seedance-asset.sh --query-asset 12
  NEWAPI_API_KEY=sk-xxx ./test-seedance-asset.sh --query-asset asset-20260923121923-gtqgb

注意事项:
  - 素材库按 URL 建索引：同一 URL 已有 Active 素材时直接复用，不重复上传
  - 素材激活通常需要 10-30 秒，超时可稍后用 --query-asset 复查
  - 资产图必须是公网可访问的 HTTPS 地址，内网/本地路径无法被上游拉取
  - --run 会创建计费视频任务；--asset-only 不产生生成费用

============================================================

EOF
  exit 0
}

# ---------- 参数解析 ----------
MODE=""
CLI_ASSET_ID=""
case "${1:-}" in
  --run)          MODE="run" ;;
  --asset-only)   MODE="asset-only" ;;
  --query-asset)
    MODE="query-asset"
    CLI_ASSET_ID="${2:-}"
    if [ -z "$CLI_ASSET_ID" ]; then
      log_error "缺少素材 ID"
      echo "用法: $0 --query-asset <asset_id|local_id>"
      exit 1
    fi
    ;;
  --list-assets)  MODE="list-assets" ;;
  --list-groups)  MODE="list-groups" ;;
  ""|--help|-h)   show_help ;;
  *)              log_error "未知参数: $1"; show_help ;;
esac

# ---------- 配置区 ----------
NEWAPI_BASE_URL="${NEWAPI_BASE_URL:-http://127.0.0.1:3000}"
NEWAPI_API_KEY="${NEWAPI_API_KEY:-}"

ASSET_IMAGE_URL="${ASSET_IMAGE_URL:-}"
ASSET_TYPE="${ASSET_TYPE:-Image}"
ASSET_NAME="${ASSET_NAME:-asset-test-$(date +%Y%m%d%H%M%S)}"
ASSET_ID="${ASSET_ID:-$CLI_ASSET_ID}"
ASSET_API_STYLE="${ASSET_API_STYLE:-images}"
ASSET_POLL_MAX="${ASSET_POLL_MAX:-40}"
ASSET_POLL_INTERVAL="${ASSET_POLL_INTERVAL:-3}"

MODEL="${MODEL:-doubao-seedance-2-0}"
PROMPT="${PROMPT:-女子人物面对镜头自然说话}"
DURATION="${DURATION:-4}"
RESOLUTION="${RESOLUTION:-480p}"
RATIO="${RATIO:-16:9}"
GENERATE_AUDIO="${GENERATE_AUDIO:-true}"
WATERMARK="${WATERMARK:-false}"

TASK_POLL_MAX="${TASK_POLL_MAX:-90}"
TASK_POLL_INTERVAL="${TASK_POLL_INTERVAL:-10}"
QUOTA_PER_YUAN="${QUOTA_PER_YUAN:-500000}"

PAGE="${PAGE:-1}"
PAGE_SIZE="${PAGE_SIZE:-10}"

# ---------- 前置检查 ----------
check_deps() {
  local missing=0
  for cmd in curl jq; do
    command -v "$cmd" &>/dev/null || { log_error "$cmd 未安装，请先安装"; missing=1; }
  done
  [ "$missing" -eq 0 ] || exit 1
  if command -v bc &>/dev/null; then HAVE_BC=1; else HAVE_BC=0; fi
}

require_key() {
  if [ -z "$NEWAPI_API_KEY" ]; then
    log_error "未设置 NEWAPI_API_KEY"
    echo "示例: NEWAPI_API_KEY=sk-xxx $0 ${MODE:+--$MODE}"
    exit 1
  fi
}

quota_to_yuan() {
  local q="$1"
  if [ "$q" = "null" ] || [ "$q" = "-1" ] || [ -z "$q" ]; then echo "N/A"; return; fi
  if [ "$HAVE_BC" -eq 1 ]; then
    echo "scale=6; $q / $QUOTA_PER_YUAN" | bc 2>/dev/null || echo "N/A"
  else
    echo "N/A"
  fi
}

# ---------- HTTP 封装：输出 BODY，全局 HTTP_CODE ----------
http_call() {
  local method="$1" url="$2" body="${3:-}"
  local tmpfile; tmpfile=$(mktemp)
  local args=(-s -o "$tmpfile" -w "%{http_code}" -X "$method" "$url"
              -H "Authorization: Bearer ${NEWAPI_API_KEY}"
              -H "Accept: application/json")
  if [ -n "$body" ]; then
    args+=(-H "Content-Type: application/json" -d "$body")
  fi
  HTTP_CODE=$(curl "${args[@]}")
  HTTP_BODY=$(cat "$tmpfile"); rm -f "$tmpfile"
}

pretty() { echo "$1" | jq '.' 2>/dev/null || echo "$1"; }

# ============================================================
# 素材：上传
# ============================================================
create_asset() {
  log_title "Step 1 · 上传资产图到素材库"

  if [ -z "$ASSET_IMAGE_URL" ]; then
    log_error "未设置 ASSET_IMAGE_URL（资产图必须是公网可访问的 URL）"
    echo "示例: ASSET_IMAGE_URL='https://your-cdn.com/role.png' $0 --run"
    exit 1
  fi
  case "$ASSET_IMAGE_URL" in
    http://*|https://*) ;;
    *) log_error "ASSET_IMAGE_URL 必须是 http(s) 开头的公网地址，当前: ${ASSET_IMAGE_URL}"; exit 1 ;;
  esac

  local req
  req=$(jq -n \
    --arg url "$ASSET_IMAGE_URL" \
    --arg type "$ASSET_TYPE" \
    --arg name "$ASSET_NAME" \
    '{URL:$url, AssetType:$type, Name:$name}')

  local url="${NEWAPI_BASE_URL}/api/seedance/assets"
  log_info "请求: POST ${url}"
  log_info "请求体:"; pretty "$req"

  http_call POST "$url" "$req"
  log_info "HTTP ${HTTP_CODE}"

  if [ "$HTTP_CODE" != "200" ]; then
    log_error "素材上传失败"
    pretty "$HTTP_BODY"
    exit 1
  fi

  ASSET_UPSTREAM_ID=$(echo "$HTTP_BODY" | jq -r '.Result.Id // empty')
  ASSET_LOCAL_ID=$(echo "$HTTP_BODY" | jq -r '.Result.LocalId // empty')
  ASSET_REF=$(echo "$HTTP_BODY" | jq -r '.Result.AssetRef // empty')
  ASSET_STATUS=$(echo "$HTTP_BODY" | jq -r '.Result.Status // "unknown"')

  if [ -z "$ASSET_UPSTREAM_ID" ]; then
    log_error "响应中没有素材 ID"
    pretty "$HTTP_BODY"
    exit 1
  fi
  [ -n "$ASSET_REF" ] || ASSET_REF="asset://${ASSET_UPSTREAM_ID}"

  log_ok "素材已受理"
  log_info "上游素材 ID: ${ASSET_UPSTREAM_ID}"
  log_info "本地素材 ID: ${ASSET_LOCAL_ID}"
  log_info "初始状态:    ${ASSET_STATUS}"
  log_info "引用格式:    ${ASSET_REF}"
}

# ============================================================
# 素材：轮询激活（可接受初始状态，避免重复查询）
# ============================================================
wait_asset_active() {
  local current_status="${1:-unknown}"
  local query_id="${ASSET_LOCAL_ID:-$ASSET_UPSTREAM_ID}"

  if [ "$current_status" = "Active" ]; then
    log_ok "素材已是 Active，无需等待"
    ASSET_STATUS="Active"
    return 0
  fi

  log_title "Step 2 · 等待素材激活"
  log_info "查询: GET ${NEWAPI_BASE_URL}/api/seedance/assets/${query_id}"
  log_info "间隔 ${ASSET_POLL_INTERVAL}s，最多 ${ASSET_POLL_MAX} 次"

  local i=0
  while [ "$i" -lt "$ASSET_POLL_MAX" ]; do
    i=$((i + 1))
    sleep "$ASSET_POLL_INTERVAL"

    http_call GET "${NEWAPI_BASE_URL}/api/seedance/assets/${query_id}"
    if [ "$HTTP_CODE" != "200" ]; then
      log_error "查询素材失败 HTTP ${HTTP_CODE}"
      pretty "$HTTP_BODY"
      exit 1
    fi

    ASSET_STATUS=$(echo "$HTTP_BODY" | jq -r '.Result.Status // "unknown"')
    log_info "#${i}/${ASSET_POLL_MAX} 素材状态: ${ASSET_STATUS}"

    case "$ASSET_STATUS" in
      Active)
        log_ok "素材已激活: ${ASSET_REF}"
        return 0
        ;;
      Failed)
        log_error "素材处理失败"
        pretty "$HTTP_BODY"
        exit 1
        ;;
    esac
  done

  log_error "素材激活超时（最后状态: ${ASSET_STATUS}）"
  log_info "素材仍在处理时可稍后复查:"
  echo "  NEWAPI_API_KEY=... $0 --query-asset ${ASSET_UPSTREAM_ID}"
  exit 1
}

# ============================================================
# 视频：构造 content/images 并提交
# ============================================================
build_video_body() {
  local content_array images_array

  if [ "$ASSET_API_STYLE" = "content" ]; then
    content_array=$(jq -n \
      --arg text "$PROMPT" \
      --arg url "$ASSET_REF" \
      '[{"type":"text","text":$text},
        {"type":"image_url","image_url":{"url":$url}}]')
    jq -n \
      --arg model "$MODEL" \
      --argjson duration "$DURATION" \
      --arg resolution "$RESOLUTION" \
      --arg ratio "$RATIO" \
      --argjson generate_audio "$GENERATE_AUDIO" \
      --argjson watermark "$WATERMARK" \
      --argjson content "$content_array" \
      '{model:$model, content:$content, resolution:$resolution, ratio:$ratio,
        duration:$duration, generate_audio:$generate_audio, watermark:$watermark}'
  else
    images_array=$(jq -n --arg url "$ASSET_REF" '[$url]')
    jq -n \
      --arg model "$MODEL" \
      --arg prompt "$PROMPT" \
      --arg seconds "$DURATION" \
      --argjson images "$images_array" \
      --arg resolution "$RESOLUTION" \
      --arg ratio "$RATIO" \
      --argjson generate_audio "$GENERATE_AUDIO" \
      --argjson watermark "$WATERMARK" \
      '{model:$model, prompt:$prompt, seconds:$seconds, images:$images,
        metadata:{resolution:$resolution, ratio:$ratio,
                  generate_audio:$generate_audio, watermark:$watermark}}'
  fi
}

submit_video() {
  log_title "Step 3 · 用资产图提交视频任务（计费）"

  local body; body=$(build_video_body)
  log_info "引用方式: ASSET_API_STYLE=${ASSET_API_STYLE}"
  log_info "资产引用: ${ASSET_REF}"
  log_info "请求: POST ${NEWAPI_BASE_URL}/v1/video/generations"
  log_info "请求体:"; pretty "$body"

  SUBMIT_TS=$(date +%s)
  http_call POST "${NEWAPI_BASE_URL}/v1/video/generations" "$body"
  log_info "HTTP ${HTTP_CODE}"

  if [ "$HTTP_CODE" != "200" ]; then
    log_error "视频任务提交失败"
    pretty "$HTTP_BODY"
    exit 1
  fi

  TASK_ID=$(echo "$HTTP_BODY" | jq -r '.id // .task_id // empty')
  SUBMIT_STATUS=$(echo "$HTTP_BODY" | jq -r '.status // "unknown"')
  if [ -z "$TASK_ID" ]; then
    log_error "未获取到 task_id"
    pretty "$HTTP_BODY"
    exit 1
  fi
  log_ok "task_id = ${TASK_ID}  状态 = ${SUBMIT_STATUS}"
}

poll_video() {
  log_title "Step 4 · 轮询视频任务"
  log_info "查询: GET ${NEWAPI_BASE_URL}/v1/videos/${TASK_ID}"
  log_info "间隔 ${TASK_POLL_INTERVAL}s，最多 ${TASK_POLL_MAX} 次"

  local i=0
  while [ "$i" -lt "$TASK_POLL_MAX" ]; do
    i=$((i + 1))
    sleep "$TASK_POLL_INTERVAL"

    http_call GET "${NEWAPI_BASE_URL}/v1/videos/${TASK_ID}"
    FINAL_TASK_BODY="$HTTP_BODY"
    local status; status=$(echo "$HTTP_BODY" | jq -r '.status // "unknown"')
    local progress; progress=$(echo "$HTTP_BODY" | jq -r '.progress // ""')
    log_info "#${i}/${TASK_POLL_MAX} 状态: ${status} ${progress}"

    case "$status" in
      succeeded|completed|success) POLL_RESULT=0; return 0 ;;
      failed|error)
        log_error "任务失败"
        pretty "$HTTP_BODY"
        POLL_RESULT=1; return 1 ;;
    esac
  done

  log_warn "轮询超时，任务可能仍在处理（task_id=${TASK_ID}）"
  POLL_RESULT=2
  return 2
}

query_log() {
  local end_ts; end_ts=$(date +%s)
  curl -s \
    "${NEWAPI_BASE_URL}/api/log/self?type=1&model_name=${MODEL}&start_timestamp=${SUBMIT_TS}&end_timestamp=${end_ts}&p=1&page_size=5" \
    -H "Authorization: Bearer ${NEWAPI_API_KEY}" \
    -H "Content-Type: application/json" 2>/dev/null
}

# ============================================================
# 主流程
# ============================================================
check_deps

# ---- 查询/列表模式 ----
if [ "$MODE" = "query-asset" ]; then
  require_key
  log_title "查询素材状态"
  if [[ "$ASSET_ID" =~ ^asset://(.+)$ ]]; then
    ASSET_ID="${BASH_REMATCH[1]}"
    log_info "从 asset:// 引用中提取 ID: ${ASSET_ID}"
  fi
  log_info "查询: GET ${NEWAPI_BASE_URL}/api/seedance/assets/${ASSET_ID}"
  http_call GET "${NEWAPI_BASE_URL}/api/seedance/assets/${ASSET_ID}"
  log_info "HTTP ${HTTP_CODE}"
  pretty "$HTTP_BODY"
  if [ "$HTTP_CODE" != "200" ]; then exit 1; fi
  local_status=$(echo "$HTTP_BODY" | jq -r '.Result.Status // "unknown"')
  local_upstream=$(echo "$HTTP_BODY" | jq -r '.Result.Id // ""')
  case "$local_status" in
    Active)   log_ok "素材可用，引用: asset://${local_upstream}" ;;
    Failed)   log_error "素材处理失败" ;;
    *)        log_warn "素材状态: ${local_status}，稍后重试" ;;
  esac
  exit 0
fi

if [ "$MODE" = "list-assets" ]; then
  require_key
  log_title "查询素材列表"
  http_call GET "${NEWAPI_BASE_URL}/api/seedance/assets?page=${PAGE}&page_size=${PAGE_SIZE}"
  log_info "HTTP ${HTTP_CODE}"
  pretty "$HTTP_BODY"
  exit 0
fi

if [ "$MODE" = "list-groups" ]; then
  require_key
  log_title "查询素材组列表"
  http_call GET "${NEWAPI_BASE_URL}/api/seedance/asset-groups?page=${PAGE}&page_size=${PAGE_SIZE}"
  log_info "HTTP ${HTTP_CODE}"
  pretty "$HTTP_BODY"
  exit 0
fi

# ---- 上传 + 激活 流程 ----
require_key

echo ""
log_sep
echo -e "${BOLD}  Seedance 2.0 素材图上传 + 引用生成测试${NC}"
log_sep
echo -e "  本端 new-api:  ${NEWAPI_BASE_URL}"
echo -e "  模型:          ${MODEL}"
echo -e "  资产图:        ${ASSET_IMAGE_URL:-（复用 ASSET_ID=${ASSET_ID}）}"
echo -e "  素材类型:      ${ASSET_TYPE}"
echo -e "  引用方式:      ${ASSET_API_STYLE}"
[ "$MODE" = "run" ] && echo -e "  生成参数:      ${DURATION}s / ${RESOLUTION} / ${RATIO}  音频=${GENERATE_AUDIO} 水印=${WATERMARK}"
log_sep

if [ -n "$ASSET_ID" ]; then
  log_title "Step 1 · 复用已有素材（跳过上传）"
  ASSET_REF="asset://${ASSET_ID}"
  ASSET_LOCAL_ID=""
  ASSET_UPSTREAM_ID="$ASSET_ID"
  log_ok "使用资产引用: ${ASSET_REF}"
  # 查询当前状态，已激活则跳过轮询
  http_call GET "${NEWAPI_BASE_URL}/api/seedance/assets/${ASSET_ID}"
  if [ "$HTTP_CODE" = "200" ]; then
    ASSET_STATUS=$(echo "$HTTP_BODY" | jq -r '.Result.Status // "unknown"')
    ASSET_LOCAL_ID=$(echo "$HTTP_BODY" | jq -r '.Result.LocalId // ""')
    log_info "当前素材状态: ${ASSET_STATUS}"
  else
    log_warn "查询素材状态失败 HTTP ${HTTP_CODE}，将按未激活处理"
    ASSET_STATUS="unknown"
  fi
  wait_asset_active "$ASSET_STATUS"
else
  create_asset
  wait_asset_active "$ASSET_STATUS"
fi

log_title "资产就绪"
log_ok "ASSET_REF = ${ASSET_REF}"
echo ""
log_info "可复用此素材直接生成（跳过上传）:"
echo "  ASSET_ID=${ASSET_UPSTREAM_ID} NEWAPI_API_KEY=... $0 --run"

if [ "$MODE" = "asset-only" ]; then
  echo ""
  log_sep
  log_ok "✅ 素材上传测试成功（未生成视频，无生成费用）"
  log_sep
  exit 0
fi

# ---- 生成视频 ----
submit_video
FINAL_TASK_BODY=""
POLL_RESULT=2
poll_video
FINISH_TS=$(date +%s)

echo ""
log_info "最终任务详情:"
pretty "$FINAL_TASK_BODY"

VIDEO_URL_RESULT=$(echo "$FINAL_TASK_BODY" | jq -r '.content.video_url // .metadata.url // empty' 2>/dev/null)

# ---- 扣费日志 ----
log_title "Step 5 · 查询本端扣费日志"
sleep 3
LOG_RESP=$(query_log)
NEWAPI_QUOTA="N/A"; NEWAPI_YUAN="N/A"
if [ -n "$LOG_RESP" ] && echo "$LOG_RESP" | jq -e '.data' &>/dev/null; then
  NEWAPI_QUOTA=$(echo "$LOG_RESP" | jq -r '(.data.items[0].quota // .data[0].quota // "N/A") | tostring')
  NEWAPI_YUAN=$(quota_to_yuan "$NEWAPI_QUOTA")
  log_ok "本端扣费: ${NEWAPI_QUOTA} 积分 ≈ ${NEWAPI_YUAN} 元"
else
  log_warn "未取到日志，跳过扣费查询"
fi

# ---- 报告 ----
log_title "测试报告"
echo -e "  素材上游 ID:  ${ASSET_UPSTREAM_ID}"
echo -e "  素材本地 ID:  ${ASSET_LOCAL_ID:-N/A}"
echo -e "  资产引用:     ${ASSET_REF}"
echo -e "  task_id:      ${TASK_ID}"
echo -e "  耗时:         $((FINISH_TS - SUBMIT_TS))s"
[ "$NEWAPI_QUOTA" != "N/A" ] && echo -e "  本端扣费:     ${NEWAPI_QUOTA} 积分 ≈ ${NEWAPI_YUAN} 元"
if [ -n "$VIDEO_URL_RESULT" ]; then
  echo ""
  log_ok "视频地址: ${VIDEO_URL_RESULT}"
fi
echo ""
log_sep
if [ "$POLL_RESULT" -eq 0 ]; then
  log_ok "测试完成"
elif [ "$POLL_RESULT" -eq 1 ]; then
  log_error "测试失败：生成任务失败"
  exit 1
else
  log_warn "测试未完成：轮询超时，请用 task_id 复查"
  exit 2
fi
log_sep
