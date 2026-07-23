#!/usr/bin/env bash
# 海报渠道接口测试脚本
#
# 用法：
#   ./test_poster.sh                          # 执行全部接口（使用默认参数）
#   ./test_poster.sh <模型名> [参数...]        # 单独执行某个接口
#   ./test_poster.sh <模型名> --help          # 查看该接口参数说明
#
# 示例：
#   ./test_poster.sh poster-matting --img https://example.com/product.jpg
#   ./test_poster.sh poster-enhance --img https://example.com/photo.jpg
#   ./test_poster.sh poster-translate --img https://example.com/banner.jpg --to 1
#   ./test_poster.sh poster-partial-redraw --source https://example.com/a.jpg --prompt "将背景换成草原"
#   ./test_poster.sh poster-scene-replace --source https://example.com/a.jpg --replace https://example.com/b.jpg --prompt "将背景换成海滩"
#   ./test_poster.sh poster-product-replace --source https://example.com/scene.jpg --replace https://example.com/product.jpg --prompt "替换场景中的商品"
#   ./test_poster.sh poster-color-change --source https://example.com/bag.jpg --prompt "换成玫瑰红"
#   ./test_poster.sh poster-extension --img https://example.com/banner.jpg --ratio 16:9
#   ./test_poster.sh poster-assisted --query "为保湿面霜写产品文案"
#   ./test_poster.sh poster-generate-sync --query "高端护肤品海报"
#   ./test_poster.sh poster-generate --query "运动鞋海报，背景户外"
#   ./test_poster.sh poster-free-creation --query "科技感蓝色电子产品海报"

# ──────────────────────────────────────────────
# 配置区（必填）
# ──────────────────────────────────────────────
GATEWAY="http://book2:3002"          # new-api 网关地址
API_KEY="sk-BTx3kf9qRT0TCjaWHg3pL9H4DCbwFDcxbZjW1TUMU9lQJT"                    # 你的 new-api Token

# 默认测试图片（公网可访问）
DEFAULT_IMG_PRODUCT="https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=600"
DEFAULT_IMG_BANNER="https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=800"

# ──────────────────────────────────────────────
# 工具函数
# ──────────────────────────────────────────────
GREEN="\033[32m"; RED="\033[31m"; YELLOW="\033[33m"; RESET="\033[0m"

ok()   { echo -e "${GREEN}[PASS]${RESET} $*"; }
fail() { echo -e "${RED}[FAIL]${RESET} $*"; }
info() { echo -e "${YELLOW}[INFO]${RESET} $*"; }
sep()  { echo -e "\n────────────────────────────────────────"; }

# 解析命名参数 --key value
parse_args() {
    ARG_IMG=""
    ARG_SOURCE=""
    ARG_REPLACE=""
    ARG_PROMPT=""
    ARG_QUERY=""
    ARG_TO=""
    ARG_RATIO=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --img)     ARG_IMG="$2";     shift 2 ;;
            --source)  ARG_SOURCE="$2";  shift 2 ;;
            --replace) ARG_REPLACE="$2"; shift 2 ;;
            --prompt)  ARG_PROMPT="$2";  shift 2 ;;
            --query)   ARG_QUERY="$2";   shift 2 ;;
            --to)      ARG_TO="$2";      shift 2 ;;
            --ratio)   ARG_RATIO="$2";   shift 2 ;;
            *) shift ;;
        esac
    done
}

call_sync() {
    local model="$1"
    local body="$2"
    info "POST /v1/images/generations  model=$model"
    resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/generations" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$body")
    code=$(echo "$resp" | tail -1)
    body_resp=$(echo "$resp" | sed '$d')
    echo "HTTP $code"
    echo "$body_resp" | python3 -m json.tool 2>/dev/null || echo "$body_resp"
    has_error=$(echo "$body_resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if 'error' in d else 'no')" 2>/dev/null || echo "no")
    if [ "$code" = "200" ] && [ "$has_error" = "no" ]; then ok "$model"; else fail "$model  HTTP=$code"; fi
}

