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
#   ./seedance-2.0-B.sh group create <名称> [描述] 创建资产组（并设为默认）
#   ./seedance-2.0-B.sh group list                资产组列表
#   ./seedance-2.0-B.sh group get <id>            查询资产组
#   ./seedance-2.0-B.sh group default             获取/自动创建默认资产组
#   ./seedance-2.0-B.sh upload <文件> [title] [group_id] [--wait]
#                                                 上传文件到资产组（默认组自动创建）
#   ./seedance-2.0-B.sh assets [group_id|default] [page] [page_size]
#                                                 资产列表
#   ./seedance-2.0-B.sh t2v <图片> <提示词> [group_id] [model] [role]
#                                                 图片+提示词直接生视频
#                                                 （自动建组→上传→等就绪→用 asset:// 生成）
#   ./seedance-2.0-B.sh asset <id>                 查素材
#   ./seedance-2.0-B.sh preflight <id> [model]     素材预检查（不计费）
#   ./seedance-2.0-B.sh genre <prompt> <asset_id> [role] [model]
#                                                 用已有素材生成
#   ./seedance-2.0-B.sh task <task_id>             查任务
#   ./seedance-2.0-B.sh wait <task_id>             轮询到终态
#   ./seedance-2.0-B.sh tasks                      任务列表
#   ./seedance-2.0-B.sh download <task_id> [out]   下载视频
#   ./seedance-2.0-B.sh media-test [文件]          资产库接口测试（不计费/只读 + 可选上传轮询）
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
# KEY 优先级：环境变量 KEY > 同级 .key 文件 > KEY_FILE 指定文件
KEY_FILE="${KEY_FILE:-$(dirname "${BASH_SOURCE[0]}")/.key}"
KEY="${KEY:-}"   # 密钥不要写死；.key 文件已在 .gitignore 排除
if [ -z "$KEY" ] && [ -f "$KEY_FILE" ]; then
  KEY=$(head -1 "$KEY_FILE" | tr -d ' \r\n')
fi
MODEL="${MODEL:-seedance-2.0-B-720p}"
DURATION="${DURATION:-5}"
RATIO="${RATIO:-16:9}"
# 固定默认资产组名：先按名查询，不存在再创建
DEFAULT_GROUP_NAME="${DEFAULT_GROUP_NAME:-default-test}"

R='\033[0;31m'; G='\033[0;32m'; Y='\033[1;33m'; C='\033[0;36m'; B='\033[1m'; N='\033[0m'
info() { echo -e "${C}[INFO]${N}  $*"; }
ok()   { echo -e "${G}[OK]${N}    $*"; }
warn() { echo -e "${Y}[WARN]${N}  $*"; }
err()  { echo -e "${R}[ERROR]${N} $*" >&2; }
die()  { err "$*"; exit 1; }

for c in curl jq; do command -v "$c" >/dev/null || die "缺少 $c"; done

