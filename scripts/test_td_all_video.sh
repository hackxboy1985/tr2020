#!/usr/bin/env bash
#
# Td 渠道视频模型全量测试脚本（调用本机 new-api，非直达上游）
#
# 覆盖四个视频模型，按各自的能力矩阵逐项验证：
#   - minimax-h3            : 3-15s, 720p/1080p/2k, 9 图, 不支持 videos/audios/generate_audio
#   - seedance-2.0          : 3-15s, 仅 720p,      9 图, 3 视频, 3 音频, 支持 generate_audio
#   - seedance-2.5          : 4-30s, 仅 720p,     30 图, 10 视频, 10 音频, 支持 generate_audio
#   - grok-imagine-video-1.5: 时长枚举 6/10/15（必填）, 仅 720p, 16:9/9:16/1:1,
#                             参考图走 input_reference + extra.reference_images,
#                             无 images 字段, 不支持 videos/audios/generate_audio, extra 必填
#
# 测试分四段：
#   [1/5] 渠道可用性预检
#   [2/5] 能力矩阵校验用例（四个模型，期望 400）
#   [3/5] 各模型真实提交
#   [4/5] 轮询至终态并校验响应契约
#   [5/5] 图片任务回归（确认视频接入未污染 /v1/images/tasks）
#
# 用法：
#   ./scripts/test_td_all_video.sh <API_BASE> <API_KEY>          # 位置参数
#   或
#   export NEWAPI_BASE_URL=http://127.0.0.1:3000
#   export NEWAPI_TOKEN=sk-xxxxxxxx
#   ./scripts/test_td_all_video.sh
#
# 示例：
#   ./scripts/test_td_all_video.sh http://127.0.0.1:3000 sk-xxxxx
#
# 可选环境变量：
#   MODELS        要测的模型，逗号分隔。默认四个全测
#   SKIP_E2E      =1 只跑校验用例，不实际提交任务（不消耗额度）
#   EXTRA_MODELS  额外要测的模型（如新接入的），会按 seedance-2.5 的宽松口径校验
#   POLL_TIMEOUT  轮询超时秒数，默认 900
#   POLL_INTERVAL 轮询间隔秒数，默认 10
#   VERBOSE       =1 打印响应体
#   NEWAPI_GROUP  令牌分组（仅用于预检提示）
#
set -uo pipefail

BASE_URL="${1:-${NEWAPI_BASE_URL:-}}"
TOKEN="${2:-${NEWAPI_TOKEN:-}}"
SKIP_E2E="${SKIP_E2E:-0}"
VERBOSE="${VERBOSE:-0}"
POLL_TIMEOUT="${POLL_TIMEOUT:-900}"
POLL_INTERVAL="${POLL_INTERVAL:-10}"
NEWAPI_GROUP="${NEWAPI_GROUP:-default}"

# 默认四个模型全测
DEFAULT_MODELS="minimax-h3,seedance-2.0,seedance-2.5,grok-imagine-video-1.5"
MODELS="${MODELS:-$DEFAULT_MODELS}"
EXTRA_MODELS="${EXTRA_MODELS:-}"

# ---------- 基础检查 ----------
if [[ -z "$BASE_URL" ]]; then
  echo "用法: $0 <API_BASE> <API_KEY>" >&2
  echo "  或: export NEWAPI_BASE_URL=... NEWAPI_TOKEN=... && $0" >&2
  echo "示例: $0 http://127.0.0.1:3000 sk-xxxxx" >&2
  exit 2
fi
if [[ -z "$TOKEN" ]]; then
  echo "错误：请提供 API Key（位置参数 2 或 NEWAPI_TOKEN），即 new-api 生成的 sk- 令牌" >&2
  exit 2
fi
command -v curl >/dev/null || { echo "错误：需要 curl" >&2; exit 2; }
command -v python3 >/dev/null || { echo "错误：需要 python3" >&2; exit 2; }
BASE_URL="${BASE_URL%/}"

PASS=0
FAIL=0
SKIP=0
FAILED_CASES=()

c_green() { printf '\033[32m%s\033[0m' "$1"; }
c_red()   { printf '\033[31m%s\033[0m' "$1"; }
c_dim()   { printf '\033[2m%s\033[0m' "$1"; }
c_yellow(){ printf '\033[33m%s\033[0m' "$1"; }
hr() { printf '%s\n' "------------------------------------------------------------"; }
section() { printf '\n==> %s\n' "$1"; }