# ──────────────────────────────────────────────
# 各接口函数
# ──────────────────────────────────────────────

test_poster_matting() {
    if [ "$1" = "--help" ]; then
        echo "poster-matting  AI 抠图"
        echo "参数："
        echo "  --img <url>    图片URL，多张逗号分隔，最多6张（必填）"
        return
    fi
    parse_args "$@"
    local img="${ARG_IMG:-$DEFAULT_IMG_PRODUCT}"
    sep; echo "poster-matting  AI 抠图"
    info "img=$img"
    call_sync "poster-matting" '{
  "model": "poster-matting",
  "metadata": { "imgUrls": "'"$img"'" }
}'
}

test_poster_enlarge() {
    if [ "$1" = "--help" ]; then
        echo "poster-enlarge  无损放大"
        echo "参数："
        echo "  --img <url>      图片URL，多张逗号分隔，最多6张（必填）"
        echo "  --ratio <1|2|3>  放大倍率：1=轻度，2=标准，3=强力（默认2）"
        return
    fi
    parse_args "$@"
    local img="${ARG_IMG:-$DEFAULT_IMG_PRODUCT}"
    local ratio="${ARG_RATIO:-2}"
    sep; echo "poster-enlarge  无损放大"
    info "img=$img  ratio=$ratio"
    call_sync "poster-enlarge" '{
  "model": "poster-enlarge",
  "metadata": { "imgUrls": "'"$img"'", "scalingRatio": '"$ratio"' }
}'
}

test_poster_enhance() {
    if [ "$1" = "--help" ]; then
        echo "poster-enhance  AI 超清"
        echo "参数："
        echo "  --img <url>                      图片URL，多张逗号分隔，最多6张（必填）"
        echo "  --ratio <light|standard|strong>  增强强度（默认 standard）"
        return
    fi
    parse_args "$@"
    local img="${ARG_IMG:-$DEFAULT_IMG_PRODUCT}"
    local strength="${ARG_RATIO:-standard}"
    sep; echo "poster-enhance  AI 超清"
    info "img=$img  strength=$strength"
    call_sync "poster-enhance" '{
  "model": "poster-enhance",
  "metadata": { "imgUrls": "'"$img"'", "enhanceStrength": "'"$strength"'" }
}'
}

test_poster_extension() {
    if [ "$1" = "--help" ]; then
        echo "poster-extension  智能延展"
        echo "参数："
        echo "  --img <url>      图片URL（必填）"
        echo "  --ratio <ratio>  目标比例，如 1:1、16:9、9:16（默认 16:9）"
        return
    fi
    parse_args "$@"
    local img="${ARG_IMG:-$DEFAULT_IMG_BANNER}"
    local ratio="${ARG_RATIO:-16:9}"
    sep; echo "poster-extension  智能延展"
    info "img=$img  ratio=$ratio"
    call_sync "poster-extension" '{
  "model": "poster-extension",
  "metadata": { "imgUrlList": ["'"$img"'"], "ratio": "'"$ratio"'" }
}'
}

test_poster_translate() {
    if [ "$1" = "--help" ]; then
        echo "poster-translate  图片翻译"
        echo "参数："
        echo "  --img <url>  图片URL（必填）"
        echo "  --to <int>   目标语言编号（默认1=英文）"
        echo "               0=中文 1=英文 2=俄语 3=西班牙 4=法语 5=德语"
        echo "               6=意大利 7=荷兰 8=葡萄牙 9=越南 10=土耳其"
        echo "               11=马来 12=泰语 13=波兰 14=印尼 15=日语 16=韩语 17=繁体中文"
        return
    fi
    parse_args "$@"
    local img="${ARG_IMG:-$DEFAULT_IMG_BANNER}"
    local to="${ARG_TO:-1}"
    sep; echo "poster-translate  图片翻译"
    info "img=$img  to=$to"
    call_sync "poster-translate" '{
  "model": "poster-translate",
  "metadata": { "imageUrl": "'"$img"'", "to": '"$to"' }
}'
}