show_usage() {
  echo -e "${B}Seedance 上游网关测试脚本${N}"
  echo "基址: ${BASE}"
  echo ""
  echo -e "${B}用法:${N} KEY=<API Key> $0 <命令> [参数...]"
  echo ""
  echo -e "${B}【资产库】${N}（上传资产 → 用资产 ID 生成）"
  echo "  group create <名称> [描述]       创建资产组"
  echo "  group list                       资产组列表"
  echo "  group get <group_id>             查询单个资产组"
  echo "  group default                    查询/自动创建默认组（${DEFAULT_GROUP_NAME}）"
  echo "  upload <文件> [title] [group_id] [--wait]"
  echo "                                   上传文件到资产组（默认组自动创建）"
  echo "                                   --wait 轮询到 succeeded"
  echo "  assets [group_id|default] [page] [page_size]"
  echo "                                   资产列表"
  echo "  asset <asset_id>                 查询素材状态"
  echo "  preflight <asset_id> [model] [role]"
  echo "                                   素材×模型预检（不计费）"
  echo ""
  echo -e "${B}【生成视频】${N}（计费）"
  echo "  t2v <图片> <提示词> [group_id] [model] [role]"
  echo "                                   图片+提示词生视频"
  echo "                                   自动: 建组→上传→等就绪→用 asset:// 生成"
  echo "  genre <提示词> <asset_id> [role] [model]"
  echo "                                   用已有素材生视频"
  echo "  gen [提示词]                     纯文生视频"
  echo ""
  echo -e "${B}【任务】${N}"
  echo "  task <task_id> [输出路径]        查任务（完成自动解析下载）"
  echo "  wait <task_id> [轮数] [输出路径] 轮询到终态并自动下载"
  echo "  tasks                            任务列表"
  echo "  download <task_id> [输出]        手动下载"
  echo ""
  echo -e "${B}【诊断】${N}"
  echo "  models                           模型目录"
  echo "  caps                             平台能力"
  echo "  media-test [文件]                资产库接口测试（不计费/只读）"
  echo "  matrix                           模型×请求格式可用性（会计费）"
  echo ""
  echo -e "${B}【环境变量】${N}"
  echo "  KEY               API Key（必需）"
  echo "  BASE              上游基址（默认 ${BASE}）"
  echo "  MODEL             默认模型（${MODEL}）"
  echo "  DURATION          时长秒（${DURATION}）"
  echo "  RATIO             比例（${RATIO}）"
  echo "  DEFAULT_GROUP_NAME 默认资产组名（${DEFAULT_GROUP_NAME}）"
  echo "  ROLE              默认素材角色（reference_image）"
  echo "  CHECK=1           提交前跑 preflight 预检"
  echo "  OUT_DIR           下载目录（.）"
  echo "  AUTO_DOWNLOAD=0   关闭 task/wait 自动下载"
  echo "  WAIT_MAX          素材轮询上限（150，每次 2 秒）"
  echo ""
  echo -e "${B}【示例】${N}"
  echo "  KEY=sk-xxx $0 t2v ./photo.png '画面中人物缓慢走动'"
  echo "  KEY=sk-xxx $0 group default"
  echo "  KEY=sk-xxx $0 upload ./photo.png '参考图' --wait"
  echo "  KEY=sk-xxx $0 task task_xxxxx"
}

# 无参数 或 -h/--help：先打印用法说明（不需要 KEY）
case "${1:-}" in
  -h|--help|help) show_usage; exit 0 ;;
esac
if [ "$#" -eq 0 ]; then
  show_usage
  echo ""
  [ -n "$KEY" ] || die "请设置 KEY，例如: KEY=sk-xxx $0 models"
  exit 0
fi
[ -n "$KEY" ] || die "请设置 KEY，例如: KEY=sk-xxx $0 models  （运行 $0 -h 查看全部命令）"

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

# 任务查询成功后自动解析并下载成片
AUTO_DOWNLOAD="${AUTO_DOWNLOAD:-1}"   # 设 0 可关闭 task/wait 的自动下载
OUT_DIR="${OUT_DIR:-.}"              # 默认输出目录

# $1=task_id [$2=输出路径] [$3=已查到的内容JSON，复用可避免重复请求]
auto_download() {
  local id="$1" out="${2:-}" content="${3:-}"
  [ -n "$id" ] || { err "auto_download 缺少 task_id"; return 1; }

  if [ -z "$content" ]; then
    req GET "/v1/videos/${id}"
    [ "$CODE" = "200" ] || { err "查询失败 HTTP ${CODE}"; return 1; }
    content="$BODY"
  fi

  local st url size res sec exp storage model
  st=$(echo "$content"      | jq -r '.status // empty')
  url=$(echo "$content"     | jq -r '.url // (.urls[0] // empty) // (.outputs[0].video_url // empty) // (.content.video_url // empty)')
  size=$(echo "$content"    | jq -r '.size // empty')
  res=$(echo "$content"     | jq -r '.resolution // empty')
  sec=$(echo "$content"     | jq -r '.seconds // empty')
  model=$(echo "$content"   | jq -r '.model // empty')
  exp=$(echo "$content"     | jq -r '.storage.expires_at // .expires_at // empty')
  storage=$(echo "$content" | jq -r '.storage.status // empty')

  [ "$st" = "completed" ] || { err "任务状态 ${st}，未完成不下载"; return 1; }
  [ -n "$url" ] || { err "响应中未找到下载地址 url"; return 1; }
  info "解析: model=${model} 尺寸=${size} 分辨率=${res} 时长=${sec}s storage=${storage}"
  [ "$storage" = "ready" ] || warn "storage.status=${storage}，成片可能尚未保存完成"
  if [ -n "$exp" ]; then
    info "保存到期: $(date -r "$exp" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || echo "$exp")"
  fi

  [ -n "$out" ] || out="${OUT_DIR}/seedance-${id}.mp4"
  mkdir -p "$(dirname "$out")" 2>/dev/null || true

  # url 是相对路径（如 /v1/videos/xxx/content），必须拼接 API_BASE，且带同一用户 Key
  local full="${BASE}${url}" code bytes
  info "GET ${full} -> ${out}"
  code=$(curl -s -m 300 -o "$out" -w "%{http_code}" "$full" -H "Authorization: Bearer ${KEY}")
  bytes=$(wc -c <"$out" 2>/dev/null | tr -d " ")
  if [ "$code" != "200" ]; then
    err "下载失败 HTTP ${code}（410=已过期 404=不可见 403=无权限）"
    head -c 300 "$out" 2>/dev/null; echo; rm -f "$out"; return 1
  fi
  case "$(file -b "$out" 2>/dev/null)" in
    *"ISO Media"*|*MP4*) ok "下载成功 ${out} (${bytes} bytes)" ;;
    *) err "文件不是有效 MP4，已删除"; head -c 300 "$out"; echo; rm -f "$out"; return 1 ;;
  esac
  if command -v ffprobe >/dev/null 2>&1; then
    info "校验: $(ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,width,height -show_entries format=duration -of csv=p=0:s=x "$out" 2>/dev/null | tr "\n" " ")"
  fi
  echo "$out"
}
# ---------- 资产组 ----------
# 组名变量定义在脚本顶部（show_usage 会引用）

