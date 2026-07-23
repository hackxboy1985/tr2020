#!/usr/bin/env bash
# 海报渠道接口测试脚本
# 用法：
#   chmod +x test_poster.sh
#   ./test_poster.sh
#
# 修改下方变量后执行

set -e

# ──────────────────────────────────────────────
# 配置区（必填）
# ──────────────────────────────────────────────
GATEWAY="http://localhost:3000"     # new-api 网关地址
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
# 1. AI 抠图（无文本，返回图片 URL）
# ──────────────────────────────────────────────
sep; echo "1. poster-matting  AI 抠图"
call_sync "poster-matting" '{
  "model": "poster-matting",
  "metadata": {
    "imgUrls": "'"$IMG_PRODUCT"'"
  }
}'

# ──────────────────────────────────────────────
# 2. 无损放大
# ──────────────────────────────────────────────
sep; echo "2. poster-enlarge  无损放大"
call_sync "poster-enlarge" '{
  "model": "poster-enlarge",
  "metadata": {
    "imgUrls": "'"$IMG_PRODUCT"'",
    "scalingRatio": 2
  }
}'

# ──────────────────────────────────────────────
# 3. AI 超清
# ──────────────────────────────────────────────
sep; echo "3. poster-enhance  AI 超清"
call_sync "poster-enhance" '{
  "model": "poster-enhance",
  "metadata": {
    "imgUrls": "'"$IMG_PRODUCT"'",
    "enhanceStrength": "medium"
  }
}'

# ──────────────────────────────────────────────
# 4. 智能延展（返回字符串数组）
# ──────────────────────────────────────────────
sep; echo "4. poster-extension  智能延展"
call_sync "poster-extension" '{
  "model": "poster-extension",
  "metadata": {
    "imgUrlList": ["'"$IMG_BANNER"'"],
    "ratio": "16:9"
  }
}'

# ──────────────────────────────────────────────
# 5. 图片翻译（to=1 英文）
# ──────────────────────────────────────────────
sep; echo "5. poster-translate  图片翻译"
call_sync "poster-translate" '{
  "model": "poster-translate",
  "metadata": {
    "imageUrl": "'"$IMG_BANNER"'",
    "to": 1
  }
}'

# ──────────────────────────────────────────────
# 6. 局部重绘（有文本）
# ──────────────────────────────────────────────
sep; echo "6. poster-partial-redraw  局部重绘"
call_sync "poster-partial-redraw" '{
  "model": "poster-partial-redraw",
  "metadata": {
    "sourceUrl": "'"$IMG_BANNER"'",
    "textPrompt": "将背景换成蓝天白云"
  }
}'

# ──────────────────────────────────────────────
# 7. 场景替换
# ──────────────────────────────────────────────
sep; echo "7. poster-scene-replace  场景替换"
call_sync "poster-scene-replace" '{
  "model": "poster-scene-replace",
  "metadata": {
    "sourceUrl": "'"$IMG_PRODUCT"'",
    "replaceImageUrl": "'"$IMG_BANNER"'",
    "textPrompt": "将商品放在咖啡桌上"
  }
}'

# ──────────────────────────────────────────────
# 8. 商品替换
# ──────────────────────────────────────────────
sep; echo "8. poster-product-replace  商品替换"
call_sync "poster-product-replace" '{
  "model": "poster-product-replace",
  "metadata": {
    "sourceUrl": "'"$IMG_BANNER"'",
    "replaceImageUrl": "'"$IMG_PRODUCT"'",
    "textPrompt": "将图中商品替换为新款手表"
  }
}'

# ──────────────────────────────────────────────
# 9. 商品换色
# ──────────────────────────────────────────────
sep; echo "9. poster-color-change  商品换色"
call_sync "poster-color-change" '{
  "model": "poster-color-change",
  "metadata": {
    "sourceUrl": "'"$IMG_PRODUCT"'",
    "textPrompt": "将商品颜色换成玫瑰红",
    "modelType": 0
  }
}'

# ──────────────────────────────────────────────
# 10. AI 帮写（返回文案，url 为空，内容在 revised_prompt）
# ──────────────────────────────────────────────
sep; echo "10. poster-assisted  AI 帮写"
call_sync "poster-assisted" '{
  "model": "poster-assisted",
  "metadata": {
    "query": "为一款保湿面霜生成产品描述文案，突出天然成分和长效保湿效果",
    "generateType": "image"
  }
}'

# ──────────────────────────────────────────────
# 11. 同步海报生成（直接返回图片 URL，无需轮询）
# ──────────────────────────────────────────────
sep; echo "11. poster-generate-sync  同步海报生成"
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

# ──────────────────────────────────────────────
# 12. 异步海报生成（提交 + 轮询）
# ──────────────────────────────────────────────
sep; echo "12. poster-generate  异步海报生成（提交）"
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
    sep; echo "12. 轮询任务结果（最多等待 120s）"
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

# ──────────────────────────────────────────────
# 13. 自由创作（异步）
# ──────────────────────────────────────────────
sep; echo "13. poster-free-creation  自由创作（提交）"
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

sep
echo "测试完成"