test_poster_partial_redraw() {
    if [ "$1" = "--help" ]; then
        echo "poster-partial-redraw  局部重绘"
        echo "参数："
        echo "  --source <url>   原图URL（必填）"
        echo "  --prompt <text>  重绘描述（必填）"
        echo "  --replace <url>  参考替换图（可选）"
        return
    fi
    parse_args "$@"
    local source="${ARG_SOURCE:-$DEFAULT_IMG_BANNER}"
    local prompt="${ARG_PROMPT:-将背景换成蓝天白云}"
    sep; echo "poster-partial-redraw  局部重绘"
    info "source=$source  prompt=$prompt"
    local replace_field=""
    if [ -n "$ARG_REPLACE" ]; then
        replace_field=', "replaceImageUrl": "'"$ARG_REPLACE"'"'
    fi
    call_sync "poster-partial-redraw" '{
  "model": "poster-partial-redraw",
  "metadata": { "sourceUrl": "'"$source"'", "textPrompt": "'"$prompt"'"'"$replace_field"' }
}'
}

test_poster_scene_replace() {
    if [ "$1" = "--help" ]; then
        echo "poster-scene-replace  场景替换"
        echo "参数："
        echo "  --source <url>   原图URL（必填）"
        echo "  --replace <url>  场景参考图URL（必填）"
        echo "  --prompt <text>  场景描述（必填）"
        return
    fi
    parse_args "$@"
    local source="${ARG_SOURCE:-$DEFAULT_IMG_PRODUCT}"
    local replace="${ARG_REPLACE:-$DEFAULT_IMG_BANNER}"
    local prompt="${ARG_PROMPT:-将商品放在咖啡桌上}"
    sep; echo "poster-scene-replace  场景替换"
    info "source=$source  replace=$replace  prompt=$prompt"
    call_sync "poster-scene-replace" '{
  "model": "poster-scene-replace",
  "metadata": { "sourceUrl": "'"$source"'", "replaceImageUrl": "'"$replace"'", "textPrompt": "'"$prompt"'" }
}'
}

test_poster_product_replace() {
    if [ "$1" = "--help" ]; then
        echo "poster-product-replace  商品替换"
        echo "参数："
        echo "  --source <url>   含旧商品的场景图URL（必填）"
        echo "  --replace <url>  目标商品图URL（必填）"
        echo "  --prompt <text>  替换描述（必填）"
        return
    fi
    parse_args "$@"
    local source="${ARG_SOURCE:-$DEFAULT_IMG_BANNER}"
    local replace="${ARG_REPLACE:-$DEFAULT_IMG_PRODUCT}"
    local prompt="${ARG_PROMPT:-将图中商品替换为新款手表}"
    sep; echo "poster-product-replace  商品替换"
    info "source=$source  replace=$replace  prompt=$prompt"
    call_sync "poster-product-replace" '{
  "model": "poster-product-replace",
  "metadata": { "sourceUrl": "'"$source"'", "replaceImageUrl": "'"$replace"'", "textPrompt": "'"$prompt"'" }
}'
}

test_poster_color_change() {
    if [ "$1" = "--help" ]; then
        echo "poster-color-change  商品换色"
        echo "参数："
        echo "  --source <url>   原图URL（必填）"
        echo "  --prompt <text>  换色描述，如"换成玫瑰红"（必填）"
        return
    fi
    parse_args "$@"
    local source="${ARG_SOURCE:-$DEFAULT_IMG_PRODUCT}"
    local prompt="${ARG_PROMPT:-将商品颜色换成玫瑰红}"
    sep; echo "poster-color-change  商品换色"
    info "source=$source  prompt=$prompt"
    call_sync "poster-color-change" '{
  "model": "poster-color-change",
  "metadata": { "sourceUrl": "'"$source"'", "textPrompt": "'"$prompt"'", "modelType": 0 }
}'
}

