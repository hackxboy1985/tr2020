#!/bin/bash
# ============================================================
# Seedance 海外版（seedance-gateway）简化测试脚本
# 对应文档: docs/海外sd/Seedance_API客户接口文档_0.6.4.md
#           docs/海外sd/docs/VIDEO_API.md
#           docs/海外sd/docs/REPAIR_API.md
#
# 本脚本直连海外网关（不做本地 new-api 中转），所有路径拼接到 API_BASE：
#   API_BASE 默认: http://106.54.45.168/seedance-gateway
#
# 接口路径（均为 API_BASE 之后的部分）:
#   模型/能力:  GET  /v1/models
#               GET  /seedance/capabilities
#   创建素材:   POST /seedance/media                     （action=import，URL 导入）
#               POST /seedance/media/upload?...          （原始字节上传本地文件）
#   素材管理:   POST /seedance/media                     （get/batch_get/list/search/update/delete）
#               GET  /seedance/media?...                 （分页查询本人素材）
#               POST /seedance/media/groups              （action=create/get/list/update/delete）
#               POST /seedance/media/preflight           （素材与目标模型预检查，不扣费）
#   创建视频:   POST /v1/videos                          （主协议，支持 prompt 或 content 两种）
#               POST /seedance/api/v3/contents/generations/tasks  （旧协议兼容）
#   查询视频:   GET  /v1/videos/{id}
#               GET  /seedance/api/v3/contents/generations/tasks/{id}
#   下载视频:   GET  /v1/videos/{id}/content              （支持 HEAD；旧下载见 /v1/tasks/{id}/artifacts/video/content）
#   任务列表:   GET  /v1/videos?page=&page_size=
#   隐藏任务:   DELETE /v1/videos/{id}
#   回调状态:   GET  /seedance/callbacks/{task_id}
# ============================================================

