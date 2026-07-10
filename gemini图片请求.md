上游请求示例：

curl https://luluai.cc/v1beta/models/gemini-2.5-flash-image:generateContent \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-YOUR_TOKEN" \
  -d '
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "生成高空俯视图"
        },
        {
          "inlineData": {
            "mimeType": "image/png",
            "data": "iVBORw0KGgoAAAANSUhEUgAACgAAAAWgCAIAAAAdYo2IAAAA03RFWHRBSUdDAHsiTGFiZWwiOiIxIiwiQ"
          }
        }
      ]
    }
  ],
  "generationConfig": {
    "response_modalities": [
      "IMAGE",
      "TEXT"
    ],
    "imageConfig": {
      "aspectRatio": "16:9",
      "imageSize": "1K"
    }
  }
}'


上游返回示例：


{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "`"
          },
          {
            "inlineData": {
              "mimeType": "image/png",
              "data": "iadfdfdsfsfewfdsfefefdsfsdfCC"
            }
          }
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 264,
    "candidatesTokenCount": 1291,
    "totalTokenCount": 1555,
    "promptTokensDetails": [
      {
        "modality": "TEXT",
        "tokenCount": 6
      },
      {
        "modality": "IMAGE",
        "tokenCount": 258
      }
    ],
    "candidatesTokensDetails": [
      {
        "modality": "IMAGE",
        "tokenCount": 1290
      }
    ],
    "serviceTier": "standard"
  },
  "modelVersion": "gemini-2.5-flash-image",
  "responseId": "BgtOas67BpOJqtsP3pXGqQU"
}