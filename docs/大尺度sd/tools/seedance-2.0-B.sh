#!/bin/bash
# ============================================================
# Seedance 上游网关直连测试脚本（不经本地 new-api）
#
# 契约来源:
#   docs/大尺度sd/CLIENT_API.md, docs/VIDEO_API.md, docs/QUICKSTART.md
#   docs/海外sd73/DreaminaSeedance接口.md
#
# 基址: http://106.54.45.168/seedance-gateway
#
# 用法:
#   ./seedance-2.0-B.sh models                     模型目录
#   ./seedance-2.0-B.sh caps                       能力
#   ./seedance-2.0-B.sh gen [prompt]               文生视频（默认模型 2.0-B-720p）
#   ./seedance-2.0-B.sh upload <文件> [title]      上传本地文件 → 素材
#   ./seedance-2.0-B.sh asset <id>                 查素材
#   ./seedance-2.0-B.sh preflight <id> [model]     素材预检查（不计费）
#   ./seedance-2.0-B.sh genre <prompt> <asset_id>  用素材生成
#   ./seedance-2.0-B.sh task <task_id>             查任务
#   ./seedance-2.0-B.sh wait <task_id>             轮询到终态
#   ./seedance-2.0-B.sh tasks                      任务列表
#   ./seedance-2.0-B.sh download <task_id> [out]   下载视频
#   ./seedance-2.0-B.sh matrix                     模型×格式可用性矩阵
#
# 环境变量:
#   KEY        API Key（必需）
#   MODEL      默认 seedance-2.0-B-720p
#   DURATION   时长秒（默认 5）
#   RATIO      比例（默认 16:9）
# ============================================================
set -uo pipefail

BASE="${BASE:-http://106.54.45.168/seedance-gateway}"
KEY="${KEY:-}"
MODEL="${MODEL:-seedance-2.0-B-720p}"
DURATION="${DURATION:-5}"
RATIO="${RATIO:-16:9}"

R='\033[0;31m'; G='\033[0;32m'; Y='\033[1;33m'; C='\033[0;36m'; B='\033[1m'; N='\033[0m'
info() { echo -e "${C}[INFO]${N}  $*"; }
ok()   { echo -e "${G}[OK]${N}    $*"; }
warn() { echo -e "${Y}[WARN]${N}  $*"; }
err()  { echo -e "${R}[ERROR]${N} $*" >&2; }
die()  { err "$*"; exit 1; }

for c in curl jq; do command -v "$c" >/dev/null || die "缺少 $c"; done
[ -n "$KEY" ] || die "请设置 KEY，例如: KEY=sk-xxx $0 models"

CODE=""; BODY=""
req() {  # $1=方法 $2=路径 [$3=body] [其余=额外curl参数]
  local m="$1" p="$2"; shift 2
  local body="" extra=()
  # 第 3 个参数若不像 curl 选项，则视为 JSON body
  if [ "$#" -gt 0 ] && [ "${1:0:1}" != "-" ]; then
    body="$1"; shift
  fi
  extra=("$@")
  local tmp; tmp=$(mktemp)
  local a=(-s -o "$tmp" -w "%{http_code}" -X "$m" "${BASE}${p}"
           -H "Authorization: Bearer ${KEY}" -H "Accept: application/json")
  [ -n "$body" ] && a+=(-H "Content-Type: application/json" -d "$body")
  # 仅在确实有额外参数时展开，避免空数组在旧 bash 下被当成一个空参数
  [ "${#extra[@]}" -gt 0 ] && a+=("${extra[@]}")
  CODE=$(curl "${a[@]}")
  BODY=$(cat "$tmp"); rm -f "$tmp"
}
show()   { echo "$BODY" | jq . 2>/dev/null || echo "$BODY"; }
jqv()    { echo "$BODY" | jq -r "$1" 2>/dev/null; }
ik()     { echo "cli-$(date +%s)-$RANDOM$RANDOM"; }

