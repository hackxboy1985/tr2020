#!/bin/bash
#
# 真人认证链路测试脚本 —— 经由 new-api（中转）
#
# 用途：
#   走 new-api 的 /api/seedance/* 接口，验证中转链路（鉴权、渠道选择、本地落库、响应格式）。
#   与 test-face-verification-upstream.sh 配对使用：同一个 verification_id 应可在两边互相印证。
#
# new-api 接口（router/seedance-router.go）：
#   POST /api/seedance/face-verifications                创建认证任务
#   GET  /api/seedance/face-verifications/{id}           查询认证任务（支持 fv_xxx 或本地数字 ID）
#   GET  /api/seedance/face-verifications                本地认证任务列表
#   POST /api/seedance/asset-groups                      创建素材组
#   GET  /api/seedance/assets?group_id=xxx               本地素材列表
#   POST /api/seedance/assets                            创建素材（GroupId 传真人组）
#   GET  /api/seedance/assets/{id}                       查询素材状态（支持 asset-xxx 或本地数字 ID）
#   POST /api/seedance/assets/v2/?Action=XXX             火山 Action 风格（等价入口）
#
# 用法：
#   ./test-face-verification-newapi.sh                    # 只创建认证任务，打印 h5_url 后退出
#   ./test-face-verification-newapi.sh --status <id>      # 查询（id 可用 fv_xxx / 本地数字 ID）
#   ./test-face-verification-newapi.sh --watch <id>       # 轮询直到 verified/failed/expired
#   ./test-face-verification-newapi.sh --full             # 创建 + 轮询 + 上传真人素材 + 等 Active
#   ./test-face-verification-newapi.sh --list             # 本地认证任务列表
#   ./test-face-verification-newapi.sh --local <id>       # 只看本地落库结果（对照上游是否一致）
#   ./test-face-verification-newapi.sh --upload <group> [url] [--model m]  # 向真人组上传素材
#   ./test-face-verification-newapi.sh --upload-nogroup <url>              # 不传 GroupId（走默认 AIGC 组）
#   ./test-face-verification-newapi.sh --groups           # 本地素材组列表
#   ./test-face-verification-newapi.sh --assets [group]   # 本地素材列表
#   ./test-face-verification-newapi.sh --action-create    # Action 风格创建认证
#   ./test-face-verification-newapi.sh --action-status <id>  # Action 风格查询认证（body 传 SessionId）
#   ./test-face-verification-newapi.sh --matrix           # 端到端矩阵：AIGC 组 / 真人组 / 无组 三种上传
#
# 环境变量：
#   BASE_URL           new-api 地址，默认 http://127.0.0.1:3000
#   API_KEY            new-api 令牌（与直连上游脚本是同一枚）
#   FACE_IMAGE_URL     真人素材图片，必须是上游可访问的公网 HTTPS 地址
#   AIGC_IMAGE_URL     对照用的普通素材图片
#   RETURN_URL         认证完成后跳回的客户页面（可选）
#   MODEL              可选，上传素材时按模型选择渠道
#   POLL_INTERVAL      轮询间隔秒，默认 3
#   POLL_TIMEOUT       轮询超时秒，默认 600
#
set -uo pipefail

BASE_URL="${BASE_URL:-http://book2:3000}"
API_KEY="${API_KEY:-}"
FACE_IMAGE_URL="${FACE_IMAGE_URL:-https://static.horse-world.mints-id.com/gy/general/1/image/2026-07-08/1783497394035_4147.png}"
AIGC_IMAGE_URL="${AIGC_IMAGE_URL:-$FACE_IMAGE_URL}"
RETURN_URL="${RETURN_URL:-}"
MODEL="${MODEL:-}"
POLL_INTERVAL="${POLL_INTERVAL:-3}"
POLL_TIMEOUT="${POLL_TIMEOUT:-600}"

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'

UPSTREAM="new-api"

usage() { sed -n '3,35p' "$0" | sed 's/^# \{0,1\}//'; }