# ---------- 帮助信息 ----------
show_help() {
  cat <<EOF

Seedance 海外版（seedance-gateway）测试脚本
============================================================

用法:
  ${0##*/} --text2video [选项]
  ${0##*/} --execute [选项]
  ${0##*/} --test-asset [选项]
  ${0##*/} --query-asset <asset_id> [选项]
  ${0##*/} --delete-asset <asset_id> [选项]
  ${0##*/} --list-assets [选项]
  ${0##*/} --preflight <asset_id> [选项]
  ${0##*/} --create-group [选项]
  ${0##*/} --query-group <group_id> [选项]
  ${0##*/} --list-groups [选项]
  ${0##*/} --check-models
  ${0##*/} --check-capabilities
  ${0##*/} --list-tasks [选项]
  ${0##*/} --fetch-ark <task_id>
  ${0##*/} --download-task <task_id> [选项]

命令:
  --text2video        纯文生视频：不建组、不建素材、不引用任何素材，直接 POST /v1/videos
                      然后轮询 + 下载。别名 --t2v
  --execute           执行完整测试（建组 + 建素材 + 轮询素材 + 提交视频 + 轮询 + 下载）
                      设置 TEXT2VIDEO=1 时等价于 --text2video
  --test-asset        仅测试素材创建（URL 导入或本地字节上传）+ 轮询到 succeeded
  --query-asset       查询单个素材（先 get，再可选 preflight）
  --delete-asset      删除素材（action=delete，需要显式执行）
  --list-assets       分页查询本人素材（支持 group_id/keyword/media_type/status/tag）
  --preflight         素材与目标模型预检查（不创建任务、不扣费）
  --create-group      创建素材组（action=create）
  --query-group       查询素材组（action=get）
  --list-groups       分页查询素材组（action=list）
  --check-models      查询当前 Key 可见模型（GET /v1/models）
  --check-capabilities 查询接口能力与限制（GET /seedance/capabilities）
  --list-tasks        分页查询本人视频任务（GET /v1/videos）
  --fetch-ark         同时用主协议和旧协议查询同一任务，做对比
  --download-task     下载已完成任务的第一个输出到本地文件

必填:
  需要指定以上命令之一；未指定时显示本帮助。

环境变量:
  API_BASE              海外网关基址（默认: http://106.54.45.168/seedance-gateway）
  API_KEY               网关 API Key（必填，需有目标模型 + media-service 权限）
  MODEL                 视频模型（默认: seedance-2.0-720p；分辨率由后缀决定）
  REQUEST_FORMAT        创建视频请求体格式: content（默认）| prompt
  TEXT2VIDEO            1/true 时纯文生视频：忽略所有素材参数，不引用素材（默认: 0）
  PROMPT                提示词（两种格式都必填）
  DURATION              时长秒数，整数（默认: 5）
  RATIO                 比例（默认: 16:9，可选 16:9/9:16/4:3/3:4/1:1）
  RESOLUTION            分辨率（默认空；填写必须与模型后缀一致）
  SIZE                  目标尺寸（仅 prompt 格式，默认: 1920x1080）
  N                     生成数量 1-4（默认: 1，平台扩展）
  IDEMPOTENCY_KEY       幂等键（8-128 位；留空自动生成，重试务必复用同一个）

素材相关:
  ASSET_URL             URL 导入素材的地址（默认用下面的角色图地址）
  ASSET_UPLOAD_FILE     本地文件路径；设置后走 /seedance/media/upload 原始字节上传
  ASSET_TITLE           素材标题（默认: os-reference）
  ASSET_MEDIA_TYPE      image|video|audio（默认按 URL 后缀/上传 MIME 推断）
  ASSET_TAGS            标签，逗号分隔（最多 20 项，每项 1-64 字符）
  ASSET_ROLE            content 中的图片角色（默认: reference_image）
  ASSET_ID              直接使用已有素材 ID 或 asset:// 引用，跳过创建
  GROUP_NAME            素材组名（--execute 会先创建并关联；留空则建组时用默认名）
  ASSET_GROUP_ID        已有素材组 ID（设置后跳过创建组）

content 格式的参考素材（REQUEST_FORMAT=content 时生效）:
  IMAGE_URL             场景图 URL，role=reference_image
  LAST_FRAME_URL        尾帧图 URL；设置后角色图改为 first_frame，且不要与其它参考混用
  VIDEO_REF_URL         参考视频 URL（须同时给 VIDEO_REF_DURATION）
  VIDEO_REF_DURATION    参考视频真实时长（秒）
  AUDIO_REF_URL         参考音频 URL（须同时给 AUDIO_REF_DURATION）
  AUDIO_REF_DURATION    参考音频真实时长（秒）

轮询/下载:
  POLL_INTERVAL         视频轮询间隔秒（默认: 10）
  POLL_MAX              视频轮询最大次数（默认: 40）
  ASSET_POLL_INTERVAL   素材轮询间隔秒（默认: 3）
  ASSET_POLL_MAX        素材轮询最大次数（默认: 30）
  DOWNLOAD              1/true 时 --execute 完成后自动下载（默认: 1）
  OUTPUT_FILE           下载保存路径（默认: ./seedance-os-<task_id>.mp4）

列表查询（--list-assets / --list-groups / --list-tasks）:
  PAGE / PAGE_SIZE      分页（默认 1 / 20；素材与任务每页最大 100）
  FILTER_GROUP_ID       仅 --list-assets：按素材组过滤
  FILTER_KEYWORD        仅 --list-assets：按关键词搜索
  FILTER_MEDIA_TYPE     仅 --list-assets：image|video|audio
  FILTER_TAG            仅 --list-assets：按完整标签匹配
  FILTER_STATUS         素材状态（queued/running/succeeded/failed）或任务状态
  FILTER_MODEL          仅 --list-tasks：按模型过滤
  FILTER_IDS            仅 --list-tasks：逗号分隔的任务 ID

使用示例:
  # 1. 完整流程（URL 导入素材 -> 首帧引用生成 -> 轮询 -> 下载）
  API_KEY=sk-xxx ${0##*/} --execute

  # 2. 本地文件字节上传作为素材（不要用 multipart）
  API_KEY=sk-xxx ASSET_UPLOAD_FILE=./reference.png ${0##*/} --test-asset

  # 3. 只用已就绪的素材生成，content 格式 + 音频参考
  API_KEY=sk-xxx ASSET_ID=asset://task_asset_example \\
    AUDIO_REF_URL=https://example.com/a.wav AUDIO_REF_DURATION=3 \\
    ${0##*/} --execute

  # 4. 纯文生视频（不使用任何素材，最快验证通路）
  API_KEY=sk-xxx ${0##*/} --text2video
  API_KEY=sk-xxx MODEL=seedance-2.0-720p PROMPT="一只柯基在草地上奔跑" \\
    DURATION=5 ${0##*/} --t2v

  # 5. 查询素材 / 列表 / 预检查
  API_KEY=sk-xxx ${0##*/} --query-asset task_asset_example
  API_KEY=sk-xxx ${0##*/} --list-assets
  API_KEY=sk-xxx ${0##*/} --preflight task_asset_example

  # 6. 素材组
  API_KEY=sk-xxx ${0##*/} --create-group
  API_KEY=sk-xxx ${0##*/} --list-groups

  # 7. 任务对比与下载
  API_KEY=sk-xxx ${0##*/} --fetch-ark task_example
  API_KEY=sk-xxx ${0##*/} --download-task task_example

  # 8. 先确认 Key 的模型与能力，再提交付费任务
  API_KEY=sk-xxx ${0##*/} --check-models
  API_KEY=sk-xxx ${0##*/} --check-capabilities

注意事项:
  - 当前网关是 HTTP 明文接入，Key 只放服务端环境变量，不要写进客户端/日志/URL
  - 生成视频从完成起保存 7 天（604800 秒），参考素材读取签名同样 7 天
  - 每用户素材容量 512 MiB，单文件上传 64 MiB
  - 素材状态为 queued/running/succeeded/failed，只有 succeeded 才能被引用
  - 文生视频模式即使素材接口不可用也能验证视频通路
  - 生成请求会计费；提交超时且结果未知时不要换幂等键重提，先查任务列表
  - 网关不提供余额/消费查询 API，扣费请到平台个人账户核对

============================================================

EOF
  exit 0
}

# ---------- 颜色 ----------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_sep()   { echo -e "${CYAN}============================================================${NC}"; }
log_title() { echo -e "\n${BOLD}${CYAN}$*${NC}"; log_sep; }

# ---------- 参数解析 ----------
MODE=""
ARG_VALUE=""
case "${1:-}" in
  --text2video|--t2v)  MODE="text2video" ;;
  --execute)           MODE="execute" ;;
  --test-asset)        MODE="test-asset" ;;
  --query-asset)       MODE="query-asset";         ARG_VALUE="${2:-}" ;;
  --delete-asset)      MODE="delete-asset";        ARG_VALUE="${2:-}" ;;
  --list-assets)       MODE="list-assets" ;;
  --preflight)         MODE="preflight";          ARG_VALUE="${2:-}" ;;
  --create-group)      MODE="create-group" ;;
  --query-group)       MODE="query-group";         ARG_VALUE="${2:-}" ;;
  --list-groups)       MODE="list-groups" ;;
  --check-models)      MODE="check-models" ;;
  --check-capabilities) MODE="check-capabilities" ;;
  --list-tasks)        MODE="list-tasks" ;;
  --fetch-ark|--fetch-task) MODE="fetch-ark";      ARG_VALUE="${2:-}" ;;
  --download-task)     MODE="download-task";       ARG_VALUE="${2:-}" ;;
  ""|-h|--help)        show_help ;;
  *) log_error "未知参数: $1"; show_help ;;
esac

case "$MODE" in
  query-asset|delete-asset)
    [ -z "$ARG_VALUE" ] && { log_error "缺少素材 ID"; echo "用法: ${0##*/} --${MODE} <asset_id>"; exit 1; } ;;
  query-group)
    [ -z "$ARG_VALUE" ] && { log_error "缺少分组 ID"; echo "用法: ${0##*/} --query-group <group_id>"; exit 1; } ;;
  preflight)
    [ -z "$ARG_VALUE" ] && { log_error "缺少素材 ID"; echo "用法: ${0##*/} --preflight <asset_id>"; exit 1; } ;;
  fetch-ark|download-task)
    [ -z "$ARG_VALUE" ] && { log_error "缺少任务 ID"; echo "用法: ${0##*/} --${MODE} <task_id>"; exit 1; } ;;
esac

# ---------- 配置区（按需修改）----------
# 海外网关基址，按文档 API_BASE 拼接到所有路径
# 注意：本脚本直连海外网关，不再兼容 NEWAPI_BASE_URL，避免误打到本地 new-api-v
API_BASE="${API_BASE:-http://106.54.45.168/seedance-gateway}"
API_KEY="${API_KEY:-sk-pPqCoPLe7pXWVNSK7OWlYmBKLYUUaa7ghZw49NIkIEzM5mi}"

# 基址合法性提示：本脚本的路径只有在网关根下才成立
case "$API_BASE" in
  */seedance-gateway) ;;
  *localhost*|*127.0.0.1*|*book2*)
    log_error "API_BASE=${API_BASE} 看起来是本地 new-api 地址"
    echo "  本脚本按 docs/海外sd 路径直连海外网关，本地 new-api 的路径不同（/api/seedance/assets 等）。"
    echo "  若确实要用本地 new-api，请使用 test-seedance-simple.sh。"
    exit 1 ;;
  *)
    log_warn "API_BASE 不以 /seedance-gateway 结尾：${API_BASE}"
    log_warn "文档要求所有路径都拼接到该前缀，请确认基址配置正确。" ;;
esac

# 视频参数（主协议字段：prompt/seconds/size 或 content/duration/ratio）
MODEL="${MODEL:-seedance-2.0-720p}"
PROMPT="${PROMPT:-蓝色与金色的抽象光影缓慢流动，女子面对镜头自然说话}"
# 注意：不要用 SECONDS 变量名，它是 bash 内建的"已运行秒数"，会把默认值吃掉
DURATION="${DURATION:-5}"
case "$DURATION" in ''|*[!0-9]*) log_error "DURATION 必须是整数秒: $DURATION"; exit 1 ;; esac
RATIO="${RATIO:-16:9}"
RESOLUTION="${RESOLUTION:-}"
SIZE="${SIZE:-1920x1080}"
N="${N:-1}"
REQUEST_FORMAT="${REQUEST_FORMAT:-content}"

# 纯文生视频开关：1/true 时忽略所有素材参数，请求体里不含任何素材引用
TEXT2VIDEO="${TEXT2VIDEO:-0}"
case "$MODE" in
  text2video) TEXT2VIDEO=1 ;;
esac
case "$TEXT2VIDEO" in
  1|true|TRUE|yes|YES) TEXT2VIDEO=1 ;;
  *) TEXT2VIDEO=0 ;;
esac

# 素材参数
ASSET_URL="${ASSET_URL:-${ROLE_IMAGE_URL:-https://static.horse-world.mints-id.com/rh/20260602024700/1780339620793_5209.png}}"
ASSET_UPLOAD_FILE="${ASSET_UPLOAD_FILE:-}"
ASSET_TITLE="${ASSET_TITLE:-os-reference}"
ASSET_MEDIA_TYPE="${ASSET_MEDIA_TYPE:-}"
ASSET_TAGS="${ASSET_TAGS:-}"
ASSET_ROLE="${ASSET_ROLE:-reference_image}"
ASSET_ID="${ASSET_ID:-}"
ASSET_GROUP_ID="${ASSET_GROUP_ID:-}"
GROUP_NAME="${GROUP_NAME:-ostest}"
GROUP_DESCRIPTION="${GROUP_DESCRIPTION:-海外网关测试素材组}"

# content 格式参考素材
IMAGE_URL="${IMAGE_URL:-}"
LAST_FRAME_URL="${LAST_FRAME_URL:-}"
VIDEO_REF_URL="${VIDEO_REF_URL:-}"
VIDEO_REF_DURATION="${VIDEO_REF_DURATION:-}"
AUDIO_REF_URL="${AUDIO_REF_URL:-}"
AUDIO_REF_DURATION="${AUDIO_REF_DURATION:-}"

# 轮询与下载
POLL_INTERVAL="${POLL_INTERVAL:-10}"
POLL_MAX="${POLL_MAX:-40}"
ASSET_POLL_INTERVAL="${ASSET_POLL_INTERVAL:-3}"
ASSET_POLL_MAX="${ASSET_POLL_MAX:-30}"
DOWNLOAD="${DOWNLOAD:-1}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

# 幂等键：重试必须复用同一个
IDEMPOTENCY_KEY="${IDEMPOTENCY_KEY:-}"

# 列表分页/过滤
PAGE="${PAGE:-1}"
PAGE_SIZE="${PAGE_SIZE:-20}"
FILTER_GROUP_ID="${FILTER_GROUP_ID:-}"
FILTER_KEYWORD="${FILTER_KEYWORD:-}"
FILTER_MEDIA_TYPE="${FILTER_MEDIA_TYPE:-}"
FILTER_STATUS="${FILTER_STATUS:-}"
FILTER_TAG="${FILTER_TAG:-}"
FILTER_MODEL="${FILTER_MODEL:-}"
FILTER_IDS="${FILTER_IDS:-}"

# ---------- 依赖检查 ----------
check_deps() {
  for cmd in curl jq; do
    command -v "$cmd" &>/dev/null || { log_error "$cmd 未安装"; exit 1; }
  done
}

require_key() {
  if [ -z "$API_KEY" ]; then
    log_error "未设置 API_KEY"
    echo "  请使用平台账户创建的 API Key（需允许目标视频模型和 media-service）："
    echo "  API_KEY=sk-xxx ${0##*/} $*"
    exit 1
  fi
}

# ---------- HTTP 辅助 ----------
# 结果写入全局 HTTP_CODE / RESP_BODY
LAST_HTTP_CODE=""
RESP_BODY=""

api_get() { # $1=path
  local tmp; tmp=$(mktemp)
  LAST_HTTP_CODE=$(curl -s -o "$tmp" -w "%{http_code}" \
    -X GET "${API_BASE}$1" \
    -H "Authorization: Bearer ${API_KEY}")
  RESP_BODY=$(cat "$tmp"); rm -f "$tmp"
}

api_post_json() { # $1=path $2=body [额外 curl 参数...]
  local path="$1" body="$2"; shift 2
  local tmp; tmp=$(mktemp)
  LAST_HTTP_CODE=$(curl -s -o "$tmp" -w "%{http_code}" \
    -X POST "${API_BASE}${path}" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" "$@" -d "$body")
  RESP_BODY=$(cat "$tmp"); rm -f "$tmp"
}

api_post_binary() { # $1=path $2=file $3=mime
  local tmp; tmp=$(mktemp)
  LAST_HTTP_CODE=$(curl -s -o "$tmp" -w "%{http_code}" \
    -X POST "${API_BASE}$1" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: $3" \
    --data-binary "@$2")
  RESP_BODY=$(cat "$tmp"); rm -f "$tmp"
}

show_resp() { echo "$RESP_BODY" | jq '.' 2>/dev/null || echo "$RESP_BODY"; }

# ---------- 工具函数 ----------
urlencode() { jq -rn --arg v "$1" '$v|@uri'; }

detect_mime() {
  local f="$1" ext_mime=""
  case "${f##*.}" in
    png) ext_mime="image/png" ;; jpg|jpeg) ext_mime="image/jpeg" ;; webp) ext_mime="image/webp" ;;
    gif) ext_mime="image/gif" ;; mp4) ext_mime="video/mp4" ;; mp3) ext_mime="audio/mpeg" ;;
    wav) ext_mime="audio/wav" ;; m4a|mp4a) ext_mime="audio/mp4" ;;
  esac
  if command -v file >/dev/null 2>&1; then
    local m; m=$(file --mime-type -b "$f" 2>/dev/null)
    # 服务端会校验真实媒体类型，识别不出时用扩展名兜底
    if [ -n "$m" ] && [ "$m" != "application/octet-stream" ]; then echo "$m"; return; fi
  fi
  [ -n "$ext_mime" ] && { echo "$ext_mime"; return; }
  echo "application/octet-stream"
}

