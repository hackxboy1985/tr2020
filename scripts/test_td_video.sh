#!/usr/bin/env bash
#
# Td 视频模型调用测试脚本（调用本机 new-api，非直达上游）
#
# 覆盖：
#   1. 能力矩阵校验（未知模型、时长越界、分辨率不合法、素材不支持等应返回 400）
#   2. 视频任务提交（POST /v1/videos/generations）
#   3. 视频任务轮询（GET  /v1/videos/tasks/{task_id}）直到 completed
#   4. 图片任务回归（GET /v1/images/tasks 结构应保持 image.task）
#
# 用法：
#   ./scripts/test_td_video.sh <API_BASE> <API_KEY>          # 位置参数
#   或
#   export NEWAPI_BASE_URL=http://127.0.0.1:3000
#   export NEWAPI_TOKEN=sk-xxxxxxxx
#   ./scripts/test_td_video.sh
#
# 示例：
#   ./scripts/test_td_video.sh http://127.0.0.1:3000 sk-xxxxx
#
# 可选环境变量：
#   MODEL        默认 seedance-2.5
#   DURATION     默认 4
#   PROMPT       默认一段测试提示词
#   IMAGES       参考图 URL，逗号分隔（可选）
#   POLL_TIMEOUT 轮询超时秒数，默认 600
#   SKIP_E2E     =1 时只跑校验用例，不实际提交任务（不消耗额度）
#   SPEC_PROBE_MODEL
#                用于验证 videoModelSpecs 的拒绝逻辑：填一个「渠道已配置、
#                但未登记在 videoModelSpecs」的模型名。留空则跳过该用例。
#                注意不要填完全没配渠道的名字 —— 那种情况分发器会先返回 503，
#                请求到不了适配器，测不到 spec 层。
#   VERBOSE      =1 时打印响应体
#
set -uo pipefail

BASE_URL="${1:-${NEWAPI_BASE_URL:-}}"
TOKEN="${2:-${NEWAPI_TOKEN:-}}"
MODEL="${MODEL:-seedance-2.5}"
DURATION="${DURATION:-4}"
PROMPT="${PROMPT:-cinematic ocean waves at sunset, slow drone shot}"
IMAGES="${IMAGES:-}"
POLL_TIMEOUT="${POLL_TIMEOUT:-600}"
SKIP_E2E="${SKIP_E2E:-0}"
VERBOSE="${VERBOSE:-0}"

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
FAILED_CASES=()

c_green() { printf '\033[32m%s\033[0m' "$1"; }
c_red()   { printf '\033[31m%s\033[0m' "$1"; }
c_dim()   { printf '\033[2m%s\033[0m' "$1"; }

hr() { printf '%s\n' "------------------------------------------------------------"; }

# json_get <python-expr-on-d>   （JSON 从 stdin 读取）
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

# post_video <body-json>  -> 输出 "HTTP_CODE<TAB>BODY"
post_video() {
  local body="$1"
  local out
  out=$(curl -sS -w $'\n%{http_code}' -X POST "$BASE_URL/v1/videos/generations" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --max-time 120 \
    -d "$body" 2>&1)
  local code="${out##*$'\n'}"
  local resp="${out%$'\n'*}"
  printf '%s\t%s' "$code" "$resp"
}