need_tools() {
  for t in curl jq; do
    command -v "$t" >/dev/null 2>&1 || { echo -e "${RED}错误: 缺少依赖 $t${NC}" >&2; exit 1; }
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

pj() { jq '.' 2>/dev/null <<<"$1" || printf '%s\n' "$1"; }

# 用法: http_call <method> <path> [json_body]
# 输出: HTTP_CODE / HTTP_BODY
http_call() {
  local method="$1" path="$2" body="${3:-}"
  local url="${BASE_URL%/}${path}"
  local args=(-sS -X "$method" "$url"
    -H "Authorization: Bearer ${API_KEY}"
    -H "Content-Type: application/json"
    -H "Accept: application/json"
    -w $'\n__HTTP_CODE__%{http_code}')
  [ -n "$body" ] && args+=(-d "$body")

  echo -e "${CYAN}--> ${method} ${url}${NC}" >&2
  [ -n "$body" ] && echo "请求体: $body" >&2

  local raw
  raw="$(curl "${args[@]}" 2>&1)"
  HTTP_CODE="$(printf '%s' "$raw" | sed -n 's/^__HTTP_CODE__//p' | tail -1)"
  HTTP_BODY="$(printf '%s' "$raw" | sed '$d')"

  echo -e "${CYAN}<-- HTTP ${HTTP_CODE}${NC}" >&2
  pj "$HTTP_BODY" >&2
}

# 把 new-api 的 {success,message,data} 与裸 JSON 统一
unwrap() {
  local body="$1"
  if jq -e 'has("data") and has("success")' >/dev/null 2>&1 <<<"$body"; then
    jq -c '.data' <<<"$body"
  else
    printf '%s' "$body"
  fi
}

# HTTP 是否成功：按 2xx 判定，不要写死 ==200。
# 上游创建认证任务实测返回 201 Created；new-api 也是按 2xx 判定的
# （controller/seedance.go: statusCode < 200 || statusCode >= 300 才算错）。
is_2xx() { [ "${1:-0}" -ge 200 ] 2>/dev/null && [ "${1:-0}" -lt 300 ] 2>/dev/null; }

# 失败时打印 HTTP 码与原始响应，便于定位
fail_http() {
  echo -e "${RED}$1（HTTP ${HTTP_CODE}）${NC}" >&2
  pj "$HTTP_BODY" >&2
  return 1
}

# new-api 的错误：HTTP 200 + success=false，或非 2xx
check_ok() {
  local body="$1"
  local ok
  ok="$(jq -r 'if type=="object" and has("success") then (.success|tostring) else "true" end' 2>/dev/null <<<"$body")"
  if [ "$ok" = "false" ]; then
    echo -e "${RED}new-api 返回失败: $(jq -r '.message // "unknown"' <<<"$body")${NC}" >&2
    return 1
  fi
  return 0
}

show_field() {
  local json="$1" label="$2" filter="$3" v
  v="$(jq -r "$filter // empty" 2>/dev/null <<<"$json")"
  if [ -n "$v" ]; then printf '  %-18s %s\n' "$label" "$v"
  else printf '  %-18s %s\n' "$label" "(未返回)"; fi
}

# ---------- 业务操作 ----------

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
  check_ok "$HTTP_BODY" || return 1

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

  LAST_VERIFICATION_ID="$(jq -r '.verification_id // empty' <<<"$json")"
  [ -n "$LAST_VERIFICATION_ID" ] && echo "" && echo -e "${GREEN}VERIFICATION_ID=${LAST_VERIFICATION_ID}${NC}"

  # 创建后立刻回查本地列表，验证是否已落库
  echo ""
  echo -e "${BLUE}--- 校验本地是否已落库（GET /api/seedance/face-verifications）---${NC}"
  op_list_quiet
}

op_list_quiet() {
  http_call GET "/api/seedance/face-verifications?p=1&page_size=5"
  local json; json="$(unwrap "$HTTP_BODY")"
  jq -r '.items[]? | "  id=\(.id) verification_id=\(.verification_id) status=\(.status) group_id=\(.group_id // "-")"' <<<"$json" 2>/dev/null
}

op_list() {
  section "本地认证任务列表"
  http_call GET "/api/seedance/face-verifications?p=1&page_size=20"
  local json; json="$(unwrap "$HTTP_BODY")"
  jq '.' <<<"$json"
}