media_type_from_mime() {
  case "$1" in
    image/*) echo "image" ;; video/*) echo "video" ;; audio/*) echo "audio" ;; *) echo "" ;;
  esac
}

media_type_from_url() {
  local path="${1%%\?*}"
  case "${path##*.}" in
    png|jpg|jpeg|webp|gif|bmp) echo "image" ;;
    mp4|mov|webm|mkv) echo "video" ;;
    mp3|wav|m4a|aac|flac|ogg) echo "audio" ;;
    *) echo "" ;;
  esac
}

# Unix 秒 -> 本地可读时间（兼容 macOS / Linux）
ts_to_date() {
  local ts="$1"
  [ -z "$ts" ] || [ "$ts" = "null" ] && { echo "N/A"; return; }
  date -r "$ts" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || date -d "@$ts" '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo "$ts"
}

# 生成的下载地址可能是相对路径，必须保留 API_BASE 的 /seedance-gateway 前缀
resolve_url() {
  local u="$1"
  case "$u" in
    http://*|https://*) echo "$u" ;;
    /*) echo "${API_BASE}${u}" ;;
    asset://*) echo "$u" ;;
    *) echo "${API_BASE}/${u}" ;;
  esac
}

build_tags_json() {
  if [ -z "$ASSET_TAGS" ]; then echo "[]"; return; fi
  echo "$ASSET_TAGS" | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' \
    | jq -R . | jq -s 'map(select(length>0))'
}

# ---------- 素材：创建 ----------
# 结果: CREATED_ASSET_ID / CREATED_ASSET_STATUS / CREATED_ASSET_REF
create_asset() {
  CREATED_ASSET_ID=""; CREATED_ASSET_STATUS=""; CREATED_ASSET_REF=""

  if [ -n "$ASSET_UPLOAD_FILE" ]; then
    # 原始字节上传，不使用 multipart/form-data
    if [ ! -f "$ASSET_UPLOAD_FILE" ]; then
      log_error "ASSET_UPLOAD_FILE 不存在: $ASSET_UPLOAD_FILE"; return 1
    fi
    local mime; mime=$(detect_mime "$ASSET_UPLOAD_FILE")
    local path="/seedance/media/upload?title=$(urlencode "$ASSET_TITLE")"
    [ -n "$ASSET_GROUP_ID" ] && path="${path}&group_id=$(urlencode "$ASSET_GROUP_ID")"

    log_info "请求: POST ${API_BASE}${path}"
    log_info "Content-Type: ${mime}（原始字节，非 multipart）"
    log_info "文件: ${ASSET_UPLOAD_FILE} ($(wc -c <"$ASSET_UPLOAD_FILE" | tr -d ' ') bytes)"

    api_post_binary "$path" "$ASSET_UPLOAD_FILE" "$mime"
  else
    # URL 导入: action=import
    if [ -z "$ASSET_URL" ]; then
      log_error "未提供素材来源：请设置 ASSET_URL 或 ASSET_UPLOAD_FILE"; return 1
    fi
    local mt="$ASSET_MEDIA_TYPE"
    [ -z "$mt" ] && mt=$(media_type_from_url "$ASSET_URL")
    [ -z "$mt" ] && mt="image"

    local tags; tags=$(build_tags_json)
    local body
    body=$(jq -n \
      --arg url "$ASSET_URL" \
      --arg mt "$mt" \
      --arg title "$ASSET_TITLE" \
      --arg gid "$ASSET_GROUP_ID" \
      --argjson tags "$tags" \
      '{action:"import",url:$url,media_type:$mt,title:$title,tags:$tags}
       + (if $gid != "" then {group_id:$gid} else {} end)')

    log_info "请求: POST ${API_BASE}/seedance/media"
    log_info "请求体 (action=import):"
    echo "$body" | jq '.'
    api_post_json "/seedance/media" "$body"
  fi

  echo ""
  log_info "HTTP ${LAST_HTTP_CODE}"
  log_info "响应体:"
  show_resp

  if [ "$LAST_HTTP_CODE" != "200" ] && [ "$LAST_HTTP_CODE" != "201" ]; then
    log_error "素材创建失败 (HTTP ${LAST_HTTP_CODE})"
    return 1
  fi

  CREATED_ASSET_ID=$(echo "$RESP_BODY" | jq -r '.id // empty' 2>/dev/null)
  CREATED_ASSET_STATUS=$(echo "$RESP_BODY" | jq -r '.status // "unknown"' 2>/dev/null)
  if [ -z "$CREATED_ASSET_ID" ]; then
    log_error "响应中没有素材 id"
    return 1
  fi
  CREATED_ASSET_REF="asset://${CREATED_ASSET_ID}"
  log_ok "素材已提交: id=${CREATED_ASSET_ID} status=${CREATED_ASSET_STATUS}"
  return 0
}

# ---------- 素材：单条查询 ----------
# 结果: ASSET_STATUS / ASSET_MEDIA_TYPE / ASSET_DURATION / ASSET_GROUP_ID_RESULT
query_asset_once() {
  local id="$1"
  id="${id#asset://}"
  local body; body=$(jq -n --arg id "$id" '{action:"get",id:$id}')
  api_post_json "/seedance/media" "$body"
}

ASSET_STATUS=""
poll_asset() {
  local id="$1" retry=0
  id="${id#asset://}"
  log_info "轮询素材: POST ${API_BASE}/seedance/media (action=get, id=${id})"
  log_info "间隔 ${ASSET_POLL_INTERVAL}s，最多 ${ASSET_POLL_MAX} 次"

  while [ "$retry" -lt "$ASSET_POLL_MAX" ]; do
    retry=$((retry + 1))
    query_asset_once "$id"
    ASSET_STATUS=$(echo "$RESP_BODY" | jq -r '.status // "unknown"' 2>/dev/null)
    log_info "#${retry} 素材状态: ${ASSET_STATUS}"

    case "$ASSET_STATUS" in
      "succeeded")
        log_ok "素材已就绪，可引用: asset://${id}"
        return 0 ;;
      "failed")
        log_error "素材处理失败"
        show_resp
        return 1 ;;
    esac
    [ "$retry" -lt "$ASSET_POLL_MAX" ] && sleep "$ASSET_POLL_INTERVAL"
  done

  log_warn "素材轮询超时，最后状态: ${ASSET_STATUS}"
  return 2
}

# ---------- 素材组：创建 ----------
create_group() {
  local name="$1"
  local body; body=$(jq -n --arg n "$name" --arg d "$GROUP_DESCRIPTION" \
    '{action:"create",name:$n,description:$d}')
  log_info "请求: POST ${API_BASE}/seedance/media/groups"
  log_info "请求体 (action=create):"
  echo "$body" | jq '.'
  api_post_json "/seedance/media/groups" "$body"
  echo ""
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp
  [ "$LAST_HTTP_CODE" != "200" ] && [ "$LAST_HTTP_CODE" != "201" ] && return 1
  ASSET_GROUP_ID=$(echo "$RESP_BODY" | jq -r '.id // empty' 2>/dev/null)
  [ -z "$ASSET_GROUP_ID" ] && return 1
  log_ok "素材组: ${ASSET_GROUP_ID}"
  return 0
}

# ---------- 视频：构建请求体 ----------
build_video_body() {
  local body

  # 文生视频：忽略一切素材引用，只保留文字与生成参数
  local a_ref="" a_image="" a_last="" a_video="" a_video_d="" a_audio="" a_audio_d=""
  if [ "$TEXT2VIDEO" != "1" ]; then
    a_ref="$ASSET_REF"; a_image="$IMAGE_URL"; a_last="$LAST_FRAME_URL"
    a_video="$VIDEO_REF_URL"; a_video_d="$VIDEO_REF_DURATION"
    a_audio="$AUDIO_REF_URL"; a_audio_d="$AUDIO_REF_DURATION"
  fi

  if [ "$REQUEST_FORMAT" = "prompt" ]; then
    # 主协议简写字段：prompt/seconds/size，不要与 content 混用
    local image_field=""
    [ -n "$a_ref" ] && image_field="$a_ref"
    [ -z "$image_field" ] && [ -n "$a_image" ] && image_field="$a_image"

    body=$(jq -n \
      --arg model "$MODEL" \
      --arg prompt "$PROMPT" \
      --argjson seconds "$DURATION" \
      --arg size "$SIZE" \
      --arg input_ref "$image_field" \
      --argjson n "$N" \
      '{model:$model,prompt:$prompt,seconds:$seconds,size:$size,n:$n}
       + (if $input_ref != "" then {input_reference:$input_ref} else {} end)')

    [ -n "$RESOLUTION" ] && body=$(echo "$body" | jq --arg r "$RESOLUTION" '. + {resolution:$r}')
  else
    # content 扩展结构：duration/ratio 不要与 prompt/seconds/size 混用
    local content
    content=$(jq -n --arg text "$PROMPT" '[{type:"text",text:$text}]')

    if [ -n "$a_ref" ]; then
      content=$(echo "$content" | jq -c --arg u "$a_ref" --arg r "$ASSET_ROLE" \
        '. + [{type:"image_url",image_url:{url:$u},role:$r}]')
    fi
    if [ -n "$a_last" ]; then
      content=$(echo "$content" | jq -c --arg u "$a_last" \
        '. + [{type:"image_url",image_url:{url:$u},role:"last_frame"}]')
    fi
    if [ -n "$a_image" ]; then
      content=$(echo "$content" | jq -c --arg u "$a_image" \
        '. + [{type:"image_url",image_url:{url:$u},role:"reference_image"}]')
    fi
    if [ -n "$a_video" ]; then
      if [ -n "$a_video_d" ]; then
        content=$(echo "$content" | jq -c --arg u "$a_video" --argjson d "$a_video_d" \
          '. + [{type:"video_url",video_url:{url:$u},role:"reference_video",duration:$d}]')
      else
        log_warn "VIDEO_REF_URL 未提供 VIDEO_REF_DURATION，接口要求声明真实时长"
        content=$(echo "$content" | jq -c --arg u "$a_video" \
          '. + [{type:"video_url",video_url:{url:$u},role:"reference_video"}]')
      fi
    fi
    if [ -n "$a_audio" ]; then
      if [ -n "$a_audio_d" ]; then
        content=$(echo "$content" | jq -c --arg u "$a_audio" --argjson d "$a_audio_d" \
          '. + [{type:"audio_url",audio_url:{url:$u},role:"reference_audio",duration:$d}]')
      else
        log_warn "AUDIO_REF_URL 未提供 AUDIO_REF_DURATION，接口要求声明真实时长"
        content=$(echo "$content" | jq -c --arg u "$a_audio" \
          '. + [{type:"audio_url",audio_url:{url:$u},role:"reference_audio"}]')
      fi
    fi

    body=$(jq -n \
      --arg model "$MODEL" \
      --argjson duration "$DURATION" \
      --arg ratio "$RATIO" \
      --argjson content "$content" \
      --argjson n "$N" \
      '{model:$model,duration:$duration,ratio:$ratio,content:$content,n:$n}')

    [ -n "$RESOLUTION" ] && body=$(echo "$body" | jq --arg r "$RESOLUTION" '. + {resolution:$r}')
  fi

  echo "$body"
}

# ---------- 视频：创建 ----------
create_video() {
  local body; body=$(build_video_body)

  [ -z "$IDEMPOTENCY_KEY" ] && \
    IDEMPOTENCY_KEY="os-$(date +%s)-${RANDOM}"
  log_info "Idempotency-Key: ${IDEMPOTENCY_KEY}（重试请复用同一个）"

  log_info "请求: POST ${API_BASE}/v1/videos"
  log_info "请求体 (${REQUEST_FORMAT} 格式):"
  echo "$body" | jq '.'
  echo ""

  api_post_json "/v1/videos" "$body" -H "Idempotency-Key: ${IDEMPOTENCY_KEY}"

  log_info "HTTP ${LAST_HTTP_CODE}"
  log_info "响应体:"
  show_resp
}

# ---------- 视频：轮询 ----------
FINAL_TASK_BODY=""
poll_video() {
  local task_id="$1" retry=0 status="unknown" progress="" storage=""

  log_info "轮询任务: GET ${API_BASE}/v1/videos/${task_id}"
  log_info "间隔 ${POLL_INTERVAL}s，最多 ${POLL_MAX} 次"

  while [ "$retry" -lt "$POLL_MAX" ]; do
    retry=$((retry + 1))
    sleep "$POLL_INTERVAL"
    api_get "/v1/videos/${task_id}"
    status=$(echo "$RESP_BODY" | jq -r '.status // "unknown"' 2>/dev/null)
    progress=$(echo "$RESP_BODY" | jq -r '.progress // ""' 2>/dev/null)
    storage=$(echo "$RESP_BODY" | jq -r '.storage.status // ""' 2>/dev/null)

    log_info "#${retry} 状态: ${status} 进度: ${progress} 保存: ${storage:-N/A}"

    case "$status" in
      "completed"|"succeeded"|"success")
        FINAL_TASK_BODY="$RESP_BODY"
        log_ok "任务完成"
        return 0 ;;
      "failed"|"error")
        log_error "任务失败"
        echo "$RESP_BODY" | jq '.error // .'
        FINAL_TASK_BODY="$RESP_BODY"
        return 1 ;;
    esac
  done

  log_warn "轮询超时，任务可能仍在处理；请用 --fetch-ark ${task_id} 复查，不要重新提交"
  FINAL_TASK_BODY="$RESP_BODY"
  return 2
}

# ---------- 视频：下载 ----------
download_output() {
  local task_id="$1" json="$2" out="$3"
  local rel
  rel=$(echo "$json" | jq -r '.url // .urls[0] // .content.video_url // .outputs[0].video_url // empty' 2>/dev/null)

  if [ -z "$rel" ]; then
    # 任务详情可能没带 url，回退到固定下载路径
    rel="/v1/videos/${task_id}/content"
    log_warn "响应无 url 字段，回退到 ${rel}"
  fi

  local full; full=$(resolve_url "$rel")
  [ -z "$out" ] && out="./seedance-os-${task_id}.mp4"

  log_info "下载: GET ${full}"
  log_info "保存到: ${out}"

  local code
  code=$(curl -s -o "$out" -w "%{http_code}" -X GET "$full" \
    -H "Authorization: Bearer ${API_KEY}")

  if [ "$code" != "200" ]; then
    log_error "下载失败 (HTTP ${code})"
    log_info "  409 video_not_ready / 404 不存在或已隐藏 / 410 content_expired / 403 无模型权限"
    return 1
  fi

  local size; size=$(wc -c <"$out" | tr -d ' ')
  log_ok "已保存 ${out} (${size} bytes)"
  return 0
}

# ---------- 前置声明 ----------
check_deps
require_key "$@"

echo ""
log_sep
echo -e "${BOLD}  Seedance 海外版测试（直连 seedance-gateway）${NC}"
log_sep
echo -e "  API_BASE:        ${API_BASE}"
echo -e "  MODEL:           ${MODEL}"
echo -e "  REQUEST_FORMAT:  ${REQUEST_FORMAT}"
echo -e "  生成模式:        $([ "$TEXT2VIDEO" = "1" ] && echo "文生视频（不使用素材）" || echo "含素材引用")"
log_sep

# ============================================================
# 查询模式：模型 / 能力
# ============================================================
if [ "$MODE" = "check-models" ]; then
  log_title "GET /v1/models"
  api_get "/v1/models"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp
  exit 0
fi

if [ "$MODE" = "check-capabilities" ]; then
  log_title "GET /seedance/capabilities"
  api_get "/seedance/capabilities"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp
  echo ""
  log_info "提示: tasks.callback=true 只表示平台已配置回调主机白名单，"
  log_info "      你的 HTTPS 接收域名仍需单独确认并完成联调，否则先用轮询。"
  exit 0
fi

# ============================================================
# 素材组模式
# ============================================================
if [ "$MODE" = "create-group" ]; then
  log_title "POST /seedance/media/groups (action=create)"
  create_group "$GROUP_NAME" || { log_error "创建素材组失败"; exit 1; }
  echo ""
  log_info "后续使用: ASSET_GROUP_ID=${ASSET_GROUP_ID} ${0##*/} --execute"
  exit 0