# 预检：确认 Td 渠道确实可用。
#
# 为什么需要：校验用例期望 400，但如果根本没人配置 Td 渠道，分发器会先返回
# 503「No available channel」，校验代码压根没执行，9 个用例会全部「失败」，
# 排查方向被完全带偏。这里提前识别并给出明确指引。
preflight() {
  local res code resp
  res=$(post_video "{\"model\":\"$MODEL\",\"prompt\":\"__preflight__\"}")
  code="${res%%$'\t'*}"; resp="${res#*$'\t'}"

  if grep -q '"model_not_found"' <<<"$resp"; then
    {
      echo "错误：模型 $MODEL 没有可用渠道，无法测试。"
      echo ""
      echo "请在 new-api 管理后台完成配置："
      echo "  1. 新建渠道，类型选 'Td'，填土豆 API Key"
      echo "  2. Base URL 填 $BASE_URL 对应的上游地址（默认 https://api.ai-tudou.net）"
      echo "  3. 渠道的『模型』字段填入：minimax-h3,seedance-2.0,seedance-2.5"
      echo "  4. 确认渠道已启用，且当前令牌所在分组可访问该渠道"
      echo ""
      echo "当前令牌分组：${NEWAPI_GROUP:-default}（可用 NEWAPI_GROUP 覆盖）"
      echo "原始响应：$resp"
    } >&2
    exit 2
  fi

  # 模型能路由到渠道即视为通过；400/200 都说明校验链路已生效
  if [[ "$code" == "200" || "$code" == "400" ]]; then
    printf '  %s 渠道可用（预检 HTTP %s）\n' "$(c_green '✓')" "$code"
  else
    printf '  %s 预检返回异常 HTTP %s\n' "$(c_red '✗')" "$code"
    printf '      %s\n' "$(c_dim "$resp")"
    exit 2
  fi
}

# ---------- 1. 校验用例（应全部返回 400） ----------
expect_400() {
  local name="$1" body="$2"
  local res code resp
  res=$(post_video "$body")
  code="${res%%$'\t'*}"; resp="${res#*$'\t'}"

  if [[ "$code" == "400" ]]; then
    PASS=$((PASS+1))
    printf '  %s %s\n' "$(c_green '✓')" "$name"
    [[ "$VERBOSE" == "1" ]] && printf '      %s\n' "$(c_dim "$resp")"
  else
    FAIL=$((FAIL+1)); FAILED_CASES+=("$name")
    printf '  %s %s\n' "$(c_red '✗')" "$name"
    printf '      期望 HTTP 400，实际 %s\n' "$code"
    printf '      %s\n' "$(c_dim "$resp")"
  fi
}

echo "==> new-api 视频接口测试"
echo "    BASE_URL = $BASE_URL"
echo "    MODEL    = $MODEL"
hr

echo "[0/4] 预检：渠道可用性"
preflight
hr

echo "[1/4] 参数校验用例（均应 400）"

# 说明：这里**不能**用随便一个未知模型名来测「未知模型被拒绝」。
# 分发器在选渠道阶段就会因找不到渠道而返回 503，
# 请求根本到不了适配器，测不到 videoModelSpecs 的拒绝逻辑。
#
# 要真正测 spec 层，必须用一个「渠道已配置、但未登记在 videoModelSpecs」的模型名，
# 这正是上游新增模型后的真实场景。用 SPEC_PROBE_MODEL 指定，默认跳过。
if [[ -n "${SPEC_PROBE_MODEL:-}" ]]; then
  RES=$(post_video "{\"model\":\"$SPEC_PROBE_MODEL\",\"prompt\":\"p\"}")
  CODE="${RES%%$'\t'*}"; BODY="${RES#*$'\t'}"
  if [[ "$CODE" == "400" ]] && grep -q 'unsupported video model' <<<"$BODY"; then
    PASS=$((PASS+1))
    printf '  %s 未登记 spec 的模型被拒绝（%s）\n' "$(c_green '✓')" "$SPEC_PROBE_MODEL"
  else
    FAIL=$((FAIL+1)); FAILED_CASES+=("未登记 spec 的模型应被 400 拒绝")
    printf '  %s 未登记 spec 的模型应返回 400 unsupported video model，实际 HTTP %s\n' "$(c_red '✗')" "$CODE"
    printf '      %s\n' "$(c_dim "$BODY")"
  fi
else
  printf '  %s 跳过「未登记 spec 的模型」用例（设 SPEC_PROBE_MODEL=<已配渠道但未登记的模型名> 启用）\n' "$(c_dim '-')"
fi

expect_400 "minimax-h3 传 videos 被拒绝（文档明确不支持）" \
  '{"model":"minimax-h3","prompt":"p","videos":["https://example.com/a.mp4"]}'

expect_400 "minimax-h3 传 generate_audio 被拒绝（文档未列该参数）" \
  '{"model":"minimax-h3","prompt":"p","generate_audio":true}'

expect_400 "seedance-2.0 时长 16 秒越界被拒绝（上限 15）" \
  '{"model":"seedance-2.0","prompt":"p","duration":16}'

