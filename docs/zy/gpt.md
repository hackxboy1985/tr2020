# zy渠道

# gpt-image-2（异步）

## OpenAPI Specification

```yaml
openapi: 3.0.1
info:
  title: ''
  description: ''
  version: 1.0.0
paths:
  /v1/videos:
    post:
      summary: gpt-image-2（异步）
      deprecated: false
      description: >-
        # gpt-image-2 接口文档（/v1/videos 视频接口格式）


        ## 接口地址

        `POST /v1/videos`


        ## 功能说明

        通过 `/v1/videos` 接口调用 GPT 图片生成能力（与视频生成共用接口，通过 `model`
        参数区分），支持文生图和图生图两种模式，采用异步任务方式，先提交返回任务 ID，再轮询获取结果。


        **画幅 / 宽高比**：支持三种控制方式：

        1. `aspect_ratio` 传入固定比例字符串（如 `9:16`、`16:9` 等），精确控制输出比例

        2. `size` 传入像素尺寸字符串（如 `"1280x720"`、`"1456x624"`）——与 `aspect_ratio`
        二选一，不需要同时传

        3. 传 `"auto"`，或**两个参数均不传**——两者等价，均根据提示词内容自动推断，无固定比例；提示词中若写明了比例（如"竖屏
        9:16"），模型也会据此生成


        **分辨率档位**（`gpt-image-2.5-flare` / `gpt-image-2.5-sunburst`
        用参数选档，见下文「分辨率控制」）：

        - 旧档 `gpt-image-2` / `gpt-image-2-2K` / `gpt-image-2-4K`：档位写在 `model` 里

        - `gpt-image-2.5`：**仅 1K**，不要传 `2K` / `4K`

        - `gpt-image-2.5-flare` / `gpt-image-2.5-sunburst`：同一模型出 **1K / 2K /
        4K**，用 `size` 像素，或 `aspect_ratio` + `image_size` / `resolution`；二者调用方式相同


        ## 支持的模型


        | model 参数 | 分辨率 | 说明 |

        |-----------|--------|------|

        | `gpt-image-2` | 1K | GPT 图片生成（标准档） |

        | `gpt-image-2-2K` | 2K | 2K 档，档位写在模型名里 |

        | `gpt-image-2-4K` | 4K | 4K 档，档位写在模型名里 |

        | `gpt-image-2.5` | **仅 1K** | 新版标准档；不支持 2K / 4K |

        | `gpt-image-2.5-flare` | **1K / 2K / 4K** | 用参数选分辨率，不要换模型名 |

        | `gpt-image-2.5-sunburst` | **1K / 2K / 4K** | 调用方式与
        `gpt-image-2.5-flare` 相同 |


        以上均走同一套 `/v1/videos` 异步流程，仅 `model` 与分辨率控档方式不同；计费与渠道路由以 `model` 区分。


        ## 请求头


        | 参数名 | 类型 | 必填 | 说明 |

        |--------|------|------|------|

        | Authorization | string | 是 | Bearer YOUR_API_KEY |

        | Content-Type | string | 是 | application/json |


        ## 请求参数


        | 参数名 | 类型 | 必填 | 说明 |

        |--------|------|------|------|

        | model | string | 是 | 模型名称，见上表。大小写须与上表一致（`2K`/`4K` 为大写
        K；`gpt-image-2.5` 系列中间是点号） |

        | prompt | string | 是 | 文本提示词 |

        | aspect_ratio | string | 否 | 宽高比。传固定比例字符串（见下表）则精确控制；传 `"auto"`
        或**不传**则根据提示词自动推断。与 `size` **二选一** |

        | size | string | 否 | 像素尺寸，格式 `"宽x高"`（如
        `"1280x720"`、`"1456x624"`、`"3696x1584"`）。按对照表选 1K / 2K / 4K
        列即可直接出对应分辨率。与 `aspect_ratio` 二选一 |

        | image_size | string | 否 | 分辨率档位：`1K`、`2K`、`4K`（大写 K）。与 `resolution`
        **完全等价**，只传其中一个。配合 `aspect_ratio` 使用；`gpt-image-2.5` 仅允许 `1K`（可不传，默认 1K）
        |

        | resolution | string | 否 | 与 `image_size` 通用、等价，取值同样是 `1K` / `2K` /
        `4K`。不要与 `image_size` 同时传不同值 |

        | images | string[] | 否 | 参考图片数组（支持 Base64 或 URL），最多 8
        张。传此参数为图生图模式，不传或传空数组为文生图模式 |


        ### `aspect_ratio` 支持的值


        | 值 | 方向 | 适用场景 |

        |----|------|----------|

        | `auto` | 自动 | 不指定比例，根据提示词内容自动推断（与不传等价） |

        | `1:1` | 正方形 | 通用、社交媒体 |

        | `16:9` | 横向（宽屏） | 横屏内容、PC 端 |

        | `9:16` | 竖向 | 短视频、手机竖屏 |

        | `4:3` | 横向 | 传统屏幕 |

        | `3:4` | 竖向 | 竖屏内容 |

        | `3:2` | 横向 | 摄影常用 |

        | `2:3` | 竖向 | 竖版海报 |

        | `5:4` | 横向 | 大幅横图 |

        | `4:5` | 竖向 | Instagram 竖图 |

        | `21:9` | 超宽横向 | 影院宽幅 |


        ### 比例与 `size` 对照


        `gpt-image-2` / `gpt-image-2-2K` / `gpt-image-2-4K`：按 **model 档位**
        选对应列。  

        `gpt-image-2.5`：只能用 **1K** 列。  

        `gpt-image-2.5-flare` / `gpt-image-2.5-sunburst`：三列都可用，见下方「分辨率控制」。


        | 比例 | 1K | 2K | 4K |

        |------|----|----|-----|

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


        ### `gpt-image-2.5` / `gpt-image-2.5-flare` / `gpt-image-2.5-sunburst`
        分辨率控制


        `image_size` 与 `resolution` 是**通用字段**，语义完全相同，取值均为 `1K` / `2K` / `4K`（大写
        K），只传其中一个。


        | 模型 | 可用档位 | 说明 |

        |------|----------|------|

        | `gpt-image-2.5` | 仅 1K | 不传档位即 1K；不要传 `2K` / `4K`；`size` 只取上表 1K 列 |

        | `gpt-image-2.5-flare`、`gpt-image-2.5-sunburst` | 1K / 2K / 4K |
        调用方式相同；用下面两种方式选档，**不要**改成带 `-2K` / `-4K` 后缀的模型名 |


        **方式一：`size` 直接传目标像素**（与 `aspect_ratio` 二选一）


        按上表选对应列的 `"宽x高"`，像素本身就决定了 1K / 2K / 4K，不必再传 `image_size` /
        `resolution`。例如 `"size": "1456x624"` 为 21:9 的 1K，2K 用 `"3024x1296"`，4K 用
        `"3696x1584"`。`gpt-image-2.5` 只能传 1K 列。


        **方式二：`aspect_ratio` + `image_size` 或 `resolution`**


        `aspect_ratio` 传固定比例（如 `"21:9"`）；档位用 `image_size` 或
        `resolution`（二者等价）。例如 `"aspect_ratio": "21:9"` + `"image_size": "4K"` 即为
        21:9 的 4K。`gpt-image-2.5` 档位只能是 `1K` 或不传。


        > **宽高比 / 分辨率控制方式对比**

        >

        > | 方式 | 用法 | 效果 |

        > |------|------|------|

        > | 像素尺寸 | `"size": "1456x624"` 或 `"size": "3696x1584"` | 直接按像素出 1K / 2K
        / 4K，与 `aspect_ratio` 二选一 |

        > | 比例 + 档位 | `"aspect_ratio": "21:9"` + `"image_size"` / `"resolution"`
        | 比例与分辨率分开传；两字段通用等价 |

        > | 固定比例 | `"aspect_ratio": "9:16"` | 精确输出指定比例；旧档分辨率看 model，flare /
        sunburst 可再加 `image_size` / `resolution` |

        > | 自动推断 | `"aspect_ratio": "auto"` 或两者均不传 | 根据提示词内容自动决定，无固定比例 |

        > | 提示词描述 | 不传或传 `"auto"`，prompt 中写明比例 | 模型从提示词中读取比例并遵循 |


        ### `images` 数组元素传法


        | 方式 | 示例 |

        |------|------|

        | **图片 URL** | `"https://example.com/reference.jpg"` |

        | **Base64** | `"data:image/jpeg;base64,/9j/4AAQSkZJRg..."` |


        > **注意**：传入图片 URL 时，网关会自动将其下载并转为 Base64 后发给上游，请确保 URL 为**可公网访问的图片直链**。


        ## 请求示例


        ### 文生图 — 固定比例

        ```json

        {
          "model": "gpt-image-2",
          "prompt": "生成抖音带货风格主图，主体 xxx",
          "aspect_ratio": "9:16"
        }

        ```

        > `gpt-image-2.5` 同用法，仅 1K。旧档 2K/4K 把 `model` 改成 `gpt-image-2-2K` /
        `gpt-image-2-4K`。不传或 `"auto"` 则按提示词自动推断比例。


        ### 文生图 — size 像素（flare / sunburst 选 1K / 2K / 4K）

        ```json

        {
          "model": "gpt-image-2.5-flare",
          "prompt": "做个广告",
          "size": "1456x624"
        }

        ```

        > 对照表 1K 列即 1K；2K 用 `"3024x1296"`，4K 用 `"3696x1584"`。`gpt-image-2.5` 只能用
        1K 列。`gpt-image-2.5-sunburst` 把 `model` 换掉即可。


        ### 文生图 — 比例 + 分辨率（flare / sunburst）

        ```json

        {
          "model": "gpt-image-2.5-flare",
          "prompt": "做个广告",
          "aspect_ratio": "21:9",
          "image_size": "4K"
        }

        ```

        > `image_size` 与 `resolution` 等价，只传一个。`gpt-image-2.5-sunburst` 把 `model`
        换掉即可。


        ### 图生图

        ```json

        {
          "model": "gpt-image-2.5-flare",
          "prompt": "做个广告",
          "aspect_ratio": "21:9",
          "image_size": "4K",
          "images": [
            "https://www.baidu.com/img/PCtm_d9c8750bed0b3c7d089fa7d55720d6cf.png"
          ]
        }

        ```

        > `images` 也可传 Base64（`data:image/jpeg;base64,...`）。不传或空数组为文生图。


        ## 响应参数


        图片任务也走 `/v1/videos`，`object` 固定为 `"video"`。完成态图片地址在顶层 `url`，同时也会写在
        `metadata.image_url` / `metadata.image`，三者同值。


        | 参数名 | 类型 | 说明 |

        |--------|------|------|

        | id | string | 任务 ID，格式：`task_xxxx` |

        | object | string | 固定值：`video`（图片任务也是，类型看 `model`） |

        | model | string | 使用的模型名称 |

        | status | string |
        任务状态：`queued`（排队中）、`in_progress`（处理中）、`completed`（已完成）、`failed`（失败） |

        | progress | number | 任务进度，0–100 |

        | created_at | number | 创建时间戳（秒） |

        | completed_at | number | 完成时间戳（秒），仅在 `completed` 或 `failed` 状态返回 |

        | url | string | 生成的图片 URL，仅 `completed` 返回 |

        | metadata | object | 完成态结果；见下表 |

        | metadata.image_url | string | 与顶层 `url` 同值，仅 `completed` 返回 |

        | metadata.image | string | 与顶层 `url` 同值，可能同时存在 |

        | error | object | 错误信息，仅在 `failed` 状态返回 |

        | error.message | string | 失败原因描述 |

        | error.code | string | 错误码，如 `upstream_error` |


        ## 响应示例


        ### 提交成功（排队中）

        ```json

        {
          "id": "task_1776831820897",
          "object": "video",
          "model": "gpt-image-2",
          "status": "queued",
          "progress": 0,
          "created_at": 1709876543
        }

        ```


        ### 任务处理中

        ```json

        {
          "id": "task_1776831820897",
          "object": "video",
          "model": "gpt-image-2",
          "status": "in_progress",
          "progress": 10,
          "created_at": 1709876543
        }

        ```


        ### 任务完成

        ```json

        {
          "id": "task_1776831820897",
          "object": "video",
          "model": "gpt-image-2",
          "status": "completed",
          "progress": 100,
          "created_at": 1709876543,
          "completed_at": 1709876598,
          "url": "https://example.com/uploads/gpt-images/task_1776831820897.png",
          "metadata": {
            "image": "https://example.com/uploads/gpt-images/task_1776831820897.png",
            "image_url": "https://example.com/uploads/gpt-images/task_1776831820897.png"
          }
        }

        ```


        ### 任务失败

        ```json

        {
          "id": "task_1776831820897",
          "object": "video",
          "model": "gpt-image-2",
          "status": "failed",
          "created_at": 1718123456,
          "completed_at": 1718123456,
          "progress": 100,
          "error": {
            "message": "上游任务失败原因",
            "code": "upstream_error"
          }
        }

        ```


        ## 任务查询接口


        ### 接口地址

        `GET /v1/videos/{task_id}`


        ### 请求示例

        ```bash

        curl -X GET https://xxx.com/v1/videos/task_1776831820897 \
          -H "Authorization: Bearer YOUR_API_KEY"
        ```


        ### 响应说明

        返回字段与提交接口一致，根据 `status` 字段判断任务是否完成：

        - `queued` / `in_progress`：任务未完成，继续轮询

        - `completed`：任务完成，从顶层 `url`（或 `metadata.image_url`）获取图片地址

        - `failed`：任务失败，从 `error.message` 获取错误原因


        ## 注意事项


        1. **接口复用**：GPT 图片生成使用 `/v1/videos` 接口，与视频生成共用，通过 `model` 参数区分；支持
        `gpt-image-2`、`gpt-image-2-2K`、`gpt-image-2-4K`、`gpt-image-2.5`、`gpt-image-2.5-flare`、`gpt-image-2.5-sunburst`

        2. **参数位置**：`aspect_ratio`、`size`、`image_size`、`resolution`、`images`
        均为顶层字段，直接放在请求体根对象中

        3. **宽高比 / 分辨率**：
           - 传 `size`（如 `"1456x624"`、`"3696x1584"`）→ 直接按像素出 1K / 2K / 4K，与 `aspect_ratio` 二选一，取值见上表
           - 传 `aspect_ratio` + `image_size` 或 `resolution` → 比例与档位分开控制；`image_size` 与 `resolution` 通用等价（`1K`/`2K`/`4K`），只传一个
           - 传 `"auto"` 或**不传** `aspect_ratio`/`size` → 完全等价，根据提示词内容自动推断；提示词中若描述了比例，模型会据此生成
           - `gpt-image-2.5` **仅 1K**；`gpt-image-2.5-flare` / `gpt-image-2.5-sunburst` 才支持用参数选 2K / 4K
        4. **参考图片格式**：支持 JPEG、PNG、WEBP 格式

        5. **参考图片来源**：支持两种格式
           - **Base64 格式**：需包含完整的 Data URL 前缀（如：`data:image/jpeg;base64,`）
           - **URL 格式**：直接传入可访问的图片 URL 地址
        6. **任务模式**：
           - 不传 `images` 或传空数组 = 文生图模式
           - `images` 包含图片（Base64 或 URL）= 图生图模式
        7. **异步处理**：接口返回任务 ID 后，轮询 `GET /v1/videos/{task_id}`，建议间隔 2~5
        秒。`completed` 读顶层 `url`（与 `metadata.image_url` 同值）

        8. **图片有效期**：生成的图片地址有效期为 5 小时，请及时下载保存
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
                aspect_ratio:
                  type: string
                images:
                  type: array
                  items:
                    type: string
              required:
                - model
                - prompt
                - aspect_ratio
                - images
              x-apifox-orders:
                - model
                - prompt
                - aspect_ratio
                - images
            example:
              model: gpt-image-2
              prompt: 根据图片做一个广告
              aspect_ratio: '16:9'
              images:
                - https://xxx.cc/xxx.jpg
                - >-
                  https://www.baidu.com/img/PCtm_d9c8750bed0b3c7d089fa7d55720d6cf.png
      responses:
        '200':
          description: ''
          content:
            application/json:
              schema:
                type: object
                properties: {}
          headers: {}
          x-apifox-name: 成功
      security: []
      x-apifox-folder: 图片生成（Images）
      x-apifox-status: developing
      x-run-in-apifox: https://app.apifox.com/web/project/7902379/apis/api-447634846-run
components:
  schemas: {}
  securitySchemes: {}
servers: []
security: []

```