fi

if [ "$MODE" = "query-group" ]; then
  log_title "POST /seedance/media/groups (action=get)"
  GID="${ARG_VALUE#group://}"
  body=$(jq -n --arg id "$GID" '{action:"get",id:$id}')
  log_info "请求体:"
  echo "$body" | jq '.'
  api_post_json "/seedance/media/groups" "$body"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp

  if [ "$LAST_HTTP_CODE" = "200" ]; then
    echo ""
    log_title "分组信息"
    log_info "分组 ID: $(echo "$RESP_BODY" | jq -r '.id // ""')"
    log_info "名称:    $(echo "$RESP_BODY" | jq -r '.name // ""')"
    log_info "描述:    $(echo "$RESP_BODY" | jq -r '.description // ""')"
  fi
  exit 0
fi

if [ "$MODE" = "list-groups" ]; then
  log_title "POST /seedance/media/groups (action=list)"
  body=$(jq -n --argjson page "$PAGE" --argjson size "$PAGE_SIZE" \
    '{action:"list",page:$page,page_size:$size}')
  log_info "请求体:"
  echo "$body" | jq '.'
  api_post_json "/seedance/media/groups" "$body"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp

  if [ "$LAST_HTTP_CODE" = "200" ]; then
    echo ""
    log_title "分组列表统计"
    log_info "总数:       $(echo "$RESP_BODY" | jq -r '.total // 0')"
    log_info "当前页数量: $(echo "$RESP_BODY" | jq -r '.data | length')"
  fi
  exit 0