# 统一提交入口：$1=模型 $2=请求体
submit() {
  local m="$1" body="$2" k; k=$(ik)
  info "POST ${BASE}/v1/videos  (model=${m}, Idempotency-Key=${k})"
  echo "$body" | jq . 2>/dev/null
  req POST /v1/videos "$body" -H "Idempotency-Key: ${k}"
  if [ "$CODE" = "200" ]; then
    ok "已创建: $(jqv '.id')  状态: $(jqv '.status')"
  else
    err "HTTP ${CODE}"; show
  fi
}

cmd="${1:-}"; shift || true

case "$cmd" in

models)
  req GET /v1/models
  [ "$CODE" = "200" ] || { err "HTTP ${CODE}"; show; exit 1; }
  # 逐行标记当前 MODEL；末尾 true 防止最后一行不匹配导致退出码非 0
  echo "$BODY" | jq -r '.data[].id' | while read -r i; do
    if [ "$i" = "$MODEL" ]; then
      echo -e "  ${G}*${N} $i   <-- 当前 MODEL"
    else
      echo "    $i"
    fi
  done
  true
  ;;

caps)
  req GET /seedance/capabilities
  if [ "$CODE" = "200" ]; then
    echo "$BODY" | jq '{media,tasks,limits,unsupported}'
    echo "--- 模型 ---"
    echo "$BODY" | jq -r '.models[] | "  \(.id)  resolution=\(.resolution)  production_verified=\(.production_verified)"'
  else err "HTTP ${CODE}"; show; fi
  ;;

gen)   # 文生视频
  prompt="${1:-抽象光影缓慢流动，镜头缓缓推进}"
  body=$(jq -n --arg m "$MODEL" --arg t "$prompt" --argjson d "$DURATION" --arg r "$RATIO" \
    '{model:$m,duration:$d,ratio:$r,n:1,content:[{type:"text",text:$t}]}')
  submit "$MODEL" "$body"
  ;;

genre*)  # genre <prompt> <asset_id>
  prompt="${1:-让画面中的光影缓慢流动}"
  asset="${2:-}"
  [ -n "$asset" ] || die "用法: genre <prompt> <asset_id>"
  [[ "$asset" == asset://* ]] || asset="asset://${asset}"
  body=$(jq -n --arg m "$MODEL" --arg t "$prompt" --arg a "$asset" --argjson d "$DURATION" --arg r "$RATIO" \
    '{model:$m,duration:$d,ratio:$r,n:1,
      content:[{type:"text",text:$t},
               {type:"image_url",image_url:{url:$a},role:"reference_image"}]}')
  submit "$MODEL" "$body"
  ;;

upload)
  f="${1:-}"; [ -f "$f" ] || die "文件不存在: $f"
  title="${2:-$(basename "$f")}"
  case "$(file -b --mime-type "$f" 2>/dev/null)" in
    image/png)  mime=image/png ;;
    image/jpeg) mime=image/jpeg ;;
    image/webp) mime=image/webp ;;
    video/mp4)  mime=video/mp4 ;;
    audio/mpeg) mime=audio/mpeg ;;
    audio/wav|audio/x-wav) mime=audio/wav ;;
    audio/mp4)  mime=audio/mp4 ;;
    *) die "不支持的类型: $(file -b --mime-type "$f")" ;;
  esac
  info "POST ${BASE}/seedance/media/upload  ($(wc -c <"$f" | tr -d ' ') bytes, ${mime})"
  req POST "/seedance/media/upload?title=$(printf '%s' "$title" | jq -sRr @uri)" "" \
      -H "Content-Type: ${mime}" --data-binary "@${f}"
  if [ "$CODE" = "200" ]; then
    show
    ok "asset_id=$(jqv '.id')"
    warn "status=queued 只表示受理，须轮询到 succeeded 才能引用"
  else err "HTTP ${CODE}"; show; fi
  ;;

asset)
  id="${1:-}"; [ -n "$id" ] || die "缺少 asset_id"
  req POST /seedance/media "$(jq -n --arg i "$id" '{action:"get",id:$i}')"
  if [ "$CODE" = "200" ]; then
    st=$(jqv '.status'); show
    case "$st" in
      succeeded) ok "素材就绪，可引用 asset://${id}" ;;
      failed)    err "素材处理失败" ;;
      *)         warn "状态 ${st}，稍后重试" ;;
    esac
  else err "HTTP ${CODE}"; show; fi
  ;;

