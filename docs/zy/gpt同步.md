# image2（同步）

## OpenAPI Specification

```yaml
openapi: 3.0.1
info:
  title: ''
  description: ''
  version: 1.0.0
paths:
  /v1/images/generations:
    post:
      summary: image2（同步）
      deprecated: false
      description: >
        # gpt-image-2 接口文档（OpenAI 原生格式）


        ## 接口地址


        | 用途 | 方法 | 路径 | 响应格式 |

        |------|------|------|----------|

        | 文生图（图片 API） | POST | `/v1/images/generations` | `{ created, data: [{
        url, b64_json }] }`（缺省双给，可用 `response_format` 单选） |

        | 图生图（图片编辑） | POST | `/v1/images/edits` | 同上 |

        | 文生图 / 图生图（Chat API） | POST | `/v1/chat/completions` |
        `chat.completion`；图片在 `message.content` 的 markdown 里，Base64 在
        `message.images[0].b64_json` |



        ## 功能说明


        采用 OpenAI 原生端点格式的图片生成接口，支持文生图和图生图两种模式。


        - **`/v1/images/generations`、`/v1/images/edits`**：标准图片 API，缺省返回 `{ data:
        [{ url, b64_json }] }`。

        - **`/v1/chat/completions`**：兼容 Chat Completions 调用方式；将 `messages`
        自动转为内部提示词与参考图，**响应为 Chat 格式**（`![image](链接)`），Base64 走扩展字段。


        三个端点的 **`response_format` 语义完全一致**：


        | 传值 | 返回内容 |

        |------|----------|

        | **不传**（推荐） | **同时返回 URL 与 Base64** |

        | `"url"` | 只返回图片 URL |

        | `"b64_json"` | 只返回 Base64（纯字符串，不含 `data:` 前缀） |


        > 缺省双给时回包体积更大；若只需要链接，请显式传 `"response_format": "url"`。

        ---


        ## 支持的模型


        | model 参数 | 分辨率 | 说明 |

        |-----------|--------|------|

        | `gpt-image2` | **1K / 2K / 4K** | 全档位；需要 2K、4K 或更大像素尺寸时使用 |

        | `image2` | **仅 1K** | 更便宜；只需 1K 档图片时优先使用 |


        **选型建议**


        - 要用 **1K、2K、4K** 或较大自定义像素（如 `3696x1584`）→ 用 **`gpt-image2`**

        - 只要 **1K**，追求更低成本 → 用 **`image2`**


        ---


        ## 请求头


        | 参数名 | 类型 | 必填 | 说明 |

        |--------|------|------|------|

        | Authorization | string | 是 | `Bearer YOUR_API_KEY` |

        | Content-Type | string | 是 | 图片 API：支持 **`application/json`** 与
        **`multipart/form-data`**；Chat API：**仅 `application/json`** |


        ---


        ## 一、文生图接口


        ### 接口地址


        `POST /v1/images/generations`


        ### 请求参数


        | 参数名 | 类型 | 必填 | 说明 |

        |--------|------|------|------|

        | model | string | 是 | 模型名称 |

        | prompt | string | 是 | 图片描述，用于生成图片 |

        | size | string | 否 | 图片尺寸，格式 `"宽x高"`。同步接口**仅支持
        `size`**，固定比例见下表；`image2` 仅可用 1K 列 |

        | response_format | string | 否 | 不传 = URL + Base64 双给；`"url"` =
        只要链接；`"b64_json"` = 只要 Base64 |


        ### 常用 size 速查（1K 列，`image2` 可用）


        | 比例 | size（1K） | size（2K） | size（4K） |

        |------|-----------|------------------------------|------------------------------|

        | 1:1 | 1024x1024 | 2048x2048 | 2880x2880 |

        | 16:9 | 1280x720 | 2560x1440 | 3840x2160 |

        | 9:16 | 720x1280 | 1440x2560 | 2160x3840 |

        | 3:2 | 1248x832 | 2496x1664 | 3504x2336 |

        | 2:3 | 832x1248 | 1664x2496 | 2336x3504 |

        | 4:3 | 1152x864 | 2304x1728 | 3264x2448 |

        | 3:4 | 864x1152 | 1728x2304 | 2448x3264 |

        | 5:4 | 1120x896 | 2240x1792 | 3200x2560 |

        | 4:5 | 896x1120 | 1792x2240 | 2560x3200 |

        | 21:9 | 1456x624 | 3024x1296 | 3696x1584 |


        ### 请求示例（JSON — 纯文生图，1K 省钱）


        ```bash

        curl -X POST "https://xxxxxx.com/v1/images/generations" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          -H "Content-Type: application/json" \
          -d '{
            "model": "image2",
            "prompt": "生成竖屏图片 xxx抖音带货",
            "size": "720x1280"
          }'
        ```


        ### 请求示例（JSON — 纯文生图，支持 2K/4K）


        ```bash

        curl -X POST "https://xxxxxx.com/v1/images/generations" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          -H "Content-Type: application/json" \
          -d '{
            "model": "gpt-image2",
            "prompt": "生成竖屏图片 xxx抖音带货",
            "size": "3696x1584"
          }'
        ```


        ### 请求示例（表单格式）


        ```bash

        curl -X POST "https://xxxxxx.com/v1/images/generations" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          -H "Content-Type: multipart/form-data" \
          --form 'prompt="生成竖屏图片 xxx抖音带货"' \
          --form 'model="image2"' \
          --form 'size="720x1280"'
        ```


        ---


        ## 二、图生图（图片编辑）接口


        ### 接口地址


        `POST /v1/images/edits`


        ### 请求参数


        | 参数名 | 类型 | 必填 | 说明 |

        |--------|------|------|------|

        | model | string | 是 | `gpt-image2` 或 `image2` |

        | prompt | string | 是 | 图片描述，用于编辑图片 |

        | image | 见说明 | 是 | **JSON**：`string`（单图）或
        `string[]`（多图）；**Multipart**：文件字段（常见名 `image`） |

        | size | string | 否 | 图片尺寸，格式同文生图 |

        | response_format | string | 否 | 不传 = URL + Base64 双给；`"url"` =
        只要链接；`"b64_json"` = 只要 Base64 |


        ### 请求示例（JSON — URL）


        ```bash

        curl -X POST "https://xxxxxx.com/v1/images/edits" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          -H "Content-Type: application/json" \
          -d '{
            "model": "gpt-image2",
            "prompt": "根据图片做一个广告",
            "size": "3024x1296",
            "image": [
              "https://example.com/reference.png"
            ]
          }'
        ```


        ### 请求示例（JSON — Data URL / Base64）


        ```json

        {
          "model": "gpt-image2",
          "prompt": "变成灰色",
          "size": "720x1280",
          "image": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
        }

        ```


        多图时 `"image": ["data:image/png;base64,...",
        "https://..."]`，单图可传字符串或单元素数组。


        ### 请求示例（表单格式 — 本地文件）


        ```bash

        curl -X POST "https://xxxxxx.com/v1/images/edits" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          --form 'image=@"/path/to/example.jpg"' \
          --form 'prompt="变成灰色"' \
          --form 'model="image2"' \
          --form 'size="720x1280"'
        ```


        ### Python 示例（JSON，直接传 URL）


        ```python

        import httpx


        resp = httpx.post(
            "https://xxxxxx.com/v1/images/edits",
            headers={"Authorization": "Bearer YOUR_API_KEY"},
            json={
                "model": "gpt-image2",
                "prompt": "根据图片做一个广告",
                "size": "3024x1296",
                "image": ["https://example.com/reference.png"],
            },
        )

        print(resp.json())

        ```


        ---


        ## 三、Chat Completions 接口（文生图 / 图生图）


        ### 接口地址


        `POST /v1/chat/completions`


        ### 功能说明


        - 入参使用 OpenAI Chat 的 **`messages`** 格式（`text` + 可选 `image_url`）。

        - 自动转为内部提示词与参考图，与图片 API **共用**同一套生成逻辑。

        - 响应为 **`chat.completion`**：
          - 图片链接在 `choices[0].message.content`，形如：`![image](https://成品图地址)\n\n`
          - Base64 在 `choices[0].message.images[0].b64_json`（扩展字段）
          - 传 `"response_format": "b64_json"` 且没有 URL 时，`content` 会退化为 `![image](data:image/png;base64,...)`

        ### 请求参数


        | 参数名 | 类型 | 必填 | 说明 |

        |--------|------|------|------|

        | model | string | 是 | `gpt-image2` 或 `image2` |

        | messages | array | 是 | 至少一条 `role: user` 的消息 |

        | size | string | 否 | 图片尺寸，如 `720x1280`、`3696x1584` |

        | response_format | string | 否 | 不传 = URL + Base64 双给；`"url"` =
        只要链接；`"b64_json"` = 只要 Base64。**只认字符串**；若传对象（如
        `{"type":"json_object"}`）会被忽略，按缺省双给处理 |


        `messages[].content` 支持两种写法：


        1. **字符串**：纯文生图  
           `"content": "做一个广告"`
        2. **数组**：文生图或图生图  
           - `{ "type": "text", "text": "..." }`  
           - `{ "type": "image_url", "image_url": { "url": "https://..." } }`（图生图）

        > 图生图请使用 **`image_url`**，不要用顶层 `image` 字段（那是图片 API 的字段名）。


        ### 请求示例 — Chat 文生图（纯文本）


        **写法 A：`content` 为字符串**


        ```bash

        curl -X POST "https://xxxxxx.com/v1/chat/completions" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          -H "Content-Type: application/json" \
          -d '{
            "model": "gpt-image2",
            "messages": [
              {
                "role": "user",
                "content": "做一个广告"
              }
            ],
            "size": "3696x1584"
          }'
        ```


        **写法 B：`content` 为数组（仅 text）**


        ```json

        {
          "model": "image2",
          "messages": [
            {
              "role": "user",
              "content": [
                { "type": "text", "text": "做一个广告" }
              ]
            }
          ],
          "size": "720x1280"
        }

        ```


        ### 请求示例 — Chat 图生图（text + 参考图）


        ```bash

        curl -X POST "https://xxxxxx.com/v1/chat/completions" \
          -H "Authorization: Bearer YOUR_API_KEY" \
          -H "Content-Type: application/json" \
          -d '{
            "model": "gpt-image2",
            "messages": [
              {
                "role": "user",
                "content": [
                  { "type": "text", "text": "根据图片做一个广告" },
                  {
                    "type": "image_url",
                    "image_url": {
                      "url": "https://example.com/reference.png"
                    }
                  }
                ]
              }
            ],
            "size": "3696x1584"
          }'
        ```


        ### Chat 响应示例（缺省：URL + Base64）


        ```json

        {
          "id": "chatcmpl-d202e7d4-370f-4dd4-817c-6f7d212cc2e3",
          "object": "chat.completion",
          "created": 1784359143,
          "model": "gpt-image2",
          "choices": [
            {
              "index": 0,
              "message": {
                "role": "assistant",
                "content": "![image](https://oss-us.file-download.life/2026/07/18/7acea2ac13eca4beae93f427862e6f9e.png)\n\n",
                "images": [
                  {
                    "type": "image_url",
                    "image_url": {
                      "url": "https://oss-us.file-download.life/2026/07/18/7acea2ac13eca4beae93f427862e6f9e.png"
                    },
                    "b64_json": "iVBORw0KGgoAAAANSU..."
                  }
                ]
              },
              "finish_reason": "stop"
            }
          ]
        }

        ```


        ### Chat 取参


        | 需要 | 取哪里 |

        |------|--------|

        | 图片 URL | `choices[0].message.content` 里的
        markdown：`![image](https://...)`；或
        `choices[0].message.images[0].image_url.url` |

        | Base64 | `choices[0].message.images[0].b64_json` |

        | 只要 URL | 请求加 `"response_format": "url"`，回包无 `b64_json` |

        | 只要 Base64 | 请求加 `"response_format": "b64_json"`；此时 `content` 可能是
        `![image](data:image/png;base64,...)`，也可直接读 `images[0].b64_json` |


        > `message.images` 为扩展字段。若你的调用链路（如部分中转）只透传官方 Chat 结构、丢掉扩展字段，需要 Base64
        时请显式传 `"response_format": "b64_json"`，从 `content` 的 data-uri 中解析。


        ### Python 示例（Chat 图生图）


        ```python

        import httpx


        resp = httpx.post(
            "https://xxxxxx.com/v1/chat/completions",
            headers={"Authorization": "Bearer YOUR_API_KEY"},
            json={
                "model": "gpt-image2",
                "messages": [{
                    "role": "user",
                    "content": [
                        {"type": "text", "text": "根据图片做一个广告"},
                        {"type": "image_url", "image_url": {"url": "https://example.com/reference.png"}},
                    ],
                }],
                "size": "3696x1584",
            },
            timeout=600,
        )

        data = resp.json()

        content = data["choices"][0]["message"]["content"]

        print(content)

        ```


        ---


        ## 四、响应示例（图片 API）


        ### 缺省（同时返回 URL 与 Base64）


        ```json

        {
          "created": 1782108238,
          "data": [
            {
              "url": "https://res.papir.cc/output/2026-04-18/xxxxxx.png",
              "b64_json": "iVBORw0KGgoAAAANSU..."
            }
          ]
        }

        ```


        ### 只要 URL（请求体加 `"response_format": "url"`）


        ```json

        {
          "created": 1782108238,
          "data": [
            {
              "url": "https://res.papir.cc/output/2026-04-18/xxxxxx.png"
            }
          ]
        }

        ```


        ### 只要 Base64（请求体加 `"response_format": "b64_json"`）


        ```json

        {
          "created": 1782108238,
          "data": [
            {
              "b64_json": "iVBORw0KGgoAAAANSU..."
            }
          ]
        }

        ```


        ### 图片 API 取参


        | 需要 | 取哪里 |

        |------|--------|

        | 图片 URL | `data[0].url` |

        | Base64 | `data[0].b64_json`（纯字符串，不含 `data:` 前缀） |


        请求示例（在原有请求体上增加字段即可）：


        ```json

        {
          "model": "gpt-image2",
          "prompt": "一只可爱的猫",
          "size": "1024x1024",
          "response_format": "b64_json"
        }

        ```


        ### 任务状态说明（异步上游时）


        | status | 说明 |

        |--------|------|

        | `queued` | 任务排队中 |

        | `in_progress` | 任务处理中 |

        | `completed` | 任务已完成 |

        | `failed` | 任务失败 |


        ---


        ## 注意事项


        1. **接口格式**：采用 OpenAI 原生端点，与常见 OpenAI SDK / 兼容客户端通用。

        2. **三种端点怎么选**：
           - 纯文生图、要标准 `{ data: [{ url, b64_json }] }` → **`/v1/images/generations`**
           - 带参考图的图生图 → **`/v1/images/edits`**
           - 客户端已是 Chat / `messages` 形态 → **`/v1/chat/completions`**
        3. **`response_format`（三个端点一致）**：
           - 不传 = **URL + Base64 双给**
           - `"url"` = 只要图床链接
           - `"b64_json"` = 只要 Base64
           - Chat 端点只认字符串；传对象会被忽略，按缺省双给处理
        4. **响应格式**：
           - 图片 API：读 `data[0].url` / `data[0].b64_json`
           - Chat API：读 `message.content` 的 markdown；Base64 读 `message.images[0].b64_json`
        5. **模型与分辨率**：
           - **`gpt-image2`**：1K / 2K / 4K，适合高分辨率或高档位
           - **`image2`**：仅 1K，价格更低
        6. **尺寸**：同步接口仅支持 `size` 控制画幅，固定比例见上文对照表；`image2` 仅可用 1K 列

        7. **参考图**（图生图）：
           - **`/v1/images/edits`**：JSON 下用字段 **`image`**（URL / Base64 / 数组）；multipart 上传文件
           - **`/v1/chat/completions`**：在 **`messages[].content`** 里用 **`image_url`**
           - 参考图 URL 须**公网可访问**（上游服务器需能下载）；防盗链地址可能报 `Failed to fetch image_url`
        8. **参考图格式**：支持 JPEG、PNG、WEBP 等常见图片格式

        9. **图片有效期**：返回 URL 通常有时效（如约 2 小时），请及时保存

        10. **回包体积**：缺省双给时 Base64 会显著增大响应体；只需要链接时请传 `"response_format": "url"`
      tags:
        - 图片生成（Images）
      parameters:
        - name: Authorization
          in: header
          description: ''
          required: true
          example: Bearer {{YOUR_API_KEY}}
          schema:
            type: string
        - name: Content-Type
          in: header
          description: ''
          required: false
          example: application/json
          schema:
            type: string
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                model:
                  type: string
                prompt:
                  type: string
                size:
                  type: string
                image:
                  type: array
                  items:
                    type: string
              required:
                - model
                - prompt
                - size
                - image
              x-apifox-orders:
                - model
                - prompt
                - size
                - image
            example:
              model: image2
              prompt: 根据图片做一个广告
              size: 1024x1792
              image:
                - >-
                  https://res.papir.cc/user-upload/creati-web-app/2026-04-18/1776519962551vv1AXmBu-ZMWqAckJIbt81167-600x751h.jpg
                - >-
                  https://www.baidu.com/img/PCtm_d9c8750bed0b3c7d089fa7d55720d6cf.png
      responses:
        '200':
          description: ''
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      type: object
                      properties:
                        b64_json:
                          type: string
                      x-apifox-orders:
                        - b64_json
                  created:
                    type: integer
                  usage:
                    type: object
                    properties:
                      input_tokens:
                        type: integer
                      output_tokens:
                        type: integer
                      total_tokens:
                        type: integer
                      input_tokens_details:
                        type: object
                        properties:
                          text_tokens:
                            type: integer
                          image_tokens:
                            type: integer
                        required:
                          - text_tokens
                          - image_tokens
                        x-apifox-orders:
                          - text_tokens
                          - image_tokens
                      output_tokens_details:
                        type: object
                        properties:
                          text_tokens:
                            type: integer
                          image_tokens:
                            type: integer
                        required:
                          - text_tokens
                          - image_tokens
                        x-apifox-orders:
                          - text_tokens
                          - image_tokens
                    required:
                      - input_tokens
                      - output_tokens
                      - total_tokens
                      - input_tokens_details
                      - output_tokens_details
                    x-apifox-orders:
                      - input_tokens
                      - output_tokens
                      - total_tokens
                      - input_tokens_details
                      - output_tokens_details
                required:
                  - data
                  - created
                  - usage
                x-apifox-orders:
                  - data
                  - created
                  - usage
              example:
                data:
                  - b64_json: iVBORw0KGgoAAAANSU...
                created: 1782108238
                usage:
                  input_tokens: 4
                  output_tokens: 1105
                  total_tokens: 1109
                  input_tokens_details:
                    text_tokens: 4
                    image_tokens: 0
                  output_tokens_details:
                    text_tokens: 0
                    image_tokens: 1105
          headers: {}
          x-apifox-name: 成功
      security: []
      x-apifox-folder: 图片生成（Images）
      x-apifox-status: developing
      x-run-in-apifox: https://app.apifox.com/web/project/7902379/apis/api-447357296-run
components:
  schemas: {}
  securitySchemes: {}
servers: []
security: []

```