fi

# ============================================================
# 素材查询模式
# ============================================================
if [ "$MODE" = "query-asset" ]; then
  log_title "POST /seedance/media (action=get)"
  AID="${ARG_VALUE#asset://}"
  log_info "素材 ID: ${AID}"

  query_asset_once "$AID"
  log_info "HTTP ${LAST_HTTP_CODE}"
  log_info "响应体:"
  show_resp

  if [ "$LAST_HTTP_CODE" != "200" ]; then
    log_error "查询失败 (HTTP ${LAST_HTTP_CODE})"
    exit 1
  fi

  echo ""
  log_title "素材信息"
  log_info "素材 ID:  $(echo "$RESP_BODY" | jq -r '.id // ""')"
  log_info "标题:     $(echo "$RESP_BODY" | jq -r '.title // ""')"
  log_info "类型:     $(echo "$RESP_BODY" | jq -r '.media_type // "null"')"
  log_info "状态:     $(echo "$RESP_BODY" | jq -r '.status // "unknown"')"
  log_info "时长:     $(echo "$RESP_BODY" | jq -r '.duration // "null"')"
  log_info "分组:     $(echo "$RESP_BODY" | jq -r '.group_id // ""')"
  log_info "标签:     $(echo "$RESP_BODY" | jq -r '.tags // [] | join(",")')"

  ST=$(echo "$RESP_BODY" | jq -r '.status // ""')
  case "$ST" in
    "succeeded") log_ok "素材已就绪，引用格式: asset://$(echo "$RESP_BODY" | jq -r '.id // ""')" ;;
    "queued"|"running") log_warn "素材仍在注册中，稍后再查（只有 succeeded 可引用）" ;;
    "failed") log_error "素材处理失败" ;;
    *) log_warn "未知状态: ${ST}" ;;
  esac
  exit 0