# 内部：按名称查找资产组，输出 group_id；找不到输出空
find_group_by_name() {
  local name="$1" i page=1 size=100 max_page=10 total=0
  while [ "$page" -le "$max_page" ]; do
    req POST "/seedance/media/groups" "$(jq -n --argjson p "$page" --argjson s "$size" '{action:"list",page:$p,page_size:$s}')"
    [ "$CODE" = "200" ] || return 1
    # 先按名称精确匹配当前页
    local hit
    hit=$(echo "$BODY" | jq -r --arg n "$name" '(.data // [])[] | select(.name == $n) | .id' | head -1)
    [ -n "$hit" ] && [ "$hit" != "null" ] && { echo "$hit"; return 0; }
    # 判断是否还有下一页
    total=$(echo "$BODY" | jq -r '.total // ((.data // []) | length)')
    local cnt; cnt=$(echo "$BODY" | jq '(.data // []) | length')
    [ "$cnt" -lt "$size" ] && break
    [ "$((page * size))" -ge "$total" ] && break
    page=$((page + 1))
  done
  echo ""
  return 0
}

# 内部：确保存在指定名称的资产组（默认 default-test），输出 group_id
# 流程：按名字 list 查询 → 有则复用 → 没有则 create
ensure_group() {
  local name="${1:-$DEFAULT_GROUP_NAME}" gid=""
  info "查询资产组: name=\"${name}\"" >&2
  gid=$(find_group_by_name "$name")
  if [ -n "$gid" ]; then
    ok "已存在资产组，直接复用: ${gid}" >&2
    echo "$gid"; return 0
  fi
  info "资产组 \"${name}\" 不存在，创建中" >&2
  req POST "/seedance/media/groups" "$(jq -n --arg n "$name" '{action:"create",name:$n,description:"auto-created for asset upload"}')"
  if [ "$CODE" != "200" ]; then
    err "创建资产组失败 HTTP ${CODE}" >&2; show >&2; return 1
  fi
  gid=$(jqv '.id')
  [ -n "$gid" ] && [ "$gid" != "null" ] || { err "创建资产组未返回 id" >&2; show >&2; return 1; }
  ok "已创建资产组: ${gid}" >&2
  echo "$gid"
}

