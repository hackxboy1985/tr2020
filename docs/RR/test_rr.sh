#!/usr/bin/env bash
# RR 渠道接口测试脚本  模型: image-g2
#
# 用法：
#   ./test_rr.sh                          # 执行全部用例
#   ./test_rr.sh t2i [参数...]            # 文生图
#   ./test_rr.sh i2i [参数...]            # 图生图
#   ./test_rr.sh task-query <task_id>     # 查询任务状态
#
# 示例：
#   ./test_rr.sh t2i
#   ./test_rr.sh t2i --prompt "a cat" --size 1280x720 --quality hd
#   ./test_rr.sh i2i --prompt "将背景换成雪山" --img https://example.com/photo.jpg
#   ./test_rr.sh task-query task_abc123

# ──────────────────────────────────────────────
# 配置区（必填）
# ──────────────────────────────────────────────
GATEWAY="http://ai.passorico.com"
API_KEY="sk-TmnBitxnzFMupKgxalfkQA34jJwGpXynfIwYfoxe8OVgqEOc"

MODEL="image-g2"

# 默认参数
DEFAULT_PROMPT="A serene mountain landscape at sunrise, photorealistic, 8k"
DEFAULT_SIZE="1024x1024"
DEFAULT_QUALITY="standard"
DEFAULT_IMG="https://images.unsplash.com/photo-1506905925346-21bda4d32df4?w=800"

# 轮询配置
POLL_MAX=24        # 最多轮询次数
POLL_INTERVAL=5    # 每次间隔秒数

# ──────────────────────────────────────────────
# 工具函数
# ──────────────────────────────────────────────
GREEN="\033[32m"; RED="\033[31m"; YELLOW="\033[33m"; RESET="\033[0m"

ok()   { echo -e "${GREEN}[PASS]${RESET} $*"; }
fail() { echo -e "${RED}[FAIL]${RESET} $*"; }
info() { echo -e "${YELLOW}[INFO]${RESET} $*"; }
sep()  { echo -e "\n────────────────────────────────────────"; }

parse_args() {
    ARG_PROMPT=""
    ARG_SIZE=""
    ARG_QUALITY=""
    ARG_IMG=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --prompt)  ARG_PROMPT="$2";  shift 2 ;;
            --size)    ARG_SIZE="$2";    shift 2 ;;
            --quality) ARG_QUALITY="$2"; shift 2 ;;
            --img)     ARG_IMG="$2";     shift 2 ;;
            *) shift ;;
        esac
    done
}

# 提交图像任务并自动轮询
submit_and_poll() {
    local label="$1"
    local body="$2"

    sep; echo "$label"
    info "POST $GATEWAY/v1/images/generations  model=$MODEL"
    echo "── 请求报文 ──────────────────────────"
    echo "$body" | python3 -m json.tool 2>/dev/null || echo "$body"

    resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/generations" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$body")
    http_code=$(echo "$resp" | tail -1)
    resp_body=$(echo "$resp" | sed '$d')

    echo ""
    echo "── 响应报文 ──────────────────────────"
    echo "HTTP $http_code"
    echo "$resp_body" | python3 -m json.tool 2>/dev/null || echo "$resp_body"

    if [ "$http_code" != "200" ]; then
        fail "$label  HTTP=$http_code"
        return
    fi

    # 尝试从响应中提取 task id（兼容 id / task_id 字段名）
    TASK_ID=$(echo "$resp_body" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(d.get('id') or d.get('task_id') or '')
" 2>/dev/null || true)

    if [ -z "$TASK_ID" ]; then
        # 响应里没有 task id，可能是同步返回了结果
        has_url=$(echo "$resp_body" | python3 -c "
import sys, json
d = json.load(sys.stdin)
data = d.get('data', [])
print('yes' if data and (data[0].get('url') or data[0].get('b64_json')) else 'no')
" 2>/dev/null || echo "no")
        if [ "$has_url" = "yes" ]; then
            ok "$label  (同步返回结果)"
        else
            fail "$label  响应中无 task id 也无图片数据"
        fi
        return
    fi

    ok "$label 提交成功，task_id=$TASK_ID"
    poll_task "$TASK_ID"
}

# 轮询任务直到完成或超时
poll_task() {
    local task_id="$1"
    sep; echo "轮询任务结果（最多等待 $((POLL_MAX * POLL_INTERVAL))s）  task_id=$task_id"

    for i in $(seq 1 "$POLL_MAX"); do
        sleep "$POLL_INTERVAL"
        info "第 $i 次轮询... GET /v1/images/tasks/$task_id"
        poll_resp=$(curl -s -w "\n%{http_code}" "$GATEWAY/v1/images/tasks/$task_id" \
            -H "Authorization: Bearer $API_KEY")
        poll_code=$(echo "$poll_resp" | tail -1)
        poll_body=$(echo "$poll_resp" | sed '$d')
        echo "HTTP $poll_code"
        echo "$poll_body" | python3 -m json.tool 2>/dev/null || echo "$poll_body"

        status=$(echo "$poll_body" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(d.get('status',''))
" 2>/dev/null || true)

        case "$status" in
            succeeded|SUCCESS|success)
                ok "任务完成  status=$status"
                return ;;
            failed|FAILED|fail)
                fail "任务失败  status=$status"
                return ;;
            *)
                info "当前状态: ${status:-未知}，继续等待..."
                if [ "$i" = "$POLL_MAX" ]; then
                    echo ""
                    echo -e "${YELLOW}轮询超时，任务仍在进行中。可手动继续查询：${RESET}"
                    echo "  bash test_rr.sh task-query $task_id"
                fi
                ;;
        esac
    done
}

