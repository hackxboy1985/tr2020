#!/bin/bash

BASE_URL="${1:-https://www.luluai.cc}"
API_KEY="${2:-sk-UJkSn1Bxs2Jynb8pBj3iIpbfiXjVVnlsQQeMq8gh33EfBV3}"

MODEL="gpt-image-2:stable"
PROMPT="A futuristic city skyline at sunset with neon lights"
SIZE="1024x1536"
QUALITY="medium"
N=1
FORMAT="jpeg"

OUTPUT_DIR="/tmp/channel-image-test"
mkdir -p "$OUTPUT_DIR"
RESP_FILE="$OUTPUT_DIR/response.json"
REQ_FILE="$OUTPUT_DIR/request.json"

echo "=== Channel Image Test ==="
echo "URL: $BASE_URL/v1/images/generations"
echo "Model: $MODEL"
echo "Prompt: $PROMPT"
echo ""

cat > "$REQ_FILE" <<EOF
{
  "model": "$MODEL",
  "prompt": "$PROMPT",
  "size": "$SIZE",
  "quality": "$QUALITY",
  "n": $N,
  "format": "$FORMAT"
}
EOF


cat >  "$REQ_FILE" << EOF
"$BASE_URL/v1/chat/completions" 
EOF


echo "Request saved: $REQ_FILE"

curl -s -X POST "$BASE_URL/v1/images/generations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d @"$REQ_FILE" -o "$RESP_FILE"

echo "Response saved: $RESP_FILE"
echo ""

python3 - <<PYEOF
import json, base64, os, sys, urllib.request

resp_file = "$RESP_FILE"
output_dir = "$OUTPUT_DIR"
fmt = "$FORMAT"

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
    img_path = os.path.join(output_dir, f"result_{i}.{fmt}")
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

# macOS 打开图片
for img in "$OUTPUT_DIR"/result_*; do
  [ -f "$img" ] && open "$img"
done