fi

if [ "$MODE" = "delete-asset" ]; then
  log_title "POST /seedance/media (action=delete)"
  AID="${ARG_VALUE#asset://}"
  body=$(jq -n --arg id "$AID" '{action:"delete",ids:[$id]}')
  log_info "请求体:"
  echo "$body" | jq '.'
  api_post_json "/seedance/media" "$body"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp
  exit 0
fi

if [ "$MODE" = "list-assets" ]; then
  log_title "GET /seedance/media"
  path="/seedance/media?page=${PAGE}&page_size=${PAGE_SIZE}"
  [ -n "$FILTER_GROUP_ID" ]   && path="${path}&group_id=$(urlencode "$FILTER_GROUP_ID")"
  [ -n "$FILTER_KEYWORD" ]    && path="${path}&keyword=$(urlencode "$FILTER_KEYWORD")"
  [ -n "$FILTER_MEDIA_TYPE" ] && path="${path}&media_type=$(urlencode "$FILTER_MEDIA_TYPE")"
  [ -n "$FILTER_STATUS" ]     && path="${path}&status=$(urlencode "$FILTER_STATUS")"
  [ -n "$FILTER_TAG" ]        && path="${path}&tag=$(urlencode "$FILTER_TAG")"

  log_info "请求: GET ${API_BASE}${path}"
  api_get "$path"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp

  if [ "$LAST_HTTP_CODE" = "200" ]; then
    echo ""
    log_title "素材列表统计"
    log_info "总数:       $(echo "$RESP_BODY" | jq -r '.total // 0')"
    log_info "当前页数量: $(echo "$RESP_BODY" | jq -r '.data | length')"
  fi
  exit 0
fi