ok()   { PASS=$((PASS+1)); printf '  %s %s\n' "$(c_green '✓')" "$1"; }
bad()  { FAIL=$((FAIL+1)); FAILED_CASES+=("$1"); printf '  %s %s\n' "$(c_red '✗')" "$1"; }
skip() { SKIP=$((SKIP+1)); printf '  %s %s\n' "$(c_dim '-')" "$1"; }

# json_get <python-expr-on-d>（JSON 从 stdin 读取）
json_get() {
  python3 -c '
import json,sys
expr = sys.argv[1] if len(sys.argv) > 1 else ""
try:
    d = json.loads(sys.stdin.read())
except Exception:
    print(""); sys.exit(0)
try:
    v = eval(expr, {"d": d})
    print("" if v is None else v)
except Exception:
    print("")
' "${1:-}"
}

# post_video <body-json> -> 输出 "HTTP_CODE<TAB>BODY"
post_video() {
  local body="$1" out
  out=$(curl -sS -w $'\n%{http_code}' -X POST "$BASE_URL/v1/videos/generations" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --max-time 120 \
    -d "$body" 2>&1)
  printf '%s\t%s' "${out##*$'\n'}" "${out%$'\n'*}"
}

get_task() {
  local tid="$1" out
  out=$(curl -sS -w $'\n%{http_code}' "$BASE_URL/v1/videos/tasks/$tid" \
    -H "Authorization: Bearer $TOKEN" --max-time 60 2>&1)
  printf '%s\t%s' "${out##*$'\n'}" "${out%$'\n'*}"
}

# ---------- 预检 ----------
preflight() {
  local res code resp
  res=$(post_video '{"model":"seedance-2.5","prompt":"__preflight__","duration":4}')
  code="${res%%$'\t'*}"; resp="${res#*$'\t'}"

  if grep -q '"model_not_found"' <<<"$resp"; then
    {
      echo ""
      echo "错误：模型 seedance-2.5 没有可用渠道，无法测试。"
      echo ""
      echo "请在 new-api 管理后台完成配置："
      echo "  1. 新建渠道，类型选 'Td'，填土豆 API Key"
      echo "  2. Base URL 填 https://api.ai-tudou.net（或你的上游地址）"
      echo "  3. 渠道的『模型』字段填入：$DEFAULT_MODELS"
      echo "  4. 确认渠道已启用，且令牌分组可访问该渠道"
      echo ""
      echo "当前令牌分组：${NEWAPI_GROUP}"
      echo "原始响应：$resp"
    } >&2
    exit 2
  fi

  if [[ "$code" == "200" || "$code" == "400" ]]; then
    ok "渠道可用（预检 HTTP ${code}）"
  else
    bad "预检返回异常 HTTP $code"
    printf '      %s\n' "$(c_dim "$resp")"
    exit 2
  fi
}

# ---------- 校验用例 ----------
# expect_400 <用例名> <body>
expect_400() {
  local name="$1" body="$2" res code resp
  res=$(post_video "$body")
  code="${res%%$'\t'*}"; resp="${res#*$'\t'}"
  if [[ "$code" == "400" ]]; then
    ok "$name"
    [[ "$VERBOSE" == "1" ]] && printf '      %s\n' "$(c_dim "$resp")"
  else
    bad "$name"
    printf '      期望 HTTP 400，实际 %s\n' "$code"
    printf '      %s\n' "$(c_dim "$resp")"
  fi
}

# expect_200 <用例名> <body>
expect_200() {
  local name="$1" body="$2" res code resp
  res=$(post_video "$body")
  code="${res%%$'\t'*}"; resp="${res#*$'\t'}"
  if [[ "$code" == "200" ]]; then
    ok "$name"
    [[ "$VERBOSE" == "1" ]] && printf '      %s\n' "$(c_dim "$resp")"
  else
    bad "$name"
    printf '      期望 HTTP 200，实际 %s\n' "$code"
    printf '      %s\n' "$(c_dim "$resp")"
  fi
}

# ---------- 输出头 ----------
cat <<EOF
==> Td 渠道视频模型全量测试
    BASE_URL = $BASE_URL
    MODELS   = $MODELS
EOF
[[ -n "$EXTRA_MODELS" ]] && echo "    EXTRA    = $EXTRA_MODELS"
hr

section "[1/5] 预检：渠道可用性"
preflight
hr

# ============================================================
# [2/5] 能力矩阵校验用例
# ============================================================
section "[2/5] 能力矩阵校验用例（均期望 HTTP 400）"

has_model() { [[ ",$MODELS,$EXTRA_MODELS," == *",$1,"* ]]; }

# ---------- minimax-h3 ----------
if has_model "minimax-h3"; then
  printf '  %s\n' "$(c_dim 'minimax-h3')"
  expect_400 "h3 时长 2 秒（低于下限 3）" \
    '{"model":"minimax-h3","prompt":"p","duration":2}'
  expect_400 "h3 时长 16 秒（高于上限 15）" \
    '{"model":"minimax-h3","prompt":"p","duration":16}'
  expect_400 "h3 传 videos（文档明确不支持）" \
    '{"model":"minimax-h3","prompt":"p","videos":["https://example.com/a.mp4"]}'
  expect_400 "h3 传 generate_audio（文档未列该参数）" \
    '{"model":"minimax-h3","prompt":"p","generate_audio":true}'
  expect_400 "h3 分辨率 4k（不在 720p/1080p/2k 内）" \
    '{"model":"minimax-h3","prompt":"p","resolution":"4k"}'
  expect_400 "h3 参考图 10 张（上限 9）" \
    '{"model":"minimax-h3","prompt":"p","images":["1","2","3","4","5","6","7","8","9","10"]}'
  expect_400 "h3 比例 1:1（不支持）" \
    '{"model":"minimax-h3","prompt":"p","aspect_ratio":"1:1"}'
  # 正向：边界内应通过
  expect_200 "h3 时长 3 秒（下界含）" \
    '{"model":"minimax-h3","prompt":"p","duration":3,"resolution":"720p"}'
  expect_200 "h3 分辨率 2k（支持）" \
    '{"model":"minimax-h3","prompt":"p","duration":4,"resolution":"2k"}'
fi

# ---------- seedance-2.0 ----------
if has_model "seedance-2.0"; then
  printf '  %s\n' "$(c_dim 'seedance-2.0')"
  expect_400 "s20 时长 16 秒（高于上限 15）" \
    '{"model":"seedance-2.0","prompt":"p","duration":16}'
  expect_400 "s20 时长 2 秒（低于下限 3）" \
    '{"model":"seedance-2.0","prompt":"p","duration":2}'
  expect_400 "s20 分辨率 1080p（仅支持 720p）" \
    '{"model":"seedance-2.0","prompt":"p","resolution":"1080p"}'
  expect_400 "s20 参考视频 4 个（上限 3）" \
    '{"model":"seedance-2.0","prompt":"p","images":["https://x/1.png"],"videos":["1","2","3","4"]}'
  expect_400 "s20 参考音频 4 个（上限 3）" \
    '{"model":"seedance-2.0","prompt":"p","images":["https://x/1.png"],"audios":["1","2","3","4"]}'
  expect_400 "s20 参考图 10 张（上限 9）" \
    '{"model":"seedance-2.0","prompt":"p","images":["1","2","3","4","5","6","7","8","9","10"]}'
  expect_200 "s20 时长 15 秒（上界含）" \
    '{"model":"seedance-2.0","prompt":"p","duration":15}'
  expect_200 "s20 generate_audio（支持）" \
    '{"model":"seedance-2.0","prompt":"p","duration":5,"generate_audio":true}'
fi