preflight)
  id="${1:-}"; m="${2:-$MODEL}"; role="${3:-reference_image}"
  [ -n "$id" ] || die "用法: preflight <asset_id> [model] [role]"
  info "POST ${BASE}/seedance/media/preflight  (asset=${id}, model=${m})"
  req POST /seedance/media/preflight \
    "$(jq -n --arg i "$id" --arg m "$m" --arg r "$role" '{asset_id:$i,model:$m,role:$r}')"
  if [ "$CODE" = "200" ]; then show; else err "HTTP ${CODE}"; show; fi
  ;;

task)
  id="${1:-}"; [ -n "$id" ] || die "缺少 task_id"
  req GET "/v1/videos/${id}"; show
  ;;

wait)
  id="${1:-}"; [ -n "$id" ] || die "缺少 task_id"
  max="${2:-60}"
  for i in $(seq 1 "$max"); do
    sleep 10
    req GET "/v1/videos/${id}"
    st=$(jqv '.status'); pr=$(jqv '.progress')
    echo "  #$i  status=${st} progress=${pr}"
    case "$st" in
      completed) ok "完成"; show; exit 0 ;;
      failed)    err "失败"; show; exit 1 ;;
    esac
  done
  warn "轮询超时"; show; exit 2
  ;;

tasks)
  req GET "/v1/videos?page=1&page_size=20"
  if [ "$CODE" = "200" ]; then
    echo "$BODY" | jq -r '(.data // .items // [])[] | "  \(.id)  \(.model)  \(.status)"'
  else err "HTTP ${CODE}"; show; fi
  ;;

download)
  id="${1:-}"; out="${2:-seedance-${id}.mp4}"
  [ -n "$id" ] || die "缺少 task_id"
  info "GET ${BASE}/v1/videos/${id}/content -> ${out}"
  curl -s -m 300 -o "$out" -w "HTTP %{http_code}  %{size_download} bytes\n" \
    "${BASE}/v1/videos/${id}/content" -H "Authorization: Bearer ${KEY}"
  [ -s "$out" ] && ok "$(file -b "$out" | cut -c1-50)" || err "下载失败"
  ;;

matrix)  # 模型 × 请求格式可用性
  echo -e "${B}模型 × 请求格式可用性（200=可用, 400=参数被拒）${N}"
  echo "-----------------------------------------------------------------"
  printf "%-24s %-9s %-9s %-9s\n" "模型" "content" "prompt" "metadata"
  echo "-----------------------------------------------------------------"
  for m in seedance-2.0-mini-720p seedance-2.0-720p seedance-2.5-720p \
           seedance-2.0-B-720p seedance-2.0-B-1080p seedance-2.0-C-720p; do
    a=$(curl -s -m 30 -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/videos" \
        -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
        -H "Idempotency-Key: $(ik)" \
        -d "{\"model\":\"$m\",\"duration\":5,\"ratio\":\"16:9\",\"content\":[{\"type\":\"text\",\"text\":\"test\"}],\"n\":1}")
    b=$(curl -s -m 30 -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/videos" \
        -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
        -H "Idempotency-Key: $(ik)" \
        -d "{\"model\":\"$m\",\"prompt\":\"test\",\"seconds\":5,\"size\":\"1280x720\"}")
    cc=$(curl -s -m 30 -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/videos" \
        -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
        -H "Idempotency-Key: $(ik)" \
        -d "{\"model\":\"$m\",\"prompt\":\"test\",\"metadata\":{\"resolution\":\"720p\",\"ratio\":\"16:9\",\"duration\":5}}")
    printf "%-24s %-9s %-9s %-9s\n" "$m" "$a" "$b" "$cc"
  done
  echo "-----------------------------------------------------------------"
  warn "以上每个 200 都会创建计费任务"
  ;;

*)
  sed -n '2,32p' "$0" | sed 's/^# \{0,1\}//'
  ;;
esac