test_poster_assisted() {
    if [ "$1" = "--help" ]; then
        echo "poster-assisted  AI 帮写（返回文案，在 revised_prompt 字段）"
        echo "参数："
        echo "  --query <text>  需求描述（必填）"
        echo "  --img <url>     参考图片URL（可选）"
        return
    fi
    parse_args "$@"
    local query="${ARG_QUERY:-为一款保湿面霜生成产品描述文案，突出天然成分和长效保湿效果}"
    sep; echo "poster-assisted  AI 帮写"
    info "query=$query"
    local file_field=""
    if [ -n "$ARG_IMG" ]; then
        file_field=', "fileUrlList": ["'"$ARG_IMG"'"]'
    fi
    call_sync "poster-assisted" '{
  "model": "poster-assisted",
  "metadata": { "query": "'"$query"'", "generateType": "image"'"$file_field"' }
}'
}

test_poster_generate_sync() {
    if [ "$1" = "--help" ]; then
        echo "poster-generate-sync  同步海报生成（直接返回图片URL）"
        echo "参数："
        echo "  --query <text>  需求描述（必填）"
        echo "  --img <url>     参考图片URL（可选）"
        return
    fi
    parse_args "$@"
    local query="${ARG_QUERY:-一款高端护肤品海报，背景简洁白色，突出保湿效果}"
    sep; echo "poster-generate-sync  同步海报生成"
    info "query=$query"
    local file_field=""
    if [ -n "$ARG_IMG" ]; then
        file_field=', "fileUrlList": ["'"$ARG_IMG"'"]'
    fi
    call_sync "poster-generate-sync" '{
  "model": "poster-generate-sync",
  "metadata": {
    "query": "'"$query"'",
    "generateType": 100,
    "posterType": 6,
    "platformType": "天猫",
    "languageType": "中文",
    "detailPictureNumber": 2,
    "modelEdition": 3,
    "needText": true,
    "aspectRatio": "1:1"'"$file_field"'
  }
}'
}

test_poster_generate() {
    if [ "$1" = "--help" ]; then
        echo "poster-generate  异步海报生成（提交后自动轮询，最多120s）"
        echo "参数："
        echo "  --query <text>  需求描述（必填）"
        echo "  --img <url>     参考图片URL（可选）"
        return
    fi
    parse_args "$@"
    local query="${ARG_QUERY:-一款高端护肤品海报，背景简洁白色，突出保湿效果}"
    sep; echo "poster-generate  异步海报生成（提交）"
    info "POST /v1/images/tasks  query=$query"
    local file_field=""
    if [ -n "$ARG_IMG" ]; then
        file_field=', "fileUrlList": ["'"$ARG_IMG"'"]'
    fi
    task_resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/tasks" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
  "model": "poster-generate",
  "metadata": {
    "query": "'"$query"'",
    "generateType": 100,
    "posterType": 6,
    "platformType": "天猫",
    "languageType": "中文",
    "detailPictureNumber": 2,
    "modelEdition": 3,
    "needText": true,
    "aspectRatio": "1:1"'"$file_field"'
  }
}')
    task_http=$(echo "$task_resp" | tail -1)
    task_body=$(echo "$task_resp" | sed '$d')
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
            poll_body=$(echo "$poll_resp" | sed '$d')
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

