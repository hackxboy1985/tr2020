#!/bin/bash
#
# 真人认证链路测试脚本 —— 直连上游（咪咕 Seedance Gateway :9444）
#
# 用途：
#   不经过 new-api，直接用 NewAPI Token 调上游 Gateway，验证上游行为。
#   与 test-face-verification-newapi.sh 配对使用，两边输出应保持一致。
#
# 对应上游文档：docs/咪咕/中教.md 第 7、8 节
#   POST /api/seedance/face-verifications          创建真人认证任务
#   GET  /api/seedance/face-verifications/{id}     查询认证任务
#   POST /api/seedance/proxy/assets/groups         创建素材组
#   POST /api/seedance/proxy/assets                创建素材（真人组 / AIGC 组）
#   GET  /api/seedance/proxy/assets/{id}           查询素材状态
#
# 用法：
#   ./test-face-verification-upstream.sh                 # 只创建认证任务，打印 h5_url 后退出
#   ./test-face-verification-upstream.sh --status <id>   # 查询指定 verification_id
#   ./test-face-verification-upstream.sh --watch <id>    # 轮询直到 verified/failed/expired
#   ./test-face-verification-upstream.sh --full          # 创建 + 轮询 + 上传真人素材 + 等 Active
#   ./test-face-verification-upstream.sh --expired-probe <id>  # 探测过期会话上游是否仍可查
#   ./test-face-verification-upstream.sh --groups        # 查真人素材组列表（GroupType=LivenessFace）
#   ./test-face-verification-upstream.sh --assets <group>  # 查真人素材列表
#
# 环境变量：
#   GATEWAY_BASE_URL   上游 Gateway 地址，默认 https://sd.dawnloadai.com:9444
#   API_KEY            NewAPI Token（上游与 new-api 共用同一枚）
#   FACE_IMAGE_URL     真人素材图片，必须是上游可访问的公网 HTTPS 地址
#   RETURN_URL         认证完成后跳回的客户页面（可选）
#   POLL_INTERVAL      轮询间隔秒，默认 3
#   POLL_TIMEOUT       轮询超时秒，默认 600
#
set -uo pipefail

GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-https://sd.dawnloadai.com:9444}"
API_KEY="${API_KEY:-}"
FACE_IMAGE_URL="${FACE_IMAGE_URL:-https://static.horse-world.mints-id.com/gy/general/1/image/2026-07-08/1783497394035_4147.png}"
RETURN_URL="${RETURN_URL:-}"
POLL_INTERVAL="${POLL_INTERVAL:-3}"
POLL_TIMEOUT="${POLL_TIMEOUT:-600}"

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'

UPSTREAM="upstream"

# ---------- 基础工具 ----------

usage() {
  sed -n '3,26p' "$0" | sed 's/^# \{0,1\}//'
}

need_tools() {
  for t in curl jq; do
    if ! command -v "$t" >/dev/null 2>&1; then
      echo -e "${RED}错误: 缺少依赖 $t${NC}" >&2
      exit 1
    fi
  done
}

need_key() {
  if [ -z "$API_KEY" ]; then
    echo -e "${RED}错误: 未设置 API_KEY${NC}" >&2
    echo "用法: API_KEY=sk-xxx $0 $*" >&2
    exit 1
  fi
}

ts() { date '+%Y-%m-%d %H:%M:%S'; }

section() {
  echo ""
  echo -e "${GREEN}======================================================"
  echo "[$UPSTREAM] $1"
  echo -e "======================================================${NC}"
}

# 打印 JSON，jq 不可用时原样输出
pj() { if command -v jq >/dev/null 2>&1; then jq '.' 2>/dev/null <<<"$1" || printf '%s\n' "$1"; else printf '%s\n' "$1"; fi; }

