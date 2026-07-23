#!/usr/bin/env bash
# 海报渠道接口测试脚本
# 用法：
#   ./test_poster.sh                    # 执行全部接口
#   ./test_poster.sh poster-matting     # 只执行 AI 抠图
#   ./test_poster.sh poster-enhance     # 只执行 AI 超清
#   ./test_poster.sh poster-generate    # 只执行异步海报生成（含轮询）
#
# 可用模型：
#   poster-matting          AI 抠图
#   poster-enlarge          无损放大
#   poster-enhance          AI 超清
#   poster-extension        智能延展
#   poster-translate        图片翻译
#   poster-partial-redraw   局部重绘
#   poster-scene-replace    场景替换
#   poster-product-replace  商品替换
#   poster-color-change     商品换色
#   poster-assisted         AI 帮写（返回文案）
#   poster-generate-sync    同步海报生成
#   poster-generate         异步海报生成（提交+轮询）
#   poster-free-creation    自由创作（提交）

# ──────────────────────────────────────────────
# 配置区（必填）
# ──────────────────────────────────────────────
GATEWAY="http://book:3002"          # new-api 网关地址
API_KEY="sk-xxx"                    # 你的 new-api Token

# 测试用图片 URL（需要公网可访问）
IMG_PRODUCT="https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=600"
IMG_BANNER="https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=800"

# ──────────────────────────────────────────────
# 工具函数
# ──────────────────────────────────────────────
GREEN="\033[32m"; RED="\033[31m"; YELLOW="\033[33m"; RESET="\033[0m"

ok()   { echo -e "${GREEN}[PASS]${RESET} $*"; }
fail() { echo -e "${RED}[FAIL]${RESET} $*"; }
info() { echo -e "${YELLOW}[INFO]${RESET} $*"; }
sep()  { echo -e "\n────────────────────────────────────────"; }

call_sync() {
    local model="$1"
    local body="$2"
    info "POST /v1/images/generations  model=$model"
    resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/generations" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$body")
    code=$(echo "$resp" | tail -1)
    body_resp=$(echo "$resp" | head -n -1)
    echo "HTTP $code"
    echo "$body_resp" | python3 -m json.tool 2>/dev/null || echo "$body_resp"
    if [ "$code" = "200" ]; then ok "$model"; else fail "$model  HTTP=$code"; fi
}

# ──────────────────────────────────────────────
# 各接口函数
# ──────────────────────────────────────────────

test_poster-matting() {
    sep; echo "poster-matting  AI 抠图"
    call_sync "poster-matting" '{
  "model": "poster-matting",
  "metadata": {
    "imgUrls": "'"$IMG_PRODUCT"'"
  }
}'
}

test_poster-enlarge() {
    sep; echo "poster-enlarge  无损放大"
    call_sync "poster-enlarge" '{
  "model": "poster-enlarge",
  "metadata": {
    "imgUrls": "'"$IMG_PRODUCT"'",
    "scalingRatio": 2
  }
}'
}

test_poster-enhance() {
    sep; echo "poster-enhance  AI 超清"
    call_sync "poster-enhance" '{
  "model": "poster-enhance",
  "metadata": {
    "imgUrls": "'"$IMG_PRODUCT"'",
    "enhanceStrength": "medium"
  }
}'
}

test_poster-extension() {
    sep; echo "poster-extension  智能延展"
    call_sync "poster-extension" '{
  "model": "poster-extension",
  "metadata": {
    "imgUrlList": ["'"$IMG_BANNER"'"],
    "ratio": "16:9"
  }
}'
}

test_poster-translate() {
    sep; echo "poster-translate  图片翻译"
    call_sync "poster-translate" '{
  "model": "poster-translate",
  "metadata": {
    "imageUrl": "'"$IMG_BANNER"'",
    "to": 1
  }
}'
}

test_poster-partial-redraw() {
    sep; echo "poster-partial-redraw  局部重绘"
    call_sync "poster-partial-redraw" '{
  "model": "poster-partial-redraw",
  "metadata": {
    "sourceUrl": "'"$IMG_BANNER"'",
    "textPrompt": "将背景换成蓝天白云"
  }
}'
}

test_poster-scene-replace() {
    sep; echo "poster-scene-replace  场景替换"
    call_sync "poster-scene-replace" '{
  "model": "poster-scene-replace",
  "metadata": {
    "sourceUrl": "'"$IMG_PRODUCT"'",
    "replaceImageUrl": "'"$IMG_BANNER"'",
    "textPrompt": "将商品放在咖啡桌上"
  }
}'
}

test_poster-product-replace() {
    sep; echo "poster-product-replace  商品替换"
    call_sync "poster-product-replace" '{
  "model": "poster-product-replace",
  "metadata": {
    "sourceUrl": "'"$IMG_BANNER"'",
    "replaceImageUrl": "'"$IMG_PRODUCT"'",
    "textPrompt": "将图中商品替换为新款手表"
  }
}'
}

