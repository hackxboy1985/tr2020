#!/usr/bin/env bash
# 调用 gpt-image-2-all 模型 (Td 渠道)
# 用法: ./td_test.sh <API_BASE> <API_KEY>
# 示例: ./td_test.sh https://api.luluai.cc sk-xxxxx

API_BASE="${1:-https://api.luluai.cc}"
API_KEY="${2:-}"

if [ -z "$API_KEY" ]; then
  echo "用法: $0 <API_BASE> <API_KEY>" >&2
  exit 1
fi

curl -sS -X POST "${API_BASE}/v1/images/generations" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "images": ["https://static.horse-world.mints-id.com/rh/20260604204601/1780577161831_3248.png"],
    "size": "16:9",
    "model": "gpt-image-2-all",
    "prompt": "风格真人电影,根据图中人物形象,生成全身三视图以及一张特写(最左边占满三分之一的位置是整个头部(面部)特写，右边三分之二放正视图、侧视图、后视图，纯白背景)，保持人物发丝超清晰，发丝光泽，8K高清，细节拉满。 负面词：头发模糊、发丝结块。，冲锋衣造型",
    "resolution": "2k",
    "quality": "medium"
  }'