# 查询一次，返回 0=verified 1=failed/expired 2=进行中 3=异常
probe_status() {
  local id="$1"
  http_call GET "/api/seedance/face-verifications/${id}"
  if ! is_2xx "$HTTP_CODE"; then
    echo -e "${RED}查询失败 HTTP ${HTTP_CODE}${NC}" >&2
    pj "$HTTP_BODY" >&2
    LAST_STATUS=""; LAST_GROUP=""
    return 3
  fi
  check_ok "$HTTP_BODY" || { LAST_STATUS=""; LAST_GROUP=""; return 3; }

  local json; json="$(unwrap "$HTTP_BODY")"
  LAST_STATUS="$(jq -r '.status // empty' <<<"$json")"
  LAST_GROUP="$(jq -r '.group_id // empty' <<<"$json")"

  echo ""
  echo -e "${GREEN}[$(ts)] status=${LAST_STATUS}${NC}"
  show_field "$json" "group_id"   '.group_id'
  show_field "$json" "expires_at" '.expires_at'
  show_field "$json" "h5_url"     '.h5_url'

  case "$LAST_STATUS" in
    verified) return 0 ;;
    failed|expired) return 1 ;;
    *) return 2 ;;
  esac
}

op_status() {
  section "查询认证任务 $1"
  probe_status "$1"; local rc=$?
  case $rc in
    0) echo -e "\n${GREEN}结果: verified（group_id=${LAST_GROUP}）${NC}" ;;
    1) echo -e "\n${RED}结果: ${LAST_STATUS}${NC}" ;;
    2) echo -e "\n${YELLOW}结果: 进行中 (${LAST_STATUS})${NC}" ;;
  esac
}

op_watch() {
  section "轮询认证任务 $1（间隔 ${POLL_INTERVAL}s，超时 ${POLL_TIMEOUT}s）"
  echo -e "${YELLOW}请手动打开 h5_url 完成认证，本脚本只负责等待。${NC}"

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
        return 0 ;;
      1) echo -e "${RED}认证结束且未通过: ${LAST_STATUS}${NC}"; return 1 ;;
    esac
    if [ $((now - start)) -ge "$POLL_TIMEOUT" ]; then
      echo -e "${RED}轮询超时（${POLL_TIMEOUT}s），最后状态: ${LAST_STATUS}${NC}"; return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

# 只看本地落库（不触发上游查询），用于对照
op_local() {
  section "本地落库结果 $1"
  echo "说明：本命令只读本地列表，不调用上游。"
  echo "     若这里 group_id 为空、而 --status 能拿到 group_id，说明「查询时未回写本地」。"
  echo ""
  http_call GET "/api/seedance/face-verifications?p=1&page_size=50"
  local json; json="$(unwrap "$HTTP_BODY")"
  echo -e "${BLUE}匹配 $1 的记录：${NC}"
  jq -r --arg id "$1" '
    .items[]? | select((.verification_id|tostring)==$id or (.id|tostring)==$id)
    | {id, verification_id, status, group_id, h5_url, expires_at, raw_data}
  ' <<<"$json"
  local n
  n="$(jq -r --arg id "$1" '[.items[]? | select((.verification_id|tostring)==$id or (.id|tostring)==$id)] | length' <<<"$json")"
  if [ "$n" = "0" ]; then
    echo -e "${RED}本地无该记录 → 查询接口会直接 404（resolveFaceVerification 只查本地表）${NC}"
  else
    echo -e "${GREEN}本地有记录${NC}"
  fi
}

# 上传素材
# 用法: do_upload <group_id|""> <url> <name>
do_upload() {
  local gid="$1" url="$2" name="$3"
  local body
  if [ -n "$MODEL" ]; then
    body="$(jq -nc --arg g "$gid" --arg u "$url" --arg n "$name" --arg m "$MODEL" \
      '{URL:$u,AssetType:"Image",Name:$n,Model:$m} | if $g=="" then . else .+{GroupId:$g} end')"
  else
    body="$(jq -nc --arg g "$gid" --arg u "$url" --arg n "$name" \
      '{URL:$u,AssetType:"Image",Name:$n} | if $g=="" then . else .+{GroupId:$g} end')"
  fi

  http_call POST "/api/seedance/assets" "$body"
  is_2xx "$HTTP_CODE" || { fail_http "上传请求失败"; return 1; }
  check_ok "$HTTP_BODY" || return 1

  local json; json="$(unwrap "$HTTP_BODY")"
  LAST_ASSET_ID="$(jq -r '.Id // .Result.Id // empty' <<<"$json")"
  LAST_ASSET_GROUP="$(jq -r '.GroupId // .Result.GroupId // empty' <<<"$json")"
  LAST_ASSET_STATUS="$(jq -r '.Status // .Result.Status // empty' <<<"$json")"

  echo ""
  echo -e "${GREEN}上传已提交：${NC}"
  show_field "$json" "asset_id" '.Id // .Result.Id'
  show_field "$json" "status"   '.Status // .Result.Status'
  show_field "$json" "GroupId"  '.GroupId // .Result.GroupId'
  echo -e "${BLUE}  请求时传入 GroupId = ${gid:-（未传，走默认 AIGC 组）}${NC}"
  if [ -n "$gid" ] && [ -n "$LAST_ASSET_GROUP" ] && [ "$gid" != "$LAST_ASSET_GROUP" ]; then
    echo -e "${RED}  ！！注意：上游回传的 GroupId 与请求不一致（请求 ${gid} / 返回 ${LAST_ASSET_GROUP}）"
    echo -e "     这通常意味着触发了 controller 里的「group 不存在就重建默认组」分支，素材被写进了别的组。${NC}"
  fi
  [ -n "$LAST_ASSET_ID" ] && echo -e "${GREEN}ASSET_ID=${LAST_ASSET_ID}${NC}"
}