# 内部：上传文件到指定（或默认）分组，输出 asset_id
# $1=文件 [$2=group_id] [$3=title]  输出: asset_id
upload_to_group() {
  local f="$1" gid="${2:-}" title="${3:-}"
  [ -f "$f" ] || { err "文件不存在: $f" >&2; return 1; }
  [ -n "$gid" ] || gid=$(ensure_group) || return 1
  [ -n "$title" ] || title="$(basename "$f")"
  local fsize mime
  fsize=$(wc -c <"$f" | tr -d ' ')
  [ "$fsize" -gt 0 ] || { err "文件为空: $f" >&2; return 1; }
  [ "$fsize" -le 67108864 ] || { err "超过 64 MiB: ${fsize} bytes" >&2; return 1; }
  case "$(file -b --mime-type "$f" 2>/dev/null)" in
    image/png)                       mime=image/png ;;
    image/jpeg)                      mime=image/jpeg ;;
    image/webp)                      mime=image/webp ;;
    video/mp4)                       mime=video/mp4 ;;
    audio/mpeg|audio/mp3)            mime=audio/mpeg ;;
    audio/wav|audio/x-wav)           mime=audio/wav ;;
    audio/mp4|audio/m4a|audio/x-m4a) mime=audio/mp4 ;;
    *) err "不支持的类型: $(file -b --mime-type "$f")" >&2; return 1 ;;
  esac
  req POST "/seedance/media/upload?title=$(printf '%s' "$title" | jq -sRr @uri)&group_id=$(printf '%s' "$gid" | jq -sRr @uri)" \
      "" -H "Content-Type: ${mime}" --data-binary "@${f}"
  [ "$CODE" = "200" ] || { err "上传失败 HTTP ${CODE}（${fsize} bytes, ${mime}）" >&2; show >&2; return 1; }
  local aid; aid=$(jqv '.id')
  [ -n "$aid" ] && [ "$aid" != "null" ] || { err "上传未返回 id" >&2; show >&2; return 1; }
  echo "$aid"
}

# 内部：等待素材 succeeded  [$1=asset_id] [$2=max 轮数，默认 150，每次 2 秒]
wait_asset() {
  local aid="$1" max="${2:-${WAIT_MAX:-150}}" i st=""
  for i in $(seq 1 "$max"); do
    sleep 2
    req POST "/seedance/media" "$(jq -n --arg i "$aid" '{action:"get",id:$i}')"
    st=$(jqv '.status')
    printf '  #%s  status=%s\n' "$i" "$st" >&2
    [ "$st" = "succeeded" ] && { echo "succeeded"; return 0; }
    [ "$st" = "failed" ] && { echo "failed"; return 1; }
  done
  echo "timeout"; return 2
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

genre*)  # 用资产库素材生成视频：genre <prompt> <asset_id> [role] [model]
  # role: reference_image（默认） | first_frame | last_frame | reference_video | reference_audio
  #   first_frame/last_frame 走 image_url，reference_video 走 video_url，reference_audio 走 audio_url
  # 环境变量: ROLE / MODEL / DURATION / RATIO / CHECK=1（提交前先跑 preflight 预检）
  prompt="${1:-让画面中的光影缓慢流动}"
  asset="${2:-}"
  [ -n "$asset" ] || die "用法: genre <prompt> <asset_id> [role] [model]"
  role="${3:-${ROLE:-reference_image}}"
  m="${4:-$MODEL}"
  [[ "$asset" == asset://* ]] || asset="asset://${asset}"
  aid="${asset#asset://}"

  # 1) 素材必须 succeeded 才能引用（文档：queued/running 会被 409 asset_not_ready）
  info "检查素材状态: ${aid}"
  req POST "/seedance/media" "$(jq -n --arg i "$aid" '{action:"get",id:$i}')"
  if [ "$CODE" = "200" ]; then
    ast=$(jqv '.status'); amt=$(jqv '.media_type'); adur=$(jqv '.duration')
    info "素材: status=${ast}  media_type=${amt}  duration=${adur}"
    case "$ast" in
      succeeded) ok "素材就绪" ;;
      failed)    die "素材处理失败，不能引用（重新上传或换素材）" ;;
      *)         die "素材未就绪（${ast}）；先等待 succeeded：./${0##*/} asset ${aid} 或 upload ... --wait" ;;
    esac
  else err "查询素材失败 HTTP ${CODE}"; show; exit 1; fi

  # 2) 按角色选择 content 字段
  case "$role" in
    first_frame|last_frame|reference_image) ctype="image_url"; ckey="image_url" ;;
    reference_video) ctype="video_url"; ckey="video_url" ;;
    reference_audio) ctype="audio_url"; ckey="audio_url" ;;
    *) die "未知 role: ${role}（支持 reference_image/first_frame/last_frame/reference_video/reference_audio）" ;;
  esac

  # 3) 可选：提交前不计费预检
  if [ "${CHECK:-0}" = "1" ]; then
    info "预检 preflight（不计费）"
    req POST "/seedance/media/preflight" \
      "$(jq -n --arg i "$aid" --arg m "$m" --arg r "$role" --arg t "$amt" \
         '{asset_id:$i,model:$m,role:$r} + (if $t != "" then {media_type:$t} else {} end)')"
    if [ "$CODE" = "200" ]; then
      echo "$BODY" | jq -c '{compatibility, production_verified}'
      comp=$(echo "$BODY" | jq -r '.compatibility')
      [ "$comp" = "unsupported" ] && die "预检不兼容，已中止（不提交计费任务）"
      [ "$comp" = "unverified" ] && warn "预检 unverified：未验证，仍需小额试单"
    else warn "预检失败 HTTP ${CODE}，继续提交"; fi
  fi

  # 4) 构造 content 请求体（duration/ratio/n 与 prompt 不混用）
  body=$(jq -n --arg m "$m" --arg t "$prompt" --arg a "$asset" --arg role "$role" \
                 --arg ck "$ckey" --arg ct "$ctype" \
                 --argjson d "$DURATION" --arg r "$RATIO" \
    '{model:$m,duration:$d,ratio:$r,n:1,
      content:[{type:"text",text:$t},
               ({type:$ct, ($ck):{url:$a}, role:$role})]}')
  info "引用素材: ${asset}  role=${role}  model=${m}"
  submit "$m" "$body"
  ;;