expect_400 "seedance-2.0 分辨率 1080p 被拒绝（仅支持 720p）" \
  '{"model":"seedance-2.0","prompt":"p","resolution":"1080p"}'

expect_400 "aspect_ratio 非法值被拒绝" \
  '{"model":"seedance-2.0","prompt":"p","aspect_ratio":"1:1"}'

expect_400 "prompt 为空被拒绝" \
  '{"model":"seedance-2.0","prompt":""}'

expect_400 "audio 单独使用被拒绝（需配合图片或视频）" \
  '{"model":"seedance-2.5","prompt":"p","audios":["https://example.com/a.mp3"]}'

expect_400 "参考图超过模型上限被拒绝（seedance-2.0 上限 9）" \
  '{"model":"seedance-2.0","prompt":"p","images":["1","2","3","4","5","6","7","8","9","10"]}'

hr

# ---------- 2. 提交 ----------
if [[ "$SKIP_E2E" == "1" ]]; then
  echo "[2/4] SKIP_E2E=1，跳过实际提交与轮询"
  hr
else
  echo "[2/4] 提交视频任务"

  # 构造 payload
  PAYLOAD=$(MODEL="$MODEL" DURATION="$DURATION" PROMPT="$PROMPT" IMAGES="$IMAGES" python3 -c '
import json,os
p={"model":os.environ["MODEL"],"prompt":os.environ["PROMPT"],"duration":int(os.environ["DURATION"])}
imgs=[u for u in os.environ.get("IMAGES","").split(",") if u.strip()]
if imgs: p["images"]=imgs
print(json.dumps(p,ensure_ascii=False))
')
  printf '    payload: %s\n' "$(c_dim "$PAYLOAD")"

  RES=$(post_video "$PAYLOAD")
  CODE="${RES%%$'\t'*}"; BODY="${RES#*$'\t'}"
  printf '    HTTP %s\n' "$CODE"
  [[ "$VERBOSE" == "1" ]] && printf '    %s\n' "$(c_dim "$BODY")"

  TASK_ID=$(json_get 'd.get("id","")' <<<"$BODY")
  OBJ=$(json_get 'd.get("object","")' <<<"$BODY")
  STATUS=$(json_get 'd.get("status","")' <<<"$BODY")

  SUBMIT_OK=0
  if [[ "$CODE" == "200" && -n "$TASK_ID" ]]; then
    SUBMIT_OK=1
    PASS=$((PASS+1))
    printf '  %s 提交成功 task_id=%s\n' "$(c_green '✓')" "$TASK_ID"
  else
    FAIL=$((FAIL+1)); FAILED_CASES+=("提交视频任务")
    printf '  %s 提交失败\n' "$(c_red '✗')"
  fi

  if [[ "$SUBMIT_OK" == "1" ]]; then
    if [[ "$OBJ" == "video.task" ]]; then
      PASS=$((PASS+1)); printf '  %s object=video.task\n' "$(c_green '✓')"
    else
      FAIL=$((FAIL+1)); FAILED_CASES+=("提交响应 object 应为 video.task")
      printf '  %s object 应为 video.task，实际 %s\n' "$(c_red '✗')" "$OBJ"
    fi
    if [[ "$STATUS" == "submitted" ]]; then
      PASS=$((PASS+1)); printf '  %s status=submitted\n' "$(c_green '✓')"
    else
      FAIL=$((FAIL+1)); FAILED_CASES+=("提交响应 status 应为 submitted")
      printf '  %s status 应为 submitted，实际 %s\n' "$(c_red '✗')" "$STATUS"
    fi
  fi
  hr

  # ---------- 3. 轮询 ----------
  if [[ "$SUBMIT_OK" == "1" ]]; then
    echo "[3/4] 轮询任务状态（超时 ${POLL_TIMEOUT}s）"
    ELAPSED=0
    INTERVAL=5
    FINAL_STATUS=""
    VIDEO_URL=""

    while (( ELAPSED < POLL_TIMEOUT )); do
      QRES=$(curl -sS -w $'\n%{http_code}' \
        "$BASE_URL/v1/videos/tasks/$TASK_ID" \
        -H "Authorization: Bearer $TOKEN" --max-time 60 2>&1)
      QCODE="${QRES##*$'\n'}"; QBODY="${QRES%$'\n'*}"
      [[ "$VERBOSE" == "1" ]] && printf '    %s\n' "$(c_dim "$QBODY")"

      QSTATUS=$(json_get 'd.get("status","")' <<<"$QBODY")
      VIDEO_URL=$(json_get 'd.get("result",{}).get("video_url","") if isinstance(d.get("result"),dict) else ""' <<<"$QBODY")

      printf '    [%4ds] HTTP %s status=%s\n' "$ELAPSED" "$QCODE" "${QSTATUS:-?}"

      if [[ "$QSTATUS" == "succeeded" || "$QSTATUS" == "failed" ]]; then
        FINAL_STATUS="$QSTATUS"
        OBJ2=$(json_get 'd.get("object","")' <<<"$QBODY")
        if [[ "$OBJ2" == "video.task" ]]; then
          PASS=$((PASS+1)); printf '  %s 查询响应 object=video.task\n' "$(c_green '✓')"
        else
          FAIL=$((FAIL+1)); FAILED_CASES+=("查询响应 object 应为 video.task")
          printf '  %s 查询响应 object 应为 video.task，实际 %s\n' "$(c_red '✗')" "$OBJ2"
        fi
        break
      fi

      sleep "$INTERVAL"
      ELAPSED=$((ELAPSED+INTERVAL))
    done

    if [[ -z "$FINAL_STATUS" ]]; then
      FAIL=$((FAIL+1)); FAILED_CASES+=("轮询超时")
      printf '  %s 轮询超时（%ss 内未到达终态）\n' "$(c_red '✗')" "$POLL_TIMEOUT"
    elif [[ "$FINAL_STATUS" == "failed" ]]; then
      FAIL=$((FAIL+1)); FAILED_CASES+=("任务执行失败")
      printf '  %s 任务失败：%s\n' "$(c_red '✗')" "$(json_get 'd.get("error",{}).get("message","") if isinstance(d.get("error"),dict) else ""' <<<"$QBODY")"
    else
      PASS=$((PASS+1))
      printf '  %s 任务完成\n' "$(c_green '✓')"
      if [[ -n "$VIDEO_URL" ]]; then
        PASS=$((PASS+1))
        printf '  %s 返回 result.video_url\n' "$(c_green '✓')"
        printf '      %s\n' "$VIDEO_URL"
      else
        FAIL=$((FAIL+1)); FAILED_CASES+=("完成态未返回 result.video_url")
        printf '  %s 完成态未返回 result.video_url\n' "$(c_red '✗')"
      fi
    fi
    hr
  else
    echo "[3/4] 提交失败，跳过轮询"
    hr
  fi
fi

# ---------- 4. 图片结构回归 ----------
echo "[4/4] 图片任务查询结构回归（/v1/images/tasks 应保持 image.task）"
IRES=$(curl -sS -w $'\n%{http_code}' \
  "$BASE_URL/v1/images/tasks/nonexistent_task_id" \
  -H "Authorization: Bearer $TOKEN" --max-time 30 2>&1)
ICODE="${IRES##*$'\n'}"; IBODY="${IRES%$'\n'*}"
IOBJ=$(json_get 'd.get("object","")' <<<"$IBODY")

# 不存在的任务返回业务错误是正常的，这里只确认它没有被视频结构污染
if grep -q '"video_url"' <<<"$IBODY"; then
  FAIL=$((FAIL+1)); FAILED_CASES+=("图片查询接口被视频结构污染")
  printf '  %s 图片接口响应出现了 video_url\n' "$(c_red '✗')"
else
  PASS=$((PASS+1))
  printf '  %s 图片接口未出现 video_url（HTTP %s）\n' "$(c_green '✓')" "$ICODE"
fi
hr

# ---------- 汇总 ----------
echo "==> 结果：$(c_green "通过 $PASS") / $(c_red "失败 $FAIL")"
if (( FAIL > 0 )); then
  echo "失败用例："
  for c in "${FAILED_CASES[@]}"; do echo "  - $c"; done
  exit 1
fi
echo "全部通过"