op_upload() {
  local gid="$1"; shift
  local url="${1:-$FACE_IMAGE_URL}"
  section "向真人组上传素材（groupId=${gid}）"
  do_upload "$gid" "$url" "verified-face"
}

op_upload_nogroup() {
  local url="${1:-$AIGC_IMAGE_URL}"
  section "上传素材但不传 GroupId（走默认 AIGC 组）"
  do_upload "" "$url" "no-group-asset"
}

op_wait_asset() {
  local aid="$1"
  section "轮询素材 $aid 直到 Active"
  local start now
  start="$(date +%s)"
  while true; do
    http_call GET "/api/seedance/assets/${aid}"
    if ! is_2xx "$HTTP_CODE"; then
      echo -e "${RED}查询素材失败 HTTP ${HTTP_CODE}${NC}"
      pj "$HTTP_BODY"
      return 1
    fi
    local json st
    json="$(unwrap "$HTTP_BODY")"
    st="$(jq -r '.Status // .Result.Status // empty' <<<"$json")"
    echo -e "${GREEN}[$(ts)] asset status=${st}${NC}"
    case "$st" in
      Active) echo ""; echo -e "${GREEN}素材可用，视频生成引用值: asset://${aid}${NC}"; return 0 ;;
      Failed) echo -e "${RED}素材处理失败${NC}"; pj "$json"; return 1 ;;
    esac
    now="$(date +%s)"
    [ $((now - start)) -ge "$POLL_TIMEOUT" ] && { echo -e "${RED}轮询超时，最后状态: ${st}${NC}"; return 1; }
    sleep "$POLL_INTERVAL"
  done
}

op_groups() {
  section "本地素材组列表"
  http_call GET "/api/seedance/asset-groups?p=1&page_size=20"
  echo -e "${YELLOW}提示：现阶段 GroupType 参数被忽略，真人组（LivenessFace）不会出现在这里，"
  echo -e "      它只存在于认证记录的 group_id 字段中。${NC}"
}

op_assets() {
  section "本地素材列表（group=${1:-全部}）"
  if [ -n "${1:-}" ]; then
    http_call GET "/api/seedance/assets?p=1&page_size=20&group_id=$1"
  else
    http_call GET "/api/seedance/assets?p=1&page_size=20"
  fi
}

# Action 风格：创建认证
op_action_create() {
  section "Action 风格创建认证任务"
  local body='{}'
  [ -n "$RETURN_URL" ] && body="$(jq -nc --arg u "$RETURN_URL" '{return_url:$u}')"
  http_call POST "/api/seedance/assets/v2/?Action=CreateVisualValidateSession&Version=2024-01-01" "$body"
}

# Action 风格：查询认证（body 传 SessionId）
op_action_status() {
  section "Action 风格查询认证任务 $1"
  local body; body="$(jq -nc --arg s "$1" '{SessionId:$s}')"
  http_call POST "/api/seedance/assets/v2/?Action=GetVisualValidateResult&Version=2024-01-01" "$body"
}