group)  # 创建/查询资产组：group create <名称> [描述] | group list | group get <id>
  sub="${1:-list}"
  case "$sub" in
    create)
      n="${2:-$DEFAULT_GROUP_NAME}"; d="${3:-}"
      req POST "/seedance/media/groups" "$(jq -n --arg n "$n" --arg d "$d" '{action:"create",name:$n} + (if $d != "" then {description:$d} else {} end)')"
      if [ "$CODE" = "200" ]; then show; gid=$(jqv '.id'); ok "group_id=${gid}"
        info "已创建并作为默认组名使用：${n}（后续 ensure_group 会按名复用）"
      else err "HTTP ${CODE}"; show; exit 1; fi ;;
    list)
      req POST "/seedance/media/groups" "$(jq -n '{action:"list",page:1,page_size:50}')"
      if [ "$CODE" = "200" ]; then
        echo "$BODY" | jq -r '(.data // [])[] | "  \(.id)  \(.name)  \(.description // "")"'
        [ "$(echo "$BODY" | jq '.data|length')" = "0" ] && info "暂无资产组"
      else err "HTTP ${CODE}"; show; exit 1; fi ;;
    get)
      id="${2:-}"; [ -n "$id" ] || die "用法: group get <group_id>"
      req POST "/seedance/media/groups" "$(jq -n --arg i "$id" '{action:"get",id:$i}')"
      if [ "$CODE" = "200" ]; then show; else err "HTTP ${CODE}"; show; exit 1; fi ;;
    default)
      gid=$(ensure_group) || exit 1
      ok "默认资产组: ${gid}" ;;
    *) die "用法: group create <名称> [描述] | group list | group get <id> | group default" ;;
  esac
  ;;

assets)  # 资产列表：assets [group_id|default] [page] [page_size]
  gid="${1:-}"; page="${2:-1}"; psize="${3:-20}"
  if [ "$gid" = "default" ]; then gid=$(find_group_by_name "$DEFAULT_GROUP_NAME"); fi
  if [ -n "$gid" ]; then
    qs="/seedance/media?group_id=$(printf '%s' "$gid" | jq -sRr @uri)&page=${page}&page_size=${psize}"
  else
    qs="/seedance/media?page=${page}&page_size=${psize}"
  fi
  info "GET ${BASE}${qs}"
  req GET "$qs"
  if [ "$CODE" = "200" ]; then
    echo "$BODY" | jq -r '(.data // [])[] | "  \(.id)  \(.status)  \(.media_type // "-")  group=\(.group_id // "-")  \(.title // "")"'
    info "total=$(jqv '.total')  page=$(jqv '.page')/$((($(jqv '.total // 0') + psize - 1) / psize))"
  else err "HTTP ${CODE}"; show; exit 1; fi
  ;;