if [ "$MODE" = "preflight" ]; then
  log_title "POST /seedance/media/preflight（不计费）"
  AID="${ARG_VALUE#asset://}"
  body=$(jq -n \
    --arg asset "$AID" \
    --arg model "$MODEL" \
    --arg mt "$ASSET_MEDIA_TYPE" \
    --arg role "$ASSET_ROLE" \
    '{asset_id:$asset,model:$model}
     + (if $mt != "" then {media_type:$mt} else {} end)
     + (if $role != "" then {role:$role} else {} end)')
  log_info "请求体:"
  echo "$body" | jq '.'
  api_post_json "/seedance/media/preflight" "$body"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp

  if [ "$LAST_HTTP_CODE" = "200" ]; then
    echo ""
    log_info "compatibility: $(echo "$RESP_BODY" | jq -r '.compatibility // "unknown"')"
    log_info "checks:"
    echo "$RESP_BODY" | jq '.checks' 2>/dev/null
    log_warn "unverified 表示证据不足，不是已支持；本地条件通过也不等于模型一定接受。"
  fi
  exit 0
fi

# ============================================================
# 任务列表 / 主协议与旧协议对比
# ============================================================
if [ "$MODE" = "list-tasks" ]; then
  log_title "GET /v1/videos"
  path="/v1/videos?page=${PAGE}&page_size=${PAGE_SIZE}"
  # 文档: 可按 model、status、逗号分隔的 ids 过滤；page_size 最大 100
  [ -n "$FILTER_STATUS" ] && path="${path}&status=$(urlencode "$FILTER_STATUS")"
  [ -n "$FILTER_MODEL" ]  && path="${path}&model=$(urlencode "$FILTER_MODEL")"
  [ -n "$FILTER_IDS" ]    && path="${path}&ids=$(urlencode "$FILTER_IDS")"

  log_info "请求: GET ${API_BASE}${path}"
  api_get "$path"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp

  if [ "$LAST_HTTP_CODE" = "200" ]; then
    echo ""
    log_title "任务列表统计"
    log_info "总数:       $(echo "$RESP_BODY" | jq -r '.total // 0')"
    log_info "当前页数量: $(echo "$RESP_BODY" | jq -r '.data | length')"
  fi
  exit 0
fi

if [ "$MODE" = "fetch-ark" ]; then
  TASK_ID="$ARG_VALUE"

  log_title "方式 1: 主协议 GET /v1/videos/{id}"
  api_get "/v1/videos/${TASK_ID}"
  MAIN_CODE="$LAST_HTTP_CODE"; MAIN_BODY="$RESP_BODY"
  log_info "HTTP ${MAIN_CODE}"
  show_resp
  MAIN_STATUS=$(echo "$MAIN_BODY" | jq -r '.status // "N/A"' 2>/dev/null)

  log_title "方式 2: 旧协议 GET /seedance/api/v3/contents/generations/tasks/{id}"
  api_get "/seedance/api/v3/contents/generations/tasks/${TASK_ID}"
  LEGACY_CODE="$LAST_HTTP_CODE"; LEGACY_BODY="$RESP_BODY"
  log_info "HTTP ${LEGACY_CODE}"
  show_resp
  LEGACY_STATUS=$(echo "$LEGACY_BODY" | jq -r '.status // "N/A"' 2>/dev/null)

  log_title "下载地址探测"
  log_info "主协议:     $(echo "$MAIN_BODY" | jq -r '.url // .urls[0] // "N/A"' 2>/dev/null)"
  log_info "旧协议:     $(echo "$LEGACY_BODY" | jq -r '.content.video_url // "N/A"' 2>/dev/null)"
  log_info "旧下载路径: /v1/tasks/${TASK_ID}/artifacts/video/content"

  echo ""
  log_sep
  echo -e "${BOLD}查询结果对比:${NC}"
  log_sep
  echo -e "  主协议 /v1/videos/{id}:                    HTTP ${MAIN_CODE}  状态: ${MAIN_STATUS}"
  echo -e "  旧协议 /seedance/api/v3/.../tasks/{id}:    HTTP ${LEGACY_CODE}  状态: ${LEGACY_STATUS}"
  log_sep
  echo ""
  log_info "状态取值不同: 主协议 queued/in_progress/completed/failed；旧协议 queued/running/succeeded/failed。"
  log_info "新接入统一使用 /v1/videos；旧协议仅用于兼容排查。"
  log_info "回调投递状态可用: GET /seedance/callbacks/${TASK_ID}"
  exit 0
fi

if [ "$MODE" = "download-task" ]; then
  TASK_ID="$ARG_VALUE"
  log_title "GET /v1/videos/{id} 后下载 /v1/videos/{id}/content"
  api_get "/v1/videos/${TASK_ID}"
  log_info "HTTP ${LAST_HTTP_CODE}"
  show_resp

  ST=$(echo "$RESP_BODY" | jq -r '.status // ""' 2>/dev/null)
  if [ "$ST" != "completed" ] && [ "$ST" != "succeeded" ]; then
    log_error "任务未完成（status=${ST}），不下载"
    exit 1
  fi
  STORAGE=$(echo "$RESP_BODY" | jq -r '.storage.status // ""' 2>/dev/null)
  [ -n "$STORAGE" ] && [ "$STORAGE" != "ready" ] && \
    log_warn "storage.status=${STORAGE}，仅 ready 表示全部成片已保存"

  download_output "$TASK_ID" "$RESP_BODY" "$OUTPUT_FILE" || exit 1
  exit 0
fi

# ============================================================
# 仅测试素材
# ============================================================
if [ "$MODE" = "test-asset" ]; then
  log_title "素材创建测试"

  ASSET_REF=""
  if [ -n "$ASSET_ID" ]; then
    ASSET_REF="$ASSET_ID"
    case "$ASSET_REF" in asset://*) ;; *) ASSET_REF="asset://${ASSET_ID}" ;; esac
    log_ok "跳过创建，直接查询已有素材: ${ASSET_REF}"
    poll_asset "$ASSET_REF" || exit 1
    show_resp
    exit 0
  fi

  if [ -n "$ASSET_UPLOAD_FILE" ]; then
    log_info "方式: 本地文件字节上传 POST /seedance/media/upload"
  else
    log_info "方式: URL 导入 POST /seedance/media (action=import)"
  fi

  create_asset || exit 1
  poll_asset "$CREATED_ASSET_ID" || exit 1

  echo ""
  log_info "最终素材详情:"
  show_resp
  echo ""
  log_sep
  log_ok "素材创建测试成功"
  log_sep
  log_info "素材 ID:   ${CREATED_ASSET_ID}"
  log_info "引用格式:  ${CREATED_ASSET_REF}"
  echo ""
  log_info "可直接用于生成:"
  echo "  API_KEY=... ASSET_ID=${CREATED_ASSET_ID} ${0##*/} --execute"
  exit 0
fi

# ============================================================
# 提交与收尾流程（--execute 与 --text2video 共用）
# ============================================================
run_submit_flow() {
  if [ "$TEXT2VIDEO" = "1" ]; then
    log_title "Step 1 · 文生视频模式：跳过素材组与素材"
    log_info "不创建素材、不引用素材 ID，请求体只含文字与生成参数"
    ASSET_REF=""
  else
    # Step 1: 素材组
    if [ -z "$ASSET_GROUP_ID" ] && [ -n "$GROUP_NAME" ]; then
      log_title "Step 1 · 创建素材组 POST /seedance/media/groups (action=create)"
      if create_group "$GROUP_NAME"; then
        log_ok "素材将关联到分组 ${ASSET_GROUP_ID}"
      else
        log_warn "创建素材组失败，改为不分组继续"
        ASSET_GROUP_ID=""
      fi
    else
      log_title "Step 1 · 跳过素材组创建"
      [ -n "$ASSET_GROUP_ID" ] && log_info "使用已有分组: ${ASSET_GROUP_ID}" || log_info "未配置分组"
    fi

    # Step 2: 素材
    ASSET_REF=""
    if [ -n "$ASSET_ID" ]; then
      log_title "Step 2 · 使用已有素材"
      ASSET_REF="$ASSET_ID"
      case "$ASSET_REF" in asset://*) ;; *) ASSET_REF="asset://${ASSET_ID}" ;; esac
      log_ok "素材引用: ${ASSET_REF}"
    else
      log_title "Step 2 · 创建素材"
      if create_asset; then
        poll_asset "$CREATED_ASSET_ID" || exit 1
        ASSET_REF="$CREATED_ASSET_REF"
      else
        log_error "素材创建失败，退出"
        exit 1
      fi
    fi

    # Step 3: 预检查（不扣费，失败不阻塞）
    log_title "Step 3 · 素材预检查 POST /seedance/media/preflight"
    AID="${ASSET_REF#asset://}"
    if [ -n "$AID" ]; then
      pf_body=$(jq -n --arg asset "$AID" --arg model "$MODEL" --arg role "$ASSET_ROLE" \
        '{asset_id:$asset,model:$model,role:$role}')
      api_post_json "/seedance/media/preflight" "$pf_body"
      if [ "$LAST_HTTP_CODE" = "200" ]; then
        log_info "compatibility: $(echo "$RESP_BODY" | jq -r '.compatibility // "unknown"')"
        echo "$RESP_BODY" | jq '.checks' 2>/dev/null
      else
        log_warn "预检查返回 HTTP ${LAST_HTTP_CODE}，继续按实际提交校验"
      fi
    fi
  fi

  # Step 4: 创建视频
  log_title "Step 4 · 创建视频 POST /v1/videos"
  if [ "$TEXT2VIDEO" != "1" ] && [ -n "$LAST_FRAME_URL" ] && \
     { [ -n "$IMAGE_URL" ] || [ -n "$VIDEO_REF_URL" ] || [ -n "$AUDIO_REF_URL" ]; }; then
    log_warn "首尾帧不能与其他参考模式混用，请确认参数"
  fi

  SUBMIT_TS=$(date +%s)
  create_video

  TASK_ID=$(echo "$RESP_BODY" | jq -r '.id // .task_id // empty' 2>/dev/null)
  SUBMIT_STATUS=$(echo "$RESP_BODY" | jq -r '.status // "unknown"' 2>/dev/null)

  if [ -z "$TASK_ID" ]; then
    log_error "未获取到 task_id，提交失败"
    exit 1
  fi
  log_ok "task_id = ${TASK_ID}（提交状态 ${SUBMIT_STATUS}）"
  log_info "提交时间 = $(ts_to_date "$SUBMIT_TS")"
  log_info "幂等键 = ${IDEMPOTENCY_KEY}（本次业务提交如重试必须复用）"

  # Step 5: 轮询
  log_title "Step 5 · 轮询任务 GET /v1/videos/{id}"
  FINAL_TASK_BODY=""
  poll_video "$TASK_ID"
  POLL_RESULT=$?
  FINISH_TS=$(date +%s)

  if [ "$POLL_RESULT" -eq 1 ]; then
    log_error "任务失败，退出"
    exit 1
  fi

  echo ""
  log_info "最终任务详情:"
  echo "$FINAL_TASK_BODY" | jq '.'

  # Step 6: 下载
  DOWNLOADED=""
  if [ "$POLL_RESULT" -eq 0 ]; then
    case "$DOWNLOAD" in
      1|true|TRUE|yes|YES)
        log_title "Step 6 · 下载输出 GET /v1/videos/{id}/content"
        if download_output "$TASK_ID" "$FINAL_TASK_BODY" "$OUTPUT_FILE"; then
          DOWNLOADED="${OUTPUT_FILE:-./seedance-os-${TASK_ID}.mp4}"
        fi
        ;;
      *) log_info "DOWNLOAD=${DOWNLOAD}，跳过下载" ;;
    esac
  fi

  # Step 7: 报告
  log_title "Step 7 · 测试报告"
  STORAGE_STATUS=$(echo "$FINAL_TASK_BODY" | jq -r '.storage.status // "N/A"' 2>/dev/null)
  RETENTION=$(echo "$FINAL_TASK_BODY" | jq -r '.storage.retention_seconds // 604800' 2>/dev/null)
  EXPIRES_AT=$(echo "$FINAL_TASK_BODY" | jq -r '.expires_at // "null"' 2>/dev/null)
  RESULT_URL=$(echo "$FINAL_TASK_BODY" | jq -r '.url // .urls[0] // "N/A"' 2>/dev/null)

  echo ""
  echo -e "  API_BASE:        ${API_BASE}"
  echo -e "  生成模式:        $([ "$TEXT2VIDEO" = "1" ] && echo "文生视频（无素材）" || echo "含素材引用")"
  echo -e "  task_id:         ${TASK_ID}"
  echo -e "  提交状态:        ${SUBMIT_STATUS}"
  echo -e "  最终状态:        $(echo "$FINAL_TASK_BODY" | jq -r '.status // "unknown"')"
  echo -e "  耗时:            $((FINISH_TS - SUBMIT_TS))s"
  echo -e "  素材引用:        ${ASSET_REF:-未使用}"
  echo -e "  保存状态:        ${STORAGE_STATUS}（retention=${RETENTION}s）"
  echo -e "  到期时间:        $(ts_to_date "$EXPIRES_AT")"
  [ "$RESULT_URL" != "N/A" ] && echo -e "  输出地址:        $(resolve_url "$RESULT_URL")"
  [ -n "$DOWNLOADED" ] && echo -e "  本地文件:        ${DOWNLOADED}"
  echo ""
  log_sep
  if [ "$POLL_RESULT" -eq 0 ]; then
    log_ok "测试完成"
  else
    log_warn "测试结束，但任务未进入终态，请稍后用 --fetch-ark ${TASK_ID} 复查"
  fi
  log_sep
  echo ""
  log_info "计费与退款请到平台个人账户的消费记录核对（本网关不提供余额查询 API）。"
  exit 0
}