test_poster-color-change() {
    sep; echo "poster-color-change  商品换色"
    call_sync "poster-color-change" '{
  "model": "poster-color-change",
  "metadata": {
    "sourceUrl": "'"$IMG_PRODUCT"'",
    "textPrompt": "将商品颜色换成玫瑰红",
    "modelType": 0
  }
}'
}

test_poster-assisted() {
    sep; echo "poster-assisted  AI 帮写"
    call_sync "poster-assisted" '{
  "model": "poster-assisted",
  "metadata": {
    "query": "为一款保湿面霜生成产品描述文案，突出天然成分和长效保湿效果",
    "generateType": "image"
  }
}'
}

test_poster-generate-sync() {
    sep; echo "poster-generate-sync  同步海报生成"
    call_sync "poster-generate-sync" '{
  "model": "poster-generate-sync",
  "metadata": {
    "query": "一款高端护肤品海报，背景简洁白色，突出保湿效果",
    "generateType": 100,
    "posterType": 6,
    "platformType": "天猫",
    "languageType": "中文",
    "detailPictureNumber": 2,
    "modelEdition": 3,
    "needText": true,
    "aspectRatio": "1:1"
  }
}'
}

test_poster-generate() {
    sep; echo "poster-generate  异步海报生成（提交）"
    info "POST /v1/images/tasks  model=poster-generate"
    task_resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/tasks" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
  "model": "poster-generate",
  "metadata": {
    "query": "一款高端护肤品海报，背景简洁白色，突出保湿效果",
    "generateType": 100,
    "posterType": 6,
    "platformType": "天猫",
    "languageType": "中文",
    "detailPictureNumber": 2,
    "modelEdition": 3,
    "needText": true,
    "aspectRatio": "1:1"
  }
}')
    task_http=$(echo "$task_resp" | tail -1)
    task_body=$(echo "$task_resp" | head -n -1)
    echo "HTTP $task_http"
    echo "$task_body" | python3 -m json.tool 2>/dev/null || echo "$task_body"

    TASK_ID=$(echo "$task_body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || true)
    if [ -n "$TASK_ID" ] && [ "$task_http" = "200" ]; then
        ok "poster-generate 提交成功，task_id=$TASK_ID"
        sep; echo "轮询任务结果（最多等待 120s）"
        for i in $(seq 1 12); do
            sleep 10
            info "第 $i 次轮询... GET /v1/images/tasks/$TASK_ID"
            poll_resp=$(curl -s -w "\n%{http_code}" "$GATEWAY/v1/images/tasks/$TASK_ID" \
                -H "Authorization: Bearer $API_KEY")
            poll_http=$(echo "$poll_resp" | tail -1)
            poll_body=$(echo "$poll_resp" | head -n -1)
            echo "HTTP $poll_http"
            echo "$poll_body" | python3 -m json.tool 2>/dev/null || echo "$poll_body"
            status=$(echo "$poll_body" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status',''))" 2>/dev/null || true)
            if [ "$status" = "succeeded" ]; then
                ok "任务完成  status=succeeded"; break
            elif [ "$status" = "failed" ]; then
                fail "任务失败  status=failed"; break
            else
                info "状态: $status，继续等待..."
            fi
        done
    else
        fail "poster-generate 提交失败  HTTP=$task_http"
    fi
}

test_poster-free-creation() {
    sep; echo "poster-free-creation  自由创作（提交）"
    info "POST /v1/images/tasks  model=poster-free-creation"
    fc_resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/tasks" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
  "model": "poster-free-creation",
  "metadata": {
    "query": "科技感十足的AI产品宣传海报，深蓝色调",
    "detailPictureNumber": 2,
    "aspectRatio": "16:9"
  }
}')
    fc_http=$(echo "$fc_resp" | tail -1)
    fc_body=$(echo "$fc_resp" | head -n -1)
    echo "HTTP $fc_http"
    echo "$fc_body" | python3 -m json.tool 2>/dev/null || echo "$fc_body"
    if [ "$fc_http" = "200" ]; then ok "poster-free-creation 提交成功"; else fail "poster-free-creation  HTTP=$fc_http"; fi
}

# ──────────────────────────────────────────────
# 入口：按参数执行或全部执行
# ──────────────────────────────────────────────

ALL_MODELS=(
    poster-matting
    poster-enlarge
    poster-enhance
    poster-extension
    poster-translate
    poster-partial-redraw
    poster-scene-replace
    poster-product-replace
    poster-color-change
    poster-assisted
    poster-generate-sync
    poster-generate
    poster-free-creation
)

if [ -n "$1" ]; then
    fn="test_$1"
    if declare -f "$fn" > /dev/null; then
        "$fn"
    else
        echo -e "${RED}未知模型: $1${RESET}"
        echo "可用模型："
        for m in "${ALL_MODELS[@]}"; do echo "  $m"; done
        exit 1
    fi
else
    for m in "${ALL_MODELS[@]}"; do
        "test_$m"
    done
fi

sep
echo "测试完成"