test_poster_free_creation() {
    if [ "$1" = "--help" ]; then
        echo "poster-free-creation  自由创作（异步，仅测试提交）"
        echo "参数："
        echo "  --query <text>  需求描述（必填）"
        echo "  --img <url>     参考图片URL（可选）"
        return
    fi
    parse_args "$@"
    local query="${ARG_QUERY:-科技感十足的AI产品宣传海报，深蓝色调}"
    sep; echo "poster-free-creation  自由创作（提交）"
    info "POST /v1/images/tasks  query=$query"
    local img_field=""
    if [ -n "$ARG_IMG" ]; then
        img_field=', "apiImgUrlList": ["'"$ARG_IMG"'"]'
    fi
    fc_resp=$(curl -s -w "\n%{http_code}" -X POST "$GATEWAY/v1/images/tasks" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
  "model": "poster-free-creation",
  "metadata": {
    "query": "'"$query"'",
    "detailPictureNumber": 2,
    "aspectRatio": "16:9"'"$img_field"'
  }
}')
    fc_http=$(echo "$fc_resp" | tail -1)
    fc_body=$(echo "$fc_resp" | sed '$d')
    echo "HTTP $fc_http"
    echo "$fc_body" | python3 -m json.tool 2>/dev/null || echo "$fc_body"
    if [ "$fc_http" = "200" ]; then ok "poster-free-creation 提交成功"; else fail "poster-free-creation  HTTP=$fc_http"; fi
}

# ──────────────────────────────────────────────
# 入口
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

if [ -n "$1" ] && [ "$1" != "all" ]; then
    fn="test_${1//-/_}"
    if declare -f "$fn" > /dev/null; then
        shift
        "$fn" "$@"
    else
        echo -e "${RED}未知模型: $1${RESET}"
        echo "可用模型（用 --help 查看各接口参数）："
        for m in "${ALL_MODELS[@]}"; do echo "  sh test_poster.sh $m --help"; done
        exit 1
    fi
elif [ "$1" = "all" ]; then
    for m in "${ALL_MODELS[@]}"; do
        "test_${m//-/_}"
    done
    sep
    echo "测试完成"
else
    echo "用法："
    echo "  sh test_poster.sh all                             # 执行全部接口"
    echo "  sh test_poster.sh <模型名> [参数...]              # 执行单个接口"
    echo "  sh test_poster.sh <模型名> --help                 # 查看接口参数"
    echo ""
    echo "可执行示例："
    echo "  # AI 抠图"
    echo "  sh test_poster.sh poster-matting --img https://example.com/product.jpg"
    echo "  # 无损放大"
    echo "  sh test_poster.sh poster-enlarge --img https://example.com/img.jpg --ratio 2"
    echo "  # AI 超清"
    echo "  sh test_poster.sh poster-enhance --img https://example.com/img.jpg"
    echo "  # 智能延展"
    echo "  sh test_poster.sh poster-extension --img https://example.com/banner.jpg --ratio 16:9"
    echo "  # 图片翻译（to: 0=中文 1=英文 2=俄语 ...）"
    echo "  sh test_poster.sh poster-translate --img https://example.com/banner.jpg --to 1"
    echo "  # 局部重绘"
    echo "  sh test_poster.sh poster-partial-redraw --source https://example.com/a.jpg --prompt \"将背景换成草原\""
    echo "  # 场景替换"
    echo "  sh test_poster.sh poster-scene-replace --source https://example.com/a.jpg --replace https://example.com/b.jpg --prompt \"换成海滩场景\""
    echo "  # 商品替换"
    echo "  sh test_poster.sh poster-product-replace --source https://example.com/scene.jpg --replace https://example.com/product.jpg --prompt \"替换商品\""
    echo "  # 商品换色"
    echo "  sh test_poster.sh poster-color-change --source https://example.com/bag.jpg --prompt \"换成玫瑰红\""
    echo "  # AI 帮写（返回文案，在 revised_prompt 字段）"
    echo "  sh test_poster.sh poster-assisted --query \"为保湿面霜写产品文案\""
    echo "  # 同步海报生成（直接返回图片URL）"
    echo "  sh test_poster.sh poster-generate-sync --query \"高端护肤品海报\""
    echo "  # 异步海报生成（提交后自动轮询）"
    echo "  sh test_poster.sh poster-generate --query \"运动鞋海报，背景户外\""
    echo "  # 自由创作（异步，仅测试提交）"
    echo "  sh test_poster.sh poster-free-creation --query \"科技感蓝色电子产品海报\""
fi