op_full() {
  section "完整流程：创建 -> 认证 -> 上传真人素材 -> 等 Active"
  op_create || return 1
  local vid="$LAST_VERIFICATION_ID"
  [ -z "$vid" ] && { echo -e "${RED}未取到 verification_id${NC}"; return 1; }

  echo ""
  echo -e "${YELLOW}>>> 请立即打开 h5_url 完成真人认证 <<<${NC}"

  op_watch "$vid" || return 1
  local gid="$LAST_GROUP"
  [ -z "$gid" ] && { echo -e "${RED}认证成功但未返回 group_id${NC}"; return 1; }

  op_upload "$gid" "$FACE_IMAGE_URL" || return 1
  op_wait_asset "$LAST_ASSET_ID" || return 1

  echo ""
  echo -e "${BLUE}--- 对照：本地落库是否记录了 group_id ---${NC}"
  op_local "$vid"

  section "完成"
  echo "verification_id = $vid"
  echo "group_id        = $gid"
  echo "asset_id        = $LAST_ASSET_ID"
  echo "视频生成引用值  = asset://$LAST_ASSET_ID"
}

# 端到端矩阵：三种上传路径并排对比
op_matrix() {
  section "端到端矩阵测试"
  echo "目的：验证「真人组 / AIGC 组 / 不传组」三条上传路径的行为差异，尤其是否触发默认组重建。"
  echo ""

  echo -e "${BLUE}=== [1/3] AIGC 素材组：显式创建后上传 ===${NC}"
  http_call POST "/api/seedance/asset-groups" \
    "$(jq -nc '{Name:"matrix-aigc-group",Description:"matrix test",GroupType:"AIGC"}')"
  local aigc_gid; aigc_gid="$(unwrap "$HTTP_BODY" | jq -r '.Id // .Result.Id // empty')"
  echo -e "  AIGC_GROUP_ID=${aigc_gid}"
  [ -n "$aigc_gid" ] && do_upload "$aigc_gid" "$AIGC_IMAGE_URL" "matrix-aigc-asset"
  echo ""

  echo -e "${BLUE}=== [2/3] 不传 GroupId：走默认 AIGC 组 ===${NC}"
  do_upload "" "$AIGC_IMAGE_URL" "matrix-default-asset"
  echo ""

  echo -e "${BLUE}=== [3/3] 真人组：需要先有 verified 的认证任务 ===${NC}"
  if [ -n "${1:-}" ]; then
    do_upload "$1" "$FACE_IMAGE_URL" "matrix-face-asset"
  else
    echo "  未提供 group_id，跳过。"
    echo "  用法: $0 --matrix <group_id>   （group_id 来自认证成功后的返回）"
  fi

  echo ""
  echo -e "${BLUE}=== 本地素材列表对照 ===${NC}"
  op_assets
}

# ---------- 主入口 ----------

need_tools
need_key
LAST_STATUS=""; LAST_GROUP=""; LAST_ASSET_ID=""; LAST_ASSET_GROUP=""
LAST_ASSET_STATUS=""; LAST_VERIFICATION_ID=""

case "${1:-}" in
  ""|--create)     op_create ;;
  --status)        [ -z "${2:-}" ] && { echo "需要 verification_id 或本地 ID" >&2; exit 1; }; op_status "$2" ;;
  --watch)         [ -z "${2:-}" ] && { echo "需要 verification_id 或本地 ID" >&2; exit 1; }; op_watch "$2" ;;
  --list)          op_list ;;
  --local)         [ -z "${2:-}" ] && { echo "需要 verification_id 或本地 ID" >&2; exit 1; }; op_local "$2" ;;
  --upload)        [ -z "${2:-}" ] && { echo "需要 group_id" >&2; exit 1; }; op_upload "$2" "${3:-}" ;;
  --upload-nogroup) op_upload_nogroup "${2:-}" ;;
  --wait-asset)    [ -z "${2:-}" ] && { echo "需要 asset_id" >&2; exit 1; }; op_wait_asset "$2" ;;
  --groups)        op_groups ;;
  --assets)        op_assets "${2:-}" ;;
  --action-create) op_action_create ;;
  --action-status) [ -z "${2:-}" ] && { echo "需要 verification_id" >&2; exit 1; }; op_action_status "$2" ;;
  --full)          op_full ;;
  --matrix)        op_matrix "${2:-}" ;;
  help|-h|--help)  usage ;;
  *) echo -e "${RED}未知命令: $1${NC}" >&2; usage; exit 1 ;;
esac