# 核心：图片 + 提示词 直接生视频（自动建组→上传→等就绪→用 asset:// 生视频）
# 用法: t2v <图片> <提示词> [group_id] [model] [role]
#   role 默认 reference_image；first_frame 可用首帧方式
#   环境变量: CHECK=1 提交前预检 / WAIT_MAX 轮询上限 / DURATION / RATIO
t2v)  # 图片+提示词生视频；多图：t2v "图1 图2" "提示词" [group_id] [model] [role]
  img="${1:-}"; prompt="${2:-}"; gid="${3:-}"; m="${4:-$MODEL}"; role="${5:-${ROLE:-reference_image}}"
  [ -n "$img" ] && [ -n "$prompt" ] || die "用法: t2v <图片|'图1 图2'> <提示词> [group_id] [model] [role]"
  duration="${DURATION:-5}"

  # 拆分多图（空格分隔），校验存在与数量
  img_list=()
  for f in $img; do
    [ -f "$f" ] || die "图片不存在: $f"
    img_list+=("$f")
  done
  [ "${#img_list[@]}" -le 9 ] || die "最多 9 张参考图（文档限制）"

  echo -e "${B}=== 资产图生视频 ===${N}"
  info "图片数量: ${#img_list[@]}"
  for f in "${img_list[@]}"; do info "  - ${f}"; done
  info "时长: ${duration}s  模型: ${m}  role: ${role}"
  info "提示词: ${prompt:0:60}..."

  # 1) 资产组
  if [ -n "$gid" ]; then info "使用指定资产组: ${gid}"; else info "未指定资产组，确保默认组存在"; gid=$(ensure_group) || exit 1; fi

  # 2) 逐张上传 + 等待就绪
  asset_ids=()
  n=0
  for f in "${img_list[@]}"; do
    n=$((n+1))
    info "[${n}/${#img_list[@]}] 上传 ${f}"
    aid=$(upload_to_group "$f" "$gid" "$(basename "$f")") || die "上传失败: ${f}"
    ok "asset_id=${aid}"
    info "[${n}/${#img_list[@]}] 等待素材就绪"
    wr=$(wait_asset "$aid") || { err "素材未就绪: ${wr}"; die "中止，不提交计费任务"; }
    ok "素材就绪 asset://${aid}"
    asset_ids+=("$aid")
  done

  # 3) role → content 字段
  case "$role" in
    first_frame|last_frame|reference_image) ct=image_url; ck=image_url ;;
    reference_video) ct=video_url; ck=video_url ;;
    reference_audio) ct=audio_url; ck=audio_url ;;
    *) die "未知 role: ${role}" ;;
  esac

  # 4) content = 文本 + N 个 asset:// 引用
  content_json=$(printf '%s\n' "${asset_ids[@]}" | jq -R -s --arg t "$prompt" --arg role "$role" --arg ck "$ck" --arg ct "$ct" \
    '[{type:"text",text:$t}] + (split("\n") | map(select(length>0)) | map({type:$ct, ($ck):{url:("asset://" + .)}, role:$role}))')
  body=$(jq -n --arg m "$m" --argjson d "$duration" --arg r "$RATIO" --argjson c "$content_json" \
    '{model:$m,duration:$d,ratio:$r,n:1,content:$c}')

  info "[生成] 引用 ${#asset_ids[@]} 个素材  role=${role}  model=${m}  时长=${duration}s"
  submit "$m" "$body"
  tid=$(jqv '.id')
  if [ -n "$tid" ] && [ "$tid" != "null" ]; then
    echo "  group_id : ${gid}"
    for a in "${asset_ids[@]}"; do echo "  asset_id : ${a}"; done
    echo "  task_id  : ${tid}"
    echo "  query    : ./${0##*/} task ${tid}"
  fi
  ;;

