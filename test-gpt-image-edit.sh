#!/bin/bash


BASE_URL="${1:-https://www.luluai.cc}"
API_KEY="${2:-sk-UJkSn1Bxs2Jynb8pBj3iIpbfiXjVVnlsQQeMq8gh33EfBV3}"

MODEL="gpt-image-2"
PROMPT="生成分镜图，6个镜头，图中人物在场景中行走，仿佛穿越时空，突出惊讶的表情"
IMAGE_FILE1="/Users/mac/Downloads/girl.jpg"
IMAGE_FILE2="/Users/mac/Downloads/1781612162129_8861.jpg"
N="1"
SIZE="1536x1024"
QUALITY="medium"
BACKGROUND="auto"
MODERATION="low"

OUTPUT_DIR="/tmp/gpt-image-edit-test"
mkdir -p "$OUTPUT_DIR"
RESP_FILE="$OUTPUT_DIR/response.json"
REQ_FILE="$OUTPUT_DIR/request.json"

echo "=== GPT Image Edit Test ==="
echo "URL: $BASE_URL/v1/images/edits"
echo "Model: $MODEL"
echo "Image1: $IMAGE_FILE1"
echo "Image2: $IMAGE_FILE2"
echo ""

cat > "$REQ_FILE" <<EOF
{
  "model": "$MODEL",
  "prompt": "$PROMPT",
  "image_file1": "$IMAGE_FILE1",
  "image_file2": "$IMAGE_FILE2",
  "n": "$N",
  "size": "$SIZE",
  "quality": "$QUALITY",
  "background": "$BACKGROUND",
  "moderation": "$MODERATION"
}
EOF

echo "Request saved: $REQ_FILE"

curl -s -X POST "$BASE_URL/v1/images/edits" \
  -H "Authorization: Bearer $API_KEY" \
  -F "model=$MODEL" \
  -F "prompt=$PROMPT" \
  -F "image[]=@$IMAGE_FILE1;type=image/jpeg" \
  -F "image[]=@$IMAGE_FILE2;type=image/jpeg" \
  -F "n=$N" \
  -F "size=$SIZE" \
  -F "quality=$QUALITY" \
  -F "background=$BACKGROUND" \
  -F "moderation=$MODERATION" \
  -o "$RESP_FILE"

echo "Response saved: $RESP_FILE"
echo ""

python3 - <<PYEOF
import json, base64, os, sys, urllib.request

resp_file = "$RESP_FILE"
output_dir = "$OUTPUT_DIR"

with open(resp_file) as f:
    data = json.load(f)

if "error" in data:
    print(f"ERROR: {data['error']}")
    sys.exit(1)

images = data.get("data", [])
if not images:
    print("ERROR: no images in response")
    print(json.dumps(data, indent=2, ensure_ascii=False))
    sys.exit(1)

for i, img in enumerate(images):
    img_path = os.path.join(output_dir, f"result_{i}.png")
    if img.get("b64_json"):
        with open(img_path, "wb") as f:
            f.write(base64.b64decode(img["b64_json"]))
        print(f"Image saved: {img_path}")
    elif img.get("url"):
        urllib.request.urlretrieve(img["url"], img_path)
        print(f"Image saved: {img_path} (from url)")
    if img.get("revised_prompt"):
        print(f"Revised prompt: {img['revised_prompt']}")

usage = data.get("usage", {})
if usage:
    print(f"Tokens: {usage}")
PYEOF

for img in "$OUTPUT_DIR"/result_*.png; do
  [ -f "$img" ] && open "$img"
done