# 统一的 HTTP 调用
# 用法: http_call <method> <path> [json_body]
# 输出: 全局变量 HTTP_CODE / HTTP_BODY
http_call() {
  local method="$1" path="$2" body="${3:-}"
  local url="${GATEWAY_BASE_URL%/}${path}"
  local args=(-sS -X "$method" "$url"
    -H "Authorization: Bearer ${API_KEY}"
    -H "Content-Type: application/json"
    -H "Accept: application/json"
    -w $'\n__HTTP_CODE__%{http_code}')
  [ -n "$body" ] && args+=(-d "$body")

  echo -e "${CYAN}--> ${method} ${url}${NC}" >&2
  [ -n "$body" ] && { echo "请求体: $body" >&2; }

  local raw
  raw="$(curl "${args[@]}" 2>&1)"
  HTTP_CODE="$(printf '%s' "$raw" | sed -n 's/^__HTTP_CODE__//p' | tail -1)"
  HTTP_BODY="$(printf '%s' "$raw" | sed '$d')"

  echo -e "${CYAN}<-- HTTP ${HTTP_CODE}${NC}" >&2
  pj "$HTTP_BODY" >&2
}

# 把 {"success":true,...} 与裸 JSON 统一成裸 JSON
unwrap() {
  local body="$1"
  if jq -e 'has("data") and has("success")' >/dev/null 2>&1 <<<"$body"; then
    jq -c '.data' <<<"$body"
  else
    printf '%s' "$body"
  fi
}

# HTTP 是否成功：按 2xx 判定。
# 注意：上游创建认证任务实测返回 201 Created（不是 200），不要写死 ==200。
# new-api 自己也是按 2xx 判定的（controller/seedance.go: statusCode < 200 || >= 300）。
is_2xx() { [ "${1:-0}" -ge 200 ] 2>/dev/null && [ "${1:-0}" -lt 300 ] 2>/dev/null; }

# 失败时打印 HTTP 码与原始响应，便于定位
fail_http() {
  echo -e "${RED}$1（HTTP ${HTTP_CODE}）${NC}" >&2
  pj "$HTTP_BODY" >&2
  return 1
}

show_field() {
  local json="$1" label="$2" filter="$3"
  local v
  v="$(jq -r "$filter // empty" 2>/dev/null <<<"$json")"
  if [ -n "$v" ]; then
    printf '  %-16s %s\n' "$label" "$v"
  else
    printf '  %-16s %s\n' "$label" "(未返回)"
  fi
}

# ---------- 业务操作 ----------

# 创建真人认证任务
op_create() {
  section "创建真人认证任务"

  local body
  if [ -n "$RETURN_URL" ]; then
    body="$(jq -nc --arg u "$RETURN_URL" '{return_url:$u}')"
  else
    body='{}'
  fi

  http_call POST "/api/seedance/face-verifications" "$body"
  is_2xx "$HTTP_CODE" || { fail_http "创建失败"; return 1; }

  local json; json="$(unwrap "$HTTP_BODY")"
  echo ""
  echo -e "${GREEN}创建成功（HTTP ${HTTP_CODE}）：${NC}"
  show_field "$json" "verification_id" '.verification_id'
  show_field "$json" "status"          '.status'
  show_field "$json" "expires_at"      '.expires_at'
  show_field "$json" "request_id"      '.request_id'
  echo ""
  echo -e "${YELLOW}h5_url（交给终端用户打开完成活体认证）：${NC}"
  jq -r '.h5_url // "(未返回)"' <<<"$json"

  local vid; vid="$(jq -r '.verification_id // empty' <<<"$json")"
  [ -n "$vid" ] && echo "" && echo -e "${GREEN}VERIFICATION_ID=${vid}${NC}"
  return 0
}

# 查询认证任务一次
# 返回: 0=verified 1=failed/expired 2=进行中 3=查询异常
probe_status() {
  local vid="$1"
  http_call GET "/api/seedance/face-verifications/${vid}"

  if ! is_2xx "$HTTP_CODE"; then
    echo -e "${RED}查询失败 HTTP ${HTTP_CODE}${NC}" >&2
    pj "$HTTP_BODY" >&2
    LAST_STATUS=""
    LAST_GROUP=""
    return 3
  fi

  local json; json="$(unwrap "$HTTP_BODY")"
  LAST_STATUS="$(jq -r '.status // empty' <<<"$json")"
  LAST_GROUP="$(jq -r '.group_id // empty' <<<"$json")"

  echo ""
  echo -e "${GREEN}[$(ts)] status=${LAST_STATUS}${NC}"
  show_field "$json" "group_id"   '.group_id'
  show_field "$json" "h5_url"     '.h5_url'
  show_field "$json" "expires_at" '.expires_at'

  case "$LAST_STATUS" in
    verified) return 0 ;;
    failed|expired) return 1 ;;
    *) return 2 ;;
  esac
}