upload)
  # 上传本地文件到上游资产库：POST /seedance/media/upload（原始二进制，非 multipart）
  # 用法: upload <文件> [title] [group_id] [--wait]
  #       --wait 或 WAIT=1  上传后轮询到 succeeded/failed
  #       环境变量 WAIT_MAX  轮询上限次数（每次 2 秒，默认 150）
  [ "$#" -gt 0 ] || die "用法: upload <文件> [title] [group_id] [--wait]"
  f=""; title=""; gid=""; do_wait="${WAIT:-0}"
  for a in "$@"; do
    case "$a" in
      --wait) do_wait=1 ;;
      *) if [ -z "$f" ]; then f="$a"
         elif [ -z "$title" ]; then title="$a"
         elif [ -z "$gid" ]; then gid="$a"
         fi ;;
    esac
  done
  [ -f "$f" ] || die "文件不存在: $f"
  [ -n "$title" ] || title="$(basename "$f")"

  fsize=$(wc -c <"$f" | tr -d ' ')
  [ "$fsize" -gt 0 ] || die "文件为空: $f"
  [ "$fsize" -le 67108864 ] || die "超过 64 MiB 上限: ${fsize} bytes"
  case "$(file -b --mime-type "$f" 2>/dev/null)" in
    image/png)                       mime=image/png ;;
    image/jpeg)                      mime=image/jpeg ;;
    image/webp)                      mime=image/webp ;;
    video/mp4)                       mime=video/mp4 ;;
    audio/mpeg|audio/mp3)            mime=audio/mpeg ;;
    audio/wav|audio/x-wav)           mime=audio/wav ;;
    audio/mp4|audio/m4a|audio/x-m4a) mime=audio/mp4 ;;
    *) die "不支持的类型: $(file -b --mime-type "$f")（仅 png/jpeg/webp/mp4/mpeg/wav/m4a）" ;;
  esac

  qs="?title=$(printf '%s' "$title" | jq -sRr @uri)"
  [ -n "$gid" ] && qs="${qs}&group_id=$(printf '%s' "$gid" | jq -sRr @uri)"

  info "POST ${BASE}/seedance/media/upload${qs}"
  info "文件: ${f}  (${fsize} bytes, ${mime})"
  req POST "/seedance/media/upload${qs}" "" -H "Content-Type: ${mime}" --data-binary "@${f}"

  if [ "$CODE" != "200" ]; then
    err "上传失败 HTTP ${CODE}"
    case "$CODE" in
      401) err "认证无效，检查 KEY" ;;
      403) err "权限不足，需要 media-service 权限" ;;
      413) err "文件过大" ;;
      415) err "媒体类型不被接受，必须使用真实 MIME" ;;
    esac
    show; exit 1
  fi

  show
  aid=$(jqv '.id')
  ok "已受理  asset_id=${aid}"
  [ -n "$(jqv '.asset_uri')" ] && info "asset_uri=$(jqv '.asset_uri')"
  info "status=$(jqv '.status')  media_type=$(jqv '.media_type')  duration=$(jqv '.duration')"
  warn "HTTP 200 / queued 只表示受理，须等到 succeeded 才能用于生成"

  if [ "$do_wait" = "1" ]; then
    max="${WAIT_MAX:-150}"
    info "轮询素材状态（每 2 秒，最多 ${max} 次）"
    st=""
    for i in $(seq 1 "$max"); do
      sleep 2
      req POST "/seedance/media" "$(jq -n --arg i "$aid" '{action:"get",id:$i}')"
      st=$(jqv '.status')
      echo "  #$i  status=${st}"
      [ "$st" = "succeeded" ] && break
      [ "$st" = "failed" ] && break
    done
    case "$st" in
      succeeded) ok "素材就绪，可引用: asset://${aid}" ;;
      failed)    err "素材处理失败，检查源文件尺寸/格式后重新上传"; show; exit 1 ;;
      *)         warn "轮询超时，保留 ID 稍后查询: ${aid}"; exit 2 ;;
    esac
  else
    info "稍后查询: ./${0##*/} asset ${aid}"
    info "或加 --wait 一步到底"
  fi
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
  # 查询成功且已完成时自动解析并下载；AUTO_DOWNLOAD=0 可关闭
  if [ "$CODE" = "200" ] && [ "$AUTO_DOWNLOAD" != "0" ]; then
    case "$(jqv '.status')" in
      completed) auto_download "$id" "${2:-}" "$BODY" ;;
      failed)    err "任务失败: $(jqv '.error')" ;;
      *)         info "状态 $(jqv '.status')，未完成；用 wait 轮询到完成后自动下载" ;;
    esac
  fi
  ;;

wait)
  id="${1:-}"; [ -n "$id" ] || die "缺少 task_id"
  # 用法: wait <task_id> [最大轮数=60] [输出路径]
  max="${2:-60}"
  for i in $(seq 1 "$max"); do
    sleep 10
    req GET "/v1/videos/${id}"
    st=$(jqv '.status'); pr=$(jqv '.progress')
    echo "  #$i  status=${st} progress=${pr}"
    case "$st" in
      completed)
        ok "完成"; show
        auto_download "$id" "${3:-}" "$BODY"
        exit 0 ;;
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