# ============================================================
# 纯文生视频 --text2video / --t2v
# ============================================================
if [ "$MODE" = "text2video" ]; then
  log_title "纯文生视频模式（不创建、不引用任何素材）"
  log_info "请求体只含文字与生成参数；素材相关环境变量在本模式下全部忽略"

  # 仍先确认模型权限，避免直接 403
  log_title "Step 0 · 确认模型权限 GET /v1/models"
  api_get "/v1/models"
  if [ "$LAST_HTTP_CODE" = "200" ]; then
    if echo "$RESP_BODY" | jq -e --arg m "$MODEL" \
        '(.data // .models // []) | map(.id // .name // .) | index($m) != null' >/dev/null 2>&1; then
      log_ok "当前 Key 可见模型包含 ${MODEL}"
    else
      log_warn "当前 Key 可见模型中没有 ${MODEL}，提交可能返回 403"
      show_resp
    fi
  else
    log_warn "模型目录查询失败 (HTTP ${LAST_HTTP_CODE})，继续"
  fi

  run_submit_flow
fi

# ============================================================
# 完整流程 --execute
# ============================================================
if [ "$MODE" = "execute" ]; then
  # Step 0: 模型权限
  log_title "Step 0 · 确认模型权限 GET /v1/models"
  api_get "/v1/models"
  if [ "$LAST_HTTP_CODE" = "200" ]; then
    if echo "$RESP_BODY" | jq -e --arg m "$MODEL" \
        '(.data // .models // []) | map(.id // .name // .) | index($m) != null' >/dev/null 2>&1; then
      log_ok "当前 Key 可见模型包含 ${MODEL}"
    else
      log_warn "当前 Key 可见模型中没有 ${MODEL}，提交可能返回 403"
      show_resp
    fi
  else
    log_warn "模型目录查询失败 (HTTP ${LAST_HTTP_CODE})，继续"
  fi

  run_submit_flow
fi

show_help