# ──────────────────────────────────────────────
# 用例函数
# ──────────────────────────────────────────────

test_t2i() {
    parse_args "$@"
    local prompt="${ARG_PROMPT:-$DEFAULT_PROMPT}"
    local size="${ARG_SIZE:-$DEFAULT_SIZE}"
    local quality="${ARG_QUALITY:-$DEFAULT_QUALITY}"

    submit_and_poll "文生图  size=$size  quality=$quality" "$(cat <<EOF
{
  "model": "$MODEL",
  "prompt": "$prompt",
  "size": "$size",
  "quality": "$quality"
}
EOF
)"
}

test_i2i() {
    parse_args "$@"
    local prompt="${ARG_PROMPT:-根据参考图生成一张类似风格的图片，背景换成城市夜景}"
    local size="${ARG_SIZE:-$DEFAULT_SIZE}"
    local quality="${ARG_QUALITY:-$DEFAULT_QUALITY}"
    local img="${ARG_IMG:-$DEFAULT_IMG}"

    submit_and_poll "图生图  img=$(echo "$img" | cut -c1-60)..." "$(cat <<EOF
{
  "model": "$MODEL",
  "prompt": "$prompt",
  "size": "$size",
  "quality": "$quality",
  "images": ["$img"]
}
EOF
)"
}

# ──────────────────────────────────────────────
# 入口
# ──────────────────────────────────────────────
case "$1" in
    t2i)
        shift; test_t2i "$@" ;;
    i2i)
        shift; test_i2i "$@" ;;
    task-query)
        if [ -z "$2" ]; then
            echo -e "${RED}用法: bash test_rr.sh task-query <task_id>${RESET}"
            exit 1
        fi
        poll_task "$2" ;;
    all|"")
        info "=== RR 渠道全量测试  model=$MODEL ==="

        # 文生图：标准质量 1:1
        test_t2i --size 1024x1024 --quality standard

        # 文生图：高质量 16:9
        test_t2i --size 1280x720 --quality hd

        # 文生图：竖图 9:16
        test_t2i --size 720x1280 --quality standard

        # 图生图
        test_i2i

        sep; echo -e "${GREEN}=== 全量测试结束 ===${RESET}" ;;
    *)
        echo "用法:"
        echo "  bash test_rr.sh                              # 全量测试"
        echo "  bash test_rr.sh t2i [--prompt P] [--size S] [--quality Q]"
        echo "  bash test_rr.sh i2i [--prompt P] [--img URL] [--size S] [--quality Q]"
        echo "  bash test_rr.sh task-query <task_id>"
        echo ""
        echo "size 可选值（部分）:"
        echo "  1024x1024  2048x2048  (1:1)"
        echo "  1280x720   1920x1080  (16:9)"
        echo "  720x1280   1080x1920  (9:16)"
        echo "  1024x768   1024x1360  (4:3 / 3:4)"
        echo ""
        echo "quality 可选值: standard(默认)  hd"
        ;;
esac