# ---------- seedance-2.5 ----------
if has_model "seedance-2.5"; then
  printf '  %s\n' "$(c_dim 'seedance-2.5')"
  expect_400 "s25 时长 31 秒（高于上限 30）" \
    '{"model":"seedance-2.5","prompt":"p","duration":31}'
  expect_400 "s25 时长 3 秒（低于下限 4）" \
    '{"model":"seedance-2.5","prompt":"p","duration":3}'
  expect_400 "s25 分辨率 1080p（仅支持 720p）" \
    '{"model":"seedance-2.5","prompt":"p","resolution":"1080p"}'
  S25_IMG31=$(python3 -c '
import json
print(json.dumps({"model": "seedance-2.5", "prompt": "p",
                  "images": [str(i) for i in range(31)]}))')
  expect_400 "s25 参考图 31 张（上限 30）" "$S25_IMG31"
  expect_400 "s25 音频单独使用（需配合图/视频）" \
    '{"model":"seedance-2.5","prompt":"p","audios":["https://x/a.mp3"]}'
  expect_200 "s25 时长 30 秒（上界含）" \
    '{"model":"seedance-2.5","prompt":"p","duration":30}'
  expect_200 "s25 时长 4 秒（下界含）" \
    '{"model":"seedance-2.5","prompt":"p","duration":4}'
fi

# ---------- grok-imagine-video-1.5 ----------
if has_model "grok-imagine-video-1.5"; then
  printf '  %s\n' "$(c_dim 'grok-imagine-video-1.5')"
  # 枚举时长：区间内但不在枚举集合，必须拒绝
  expect_400 "grok 时长 7 秒（区间内但非枚举 6/10/15）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":7}'
  expect_400 "grok 时长 14 秒（同上）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":14}'
  expect_400 "grok 时长 5 秒（低于最小档 6）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":5}'
  expect_400 "grok 时长 16 秒（高于最大档 15）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":16}'
  # duration 必填
  expect_400 "grok 未传 duration（必填）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p"}'
  # images 字段不支持
  expect_400 "grok 传 images（上游无该字段）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"images":["https://x/1.png"]}'
  # 单图误用 reference_images
  expect_400 "grok 单图走 extra.reference_images（应改用 input_reference）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"extra":{"aspect_ratio":"16:9","resolution":"720p","reference_images":[{"url":"https://x/1.png","role":"first_frame"}]}}'
  # reference_images 超限
  GROK_REF8=$(python3 -c '
import json
print(json.dumps({
    "model": "grok-imagine-video-1.5", "prompt": "p", "duration": 6,
    "extra": {"aspect_ratio": "16:9", "resolution": "720p",
              "reference_images": [{"url": "https://x/%d.png" % i}
                                   for i in range(8)]},
}))')
  expect_400 "grok extra.reference_images 8 张（上限 7）" "$GROK_REF8"
  # 不支持的能力
  expect_400 "grok 分辨率 1080p（仅 720p）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"resolution":"1080p"}'
  expect_400 "grok 传 videos（不支持）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"videos":["https://x/a.mp4"]}'
  expect_400 "grok 传 generate_audio（不支持）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"generate_audio":true}'
  expect_400 "grok 比例 4:3（仅 16:9/9:16/1:1）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"aspect_ratio":"4:3"}'

  # 正向
  expect_200 "grok 时长 6（枚举首档）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"extra":{"aspect_ratio":"16:9","resolution":"720p"}}'
  expect_200 "grok 时长 15（枚举末档）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":15,"extra":{"aspect_ratio":"16:9","resolution":"720p"}}'
  expect_200 "grok 比例 1:1（独有）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":6,"aspect_ratio":"1:1","extra":{"aspect_ratio":"1:1","resolution":"720p"}}'
  expect_200 "grok input_reference 字符串写法" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":10,"input_reference":"https://example.com/first.jpg","extra":{"aspect_ratio":"16:9","resolution":"720p"}}'
  expect_200 "grok input_reference 对象写法" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":10,"input_reference":{"url":"https://example.com/first.jpg"},"extra":{"aspect_ratio":"16:9","resolution":"720p"}}'
  expect_200 "grok extra.reference_images 2 张（首尾帧）" \
    '{"model":"grok-imagine-video-1.5","prompt":"p","duration":10,"extra":{"aspect_ratio":"16:9","resolution":"720p","reference_images":[{"url":"https://x/a.png","role":"first_frame"},{"url":"https://x/b.png","role":"last_frame"}]}}'
  # 注意：先用 python3 生成完整 payload 存入变量，不要在双引号字符串里
  # 混用 \" 转义与 $( ) 命令替换 —— 嵌套引号会被 bash 先行解析导致 JSON 损坏。
  GROK_REF7=$(python3 -c '
import json
print(json.dumps({
    "model": "grok-imagine-video-1.5",
    "prompt": "p",
    "duration": 6,
    "extra": {
        "aspect_ratio": "16:9",
        "resolution": "720p",
        "reference_images": [
            {"url": "https://x/%d.png" % i, "role": "reference_image"}
            for i in range(7)
        ],
    },
}))')
  expect_200 "grok extra.reference_images 7 张（上限含）" "$GROK_REF7"
fi

# ---------- 额外模型（同一套宽松口径） ----------
if [[ -n "$EXTRA_MODELS" ]]; then
  IFS=',' read -ra EXTRA_ARR <<< "$EXTRA_MODELS"
  for m in "${EXTRA_ARR[@]}"; do
    [[ -z "$m" ]] && continue
    printf '  %s\n' "$(c_dim "$m (额外)")"
    expect_400 "$m prompt 为空" "{\"model\":\"$m\",\"prompt\":\"\"}"
    expect_400 "$m 比例非法 4:3" "{\"model\":\"$m\",\"prompt\":\"p\",\"aspect_ratio\":\"4:3\"}"
    expect_200 "$m 基础文生视频" "{\"model\":\"$m\",\"prompt\":\"p\"}"
  done
fi
hr

# ============================================================
# [3/5] & [4/5] 真实提交与轮询
# ============================================================
if [[ "$SKIP_E2E" == "1" ]]; then
  section "[3/5] SKIP_E2E=1，跳过实际提交与轮询"
  hr
else
  section "[3/5] 各模型真实提交 + [4/5] 轮询至终态"

  run_e2e() {
    local model="$1" payload="$2"
    printf '  %s\n' "$(c_dim "$model")"
    printf '    payload: %s\n' "$(c_dim "$payload")"

    local res code body tid obj status
    res=$(post_video "$payload")
    code="${res%%$'\t'*}"; body="${res#*$'\t'}"
    printf '    HTTP %s\n' "$code"
    [[ "$VERBOSE" == "1" ]] && printf '    %s\n' "$(c_dim "$body")"

    tid=$(json_get 'd.get("id","")' <<<"$body")
    obj=$(json_get 'd.get("object","")' <<<"$body")
    status=$(json_get 'd.get("status","")' <<<"$body")

    if [[ "$code" != "200" || -z "$tid" ]]; then
      bad "$model 提交失败（HTTP ${code}）"
      printf '      %s\n' "$(c_dim "$body")"
      return
    fi
    ok "$model 提交成功 task_id=$tid"

    if [[ "$obj" == "video.task" ]]; then
      ok "$model 提交响应 object=video.task"
    else
      bad "$model 提交响应 object 应为 video.task，实际 $obj"
    fi
    if [[ "$status" == "submitted" ]]; then
      ok "$model 提交响应 status=submitted"
    else
      bad "$model 提交响应 status 应为 submitted，实际 $status"
    fi

    # 轮询
    local elapsed=0 qres qcode qstatus vurl final=""
    while (( elapsed < POLL_TIMEOUT )); do
      qres=$(get_task "$tid")
      qcode="${qres%%$'\t'*}"; local qbody="${qres#*$'\t'}"
      [[ "$VERBOSE" == "1" ]] && printf '      %s\n' "$(c_dim "$qbody")"
      qstatus=$(json_get 'd.get("status","")' <<<"$qbody")
      printf '      [%4ds] HTTP %s status=%s\n' "$elapsed" "$qcode" "${qstatus:-?}"

      if [[ "$qstatus" == "succeeded" || "$qstatus" == "failed" ]]; then
        final="$qstatus"
        if [[ "$(json_get 'd.get("object","")' <<<"$qbody")" == "video.task" ]]; then
          ok "$model 查询响应 object=video.task"
        else
          bad "$model 查询响应 object 应为 video.task"
        fi
        break
      fi
      sleep "$POLL_INTERVAL"
      elapsed=$((elapsed+POLL_INTERVAL))
    done

    if [[ -z "$final" ]]; then
      bad "$model 轮询超时（${POLL_TIMEOUT}s 内未到终态）"
    elif [[ "$final" == "failed" ]]; then
      # 上游失败不算脚本失败（可能内容审核），但要如实报告
      skip "$model 任务失败：$(json_get 'd.get("error",{}).get("message","") if isinstance(d.get("error"),dict) else ""' <<<"$qbody")"
    else
      ok "$model 任务完成"
      vurl=$(json_get 'd.get("result",{}).get("video_url","") if isinstance(d.get("result"),dict) else ""' <<<"$qbody")
      if [[ -n "$vurl" ]]; then
        ok "$model 返回 result.video_url"
        printf '      %s\n' "$(c_dim "$vurl")"
        # 顺带验证 videos[] 兼容字段
        varr=$(json_get 'len(d.get("result",{}).get("videos",[])) if isinstance(d.get("result"),dict) else 0' <<<"$qbody")
        if [[ "$varr" != "0" ]]; then
          ok "$model 同时返回 result.videos[]"
        else
          bad "$model 缺少 result.videos[]"
        fi
      else
        bad "$model 完成态未返回 result.video_url"
      fi
    fi
    printf '\n'
  }

  # 各模型用各自合法的参数提交
  if has_model "minimax-h3"; then
    run_e2e "minimax-h3" \
      '{"model":"minimax-h3","prompt":"cinematic ocean waves at sunset, slow drone shot","duration":3,"resolution":"720p","aspect_ratio":"16:9"}'
  fi
  if has_model "seedance-2.0"; then
    run_e2e "seedance-2.0" \
      '{"model":"seedance-2.0","prompt":"cinematic ocean waves at sunset, slow drone shot","duration":5,"aspect_ratio":"16:9"}'
  fi
  if has_model "seedance-2.5"; then
    run_e2e "seedance-2.5" \
      '{"model":"seedance-2.5","prompt":"cinematic ocean waves at sunset, slow drone shot","duration":4,"aspect_ratio":"16:9"}'
  fi
  if has_model "grok-imagine-video-1.5"; then
    run_e2e "grok-imagine-video-1.5" \
      '{"model":"grok-imagine-video-1.5","prompt":"cinematic ocean waves at sunset, slow drone shot","duration":6,"aspect_ratio":"16:9","resolution":"720p","extra":{"aspect_ratio":"16:9","resolution":"720p"}}'
  fi
fi

# ============================================================
# [5/5] 图片任务回归
# ============================================================
section "[5/5] 图片任务回归（/v1/images/tasks 结构应保持 image.task）"

IMG_SUBMIT=$(curl -sS -w $'\n%{http_code}' -X POST "$BASE_URL/v1/images/tasks" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  --max-time 120 \
  -d '{"model":"gpt-image-2-all","prompt":"a red apple on a white table","resolution":"1k","quality":"medium"}' 2>&1)
IMG_CODE="${IMG_SUBMIT##*$'\n'}"; IMG_BODY="${IMG_SUBMIT%$'\n'*}"

if [[ "$IMG_CODE" == "200" ]]; then
  IMG_ID=$(json_get 'd.get("id","")' <<<"$IMG_BODY")
  IMG_OBJ=$(json_get 'd.get("object","")' <<<"$IMG_BODY")
  if [[ "$IMG_OBJ" == "image.task" ]]; then
    ok "图片提交 object=image.task（未被视频结构污染）"
  else
    bad "图片提交 object 应为 image.task，实际 $IMG_OBJ"
  fi
  if [[ -n "$IMG_ID" && "$SKIP_E2E" != "1" ]]; then
    sleep 5
    QRES=$(curl -sS -w $'\n%{http_code}' "$BASE_URL/v1/images/tasks/$IMG_ID" \
      -H "Authorization: Bearer $TOKEN" --max-time 60 2>&1)
    QBODY="${QRES%$'\n'*}"
    QOBJ=$(json_get 'd.get("object","")' <<<"$QBODY")
    if [[ "$QOBJ" == "image.task" ]]; then
      ok "图片查询 object=image.task"
    else
      bad "图片查询 object 应为 image.task，实际 $QOBJ"
    fi
  fi
else
  skip "图片渠道不可用，跳过图片回归（HTTP ${IMG_CODE}）"
fi
hr

# ============================================================
# 汇总
# ============================================================
printf '\n==> 测试结果\n'
printf '    通过 %s   失败 %s   跳过 %s\n' "$(c_green "$PASS")" "$(c_red "$FAIL")" "$(c_yellow "$SKIP")"

if (( FAIL > 0 )); then
  printf '\n    失败用例：\n'
  for c in "${FAILED_CASES[@]}"; do
    printf '      - %s\n' "$c"
  done
  exit 1
fi

echo ""
printf '    %s\n' "$(c_green '全部通过')"
exit 0
