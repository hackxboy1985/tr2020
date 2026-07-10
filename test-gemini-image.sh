#!/bin/bash

BASE_URL="${1:-https://www.luluai.cc}"
API_KEY="${2:-sk-UJkSn1Bxs2Jynb8pBj3iIpbfiXjVVnlsQQeMq8gh33EfBV3}"
MODEL="${3:-gemini-image-banana2}"
IMAGE_FILE="/Users/mac/Downloads/girl.jpg"
IMAGE_DATA=$(base64 -i "$IMAGE_FILE" | tr -d '\n')
ASPECT_RATIO="16:9"
IMAGE_SIZE="2K"
PROMPT="真人电影，女生在空中飞翔"

OUTPUT_DIR="/tmp/gemini-image-test"
mkdir -p "$OUTPUT_DIR"
RESP_FILE="$OUTPUT_DIR/response.json"
REQ_FILE="$OUTPUT_DIR/request.json"

echo "=== Gemini Image Generation Test ==="
echo "URL: $BASE_URL"
echo "Model: $MODEL"
echo "Image data length: ${#IMAGE_DATA}"
echo ""

JSON=$(cat <<EOF
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "$PROMPT"},
        {
          "inlineData": {
            "mimeType": "image/jpeg",
            "data": "$IMAGE_DATA"
          }
        }
      ]
    }
  ],
  "generationConfig": {
    "response_modalities": ["IMAGE", "TEXT"],
    "imageConfig": {
      "aspectRatio": "$ASPECT_RATIO",
      "imageSize": "$IMAGE_SIZE"
    }
  }
}
EOF
)

# 保存请求参数（不含图片base64，避免文件过大）

echo "Request saved: $REQ_FILE"

cat >  "$REQ_FILE" << EOF
"$BASE_URL/v1beta/models/$MODEL:generateContent" 
EOF

cat >> "$REQ_FILE" <<EOF
$JSON
EOF


curl -s -X POST "$BASE_URL/v1beta/models/$MODEL:generateContent" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d "$JSON" -o "$RESP_FILE"

echo "Response saved: $RESP_FILE"
echo ""

# 提取图片并保存
python3 - <<PYEOF
import json, base64, os, sys

resp_file = "$RESP_FILE"
output_dir = "$OUTPUT_DIR"

with open(resp_file) as f:
    data = json.load(f)

candidates = data.get("candidates", [])
if not candidates:
    print("ERROR: no candidates in response")
    print(json.dumps(data, indent=2, ensure_ascii=False))
    sys.exit(1)

img_index = 0
for candidate in candidates:
    for part in candidate.get("content", {}).get("parts", []):
        if "text" in part and part["text"].strip():
            print(f"Text: {part['text']}")
        if "inlineData" in part:
            inline = part["inlineData"]
            mime = inline.get("mimeType", "image/png")
            ext = mime.split("/")[-1]
            img_path = os.path.join(output_dir, f"result_{img_index}.{ext}")
            with open(img_path, "wb") as f:
                f.write(base64.b64decode(inline["data"]))
            print(f"Image saved: {img_path}")
            img_index += 1

usage = data.get("usageMetadata", {})
if usage:
    print(f"Tokens: prompt={usage.get('promptTokenCount',0)} candidates={usage.get('candidatesTokenCount',0)} total={usage.get('totalTokenCount',0)}")
PYEOF

# macOS 打开图片
for img in "$OUTPUT_DIR"/result_*.png "$OUTPUT_DIR"/result_*.jpg "$OUTPUT_DIR"/result_*.jpeg; do
  [ -f "$img" ] && open "$img"
done