op_status() {
  section "查询认证任务 $1"
  probe_status "$1"
  local rc=$?
  case $rc in
    0) echo -e "\n${GREEN}结果: verified${NC}" ;;
    1) echo -e "\n${RED}结果: ${LAST_STATUS}${NC}" ;;
    2) echo -e "\n${YELLOW}结果: 进行中 (${LAST_STATUS})${NC}" ;;
  esac
}

op_watch() {
  section "轮询认证任务 $1（间隔 ${POLL_INTERVAL}s，超时 ${POLL_TIMEOUT}s）"
  echo -e "${YELLOW}请手动打开上面的 h5_url 完成认证，本脚本只负责等待。${NC}"

  local start now rc
  start="$(date +%s)"
  while true; do
    probe_status "$1"; rc=$?
    now="$(date +%s)"

    case $rc in
      0)
        echo ""
        echo -e "${GREEN}=========================================="
        echo "认证成功"
        echo "  verification_id = $1"
        echo "  group_id        = ${LAST_GROUP}"
        echo -e "==========================================${NC}"
        [ -n "$LAST_GROUP" ] && echo -e "${GREEN}GROUP_ID=${LAST_GROUP}${NC}"
        return 0
        ;;
      1)
        echo -e "${RED}认证结束且未通过: ${LAST_STATUS}${NC}"
        return 1
        ;;
    esac

    if [ $((now - start)) -ge "$POLL_TIMEOUT" ]; then
      echo -e "${RED}轮询超时（${POLL_TIMEOUT}s），最后状态: ${LAST_STATUS}${NC}"
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

# 向指定素材组上传素材
op_upload() {
  local group_id="$1" url="$2" name="${3:-verified-face}"

  section "向素材组 ${group_id} 上传素材"
  echo "URL       = ${url}"
  [ -z "$group_id" ] && { echo -e "${RED}需要 group_id${NC}"; return 1; }

  local body
  body="$(jq -nc --arg g "$group_id" --arg u "$url" --arg n "$name" \
    '{GroupId:$g,URL:$u,AssetType:"Image",Name:$n}')"

  http_call POST "/api/seedance/proxy/assets" "$body"
  is_2xx "$HTTP_CODE" || { fail_http "上传失败"; return 1; }

  local json; json="$(unwrap "$HTTP_BODY")"
  LAST_ASSET_ID="$(jq -r '.Result.Id // .Id // empty' <<<"$json")"
  LAST_ASSET_STATUS="$(jq -r '.Result.Status // .Status // empty' <<<"$json")"

  echo ""
  echo -e "${GREEN}上传已提交：${NC}"
  show_field "$json" "asset_id" "$(printf '%s' '.Result.Id // .Id')"
  show_field "$json" "status"   '.Result.Status // .Status'
  [ -n "$LAST_ASSET_ID" ] && echo -e "${GREEN}ASSET_ID=${LAST_ASSET_ID}${NC}"
  return 0
}

# 轮询素材直到 Active
op_wait_asset() {
  local aid="$1"
  section "轮询素材 $aid 直到 Active"

  local start now
  start="$(date +%s)"
  while true; do
    http_call GET "/api/seedance/proxy/assets/${aid}"
    if ! is_2xx "$HTTP_CODE"; then
      echo -e "${RED}查询素材失败 HTTP ${HTTP_CODE}${NC}"
      pj "$HTTP_BODY"
      return 1
    fi
    local json st
    json="$(unwrap "$HTTP_BODY")"
    st="$(jq -r '.Result.Status // .Status // empty' <<<"$json")"
    echo -e "${GREEN}[$(ts)] asset status=${st}${NC}"

    case "$st" in
      Active)
        echo ""
        echo -e "${GREEN}素材可用，视频生成引用值: asset://${aid}${NC}"
        return 0
        ;;
      Failed)
        echo -e "${RED}素材处理失败${NC}"
        pj "$json"
        return 1
        ;;
    esac

    now="$(date +%s)"
    if [ $((now - start)) -ge "$POLL_TIMEOUT" ]; then
      echo -e "${RED}轮询超时，最后状态: ${st}${NC}"
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

op_groups() {
  section "查询真人素材组（GroupType=LivenessFace）"
  http_call GET "/api/seedance/proxy/assets/groups?GroupType=LivenessFace&PageNumber=1&PageSize=20"
}

op_assets() {
  section "查询真人素材（group=${1:-全部}）"
  if [ -n "${1:-}" ]; then
    http_call GET "/api/seedance/proxy/assets?GroupType=LivenessFace&PageNumber=1&PageSize=20&GroupId=$1"
  else
    http_call GET "/api/seedance/proxy/assets?GroupType=LivenessFace&PageNumber=1&PageSize=20"
  fi
}

# 探测：认证过期 / 完成后，上游是否仍返回历史记录
# 这是为了确认 new-api 是否需要做本地结果兜底
op_expired_probe() {
  section "探测会话 ${1} 在上游的可查询性"
  echo "观察点："
  echo "  1) 长时间未认证的会话，上游是否一直返回 waiting_user"
  echo "  2) 返回 expired 后，再次查询是否仍返回 expired（而不是 404）"
  echo "  3) 返回 verified 后，再次查询是否仍返回同一个 group_id"
  echo ""
  local i
  for i in 1 2 3; do
    echo -e "${BLUE}--- 第 ${i} 次探测 ---${NC}"
    probe_status "$1"
    echo "  HTTP=${HTTP_CODE} status=${LAST_STATUS} group_id=${LAST_GROUP}"
    [ "$i" -lt 3 ] && sleep 2
  done
  echo ""
  if [ "$HTTP_CODE" = "404" ]; then
    echo -e "${RED}结论: 上游对已结束会话返回 404 → new-api 必须做本地结果兜底${NC}"
  else
    echo -e "${GREEN}结论: 上游仍可查询已结束会话（HTTP ${HTTP_CODE}）${NC}"
  fi
}

op_full() {
  section "完整流程：创建 -> 认证 -> 上传真人素材 -> 等 Active"

  op_create || return 1
  local vid="$LAST_VERIFICATION_ID"
  [ -z "$vid" ] && vid="$(echo "$HTTP_BODY" | jq -r '.verification_id // empty')"
  [ -z "$vid" ] && { echo -e "${RED}未取到 verification_id${NC}"; return 1; }

  echo ""
  echo -e "${YELLOW}>>> 请立即打开 h5_url 完成真人认证 <<<${NC}"
  echo ""

  op_watch "$vid" || return 1

  local gid="$LAST_GROUP"
  [ -z "$gid" ] && { echo -e "${RED}认证成功但未返回 group_id，无法继续${NC}"; return 1; }

  op_upload "$gid" "$FACE_IMAGE_URL" || return 1
  op_wait_asset "$LAST_ASSET_ID" || return 1

  section "完成"
  echo "verification_id = $vid"
  echo "group_id        = $gid"
  echo "asset_id        = $LAST_ASSET_ID"
  echo "视频生成引用值  = asset://$LAST_ASSET_ID"
}

# ---------- 主入口 ----------

need_tools
need_key
LAST_STATUS=""; LAST_GROUP=""; LAST_ASSET_ID=""; LAST_ASSET_STATUS=""; LAST_VERIFICATION_ID=""

case "${1:-}" in
  ""|--create)                     op_create ;;
  --status)   [ -z "${2:-}" ] && { echo "需要 verification_id" >&2; exit 1; }; op_status "$2" ;;
  --watch)    [ -z "${2:-}" ] && { echo "需要 verification_id" >&2; exit 1; }; op_watch "$2" ;;
  --upload)   [ -z "${2:-}" ] && { echo "需要 group_id" >&2; exit 1; }; op_upload "$2" "${3:-$FACE_IMAGE_URL}" ;;
  --wait-asset) [ -z "${2:-}" ] && { echo "需要 asset_id" >&2; exit 1; }; op_wait_asset "$2" ;;
  --groups)   op_groups ;;
  --assets)   op_assets "${2:-}" ;;
  --expired-probe) [ -z "${2:-}" ] && { echo "需要 verification_id" >&2; exit 1; }; op_expired_probe "$2" ;;
  --full)     op_full ;;
  help|-h|--help) usage ;;
  *) echo -e "${RED}未知命令: $1${NC}" >&2; usage; exit 1 ;;
esac