media-test)  # 资产库测试：全部不计费/只读，不创建视频任务
  # 用法: media-test [文件]        带文件则跑「上传→轮询→预检→列表→分组」全链路
  #       media-test             不带文件只跑只读自检
  f="${1:-}"
  pass=0; fail=0
  chk() { if [ "$1" = "$2" ]; then ok "$3 (HTTP $1)"; pass=$((pass+1)); else err "$3 (HTTP $1，期望 $2)"; fail=$((fail+1)); fi; }

  echo -e "${B}=== 资产库接口测试（不计费，不创建视频任务）===${N}"
  echo "BASE=${BASE}"

  # 1) 模型目录：确认 media-service 权限模型可见
  info "[1/6] GET /v1/models  查 media-service 权限"
  req GET /v1/models
  chk "$CODE" 200 "模型目录"
  if [ "$CODE" = "200" ]; then
    if echo "$BODY" | jq -e '.data[].id | select(. == "media-service")' >/dev/null 2>&1; then
      ok "media-service 在目录中"
    else err "目录中没有 media-service，素材接口大概率不可用"; fail=$((fail+1)); fi
  else show; echo -e "${R}认证/网络失败，后续跳过${N}"; exit 1; fi

  # 2) capabilities
  info "[2/6] GET /seedance/capabilities"
  req GET /seedance/capabilities
  chk "$CODE" 200 "能力目录"
  [ "$CODE" = "200" ] && echo "$BODY" | jq -c '{media, tasks, limits}'

  # 3) 素材列表（只读）
  info "[3/6] GET /seedance/media?page=1&page_size=5  素材列表"
  req GET "/seedance/media?page=1&page_size=5"
  chk "$CODE" 200 "素材列表"
  [ "$CODE" = "200" ] && echo "$BODY" | jq -c '{total:(.total // (.data|length)), first:(.data[0] // null)}'

  # 4) 分组列表（只读）
  info "[4/6] POST /seedance/media/groups {action:list}"
  req POST "/seedance/media/groups" "$(jq -n '{action:"list",page:1,page_size:5}')"
  chk "$CODE" 200 "分组列表"
  [ "$CODE" = "200" ] && show | head -20

  # 5) preflight（文档：不计费，不创建任务）——需要已就绪素材，放到第 6 步上传成功后执行
  info "[5/6] POST /seedance/media/preflight  素材预检（不计费）"
  warn "依赖第 6 步上传得到的就绪素材，见下方输出"

  # 6) 上传 + 轮询（会改变数据：占用素材额度，但不计费）
  if [ -n "$f" ]; then
    info "[6/6] POST /seedance/media/upload  $f"
    [ -f "$f" ] || die "文件不存在: $f"
    case "$(file -b --mime-type "$f" 2>/dev/null)" in
      image/png) mime=image/png ;; image/jpeg) mime=image/jpeg ;; image/webp) mime=image/webp ;;
      video/mp4) mime=video/mp4 ;; audio/mpeg) mime=audio/mpeg ;; audio/wav|audio/x-wav) mime=audio/wav ;;
      *) die "不支持的类型" ;; esac
    req POST "/seedance/media/upload?title=$(printf '%s' "test-$(date +%s)" | jq -sRr @uri)" "" -H "Content-Type: ${mime}" --data-binary "@${f}"
    chk "$CODE" 200 "上传受理"
    if [ "$CODE" = "200" ]; then
      aid=$(jqv '.id'); show
      info "素材 ${aid}，轮询注册状态（每 2 秒，最多 60 次）"
      for i in $(seq 1 60); do
        sleep 2
        req POST "/seedance/media" "$(jq -n --arg i "$aid" '{action:"get",id:$i}')"
        st=$(jqv '.status'); echo "  #$i status=${st}"
        [ "$st" = "succeeded" ] && { ok "素材就绪 asset://${aid}"; break; }
        [ "$st" = "failed" ] && { err "素材处理失败"; break; }
      done
      if [ "$st" = "succeeded" ]; then
        info "对就绪素材再跑一次 preflight"
        req POST "/seedance/media/preflight" "$(jq -n --arg i "$aid" --arg m "$MODEL" '{asset_id:$i,model:$m,media_type:"image",role:"reference_image"}')"
        chk "$CODE" 200 "就绪素材预检"
        [ "$CODE" = "200" ] && show
      fi
    fi
  else
    warn "[6/6] 跳过上传：可传参 ./seedance-2.0-B.sh media-test <图片> 触发上传测试"
  fi

  echo -e "${B}=== 结果: ${G}通过 ${pass}${N}  ${R}失败 ${fail}${N} ===${N}"
  [ "$fail" -gt 0 ] && exit 1 || exit 0
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

-h|--help|help)
  show_usage
  ;;

*)
  err "未知命令: ${cmd}"
  echo ""
  show_usage
  exit 1
  ;;
esac
