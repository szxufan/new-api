# Provider 后端 HTTP 接口要求（对接 hailuo/MiniMax v2 适配器）

> **读者**：实现视频生成 Provider 后端的开发者。你的服务将作为上游，被 new-api 的
> `hailuo.TaskAdaptor`（渠道类型 `ChannelTypeMiniMax = 35`，v2/H3 协议，模型 `MiniMax-H3`）调用。
> 该适配器是本项目对 `/v1/videos` 功能支持最全面的适配器（5 类统一素材键全覆盖）。
> 适配器代码：[relay/channel/task/hailuo/](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo)；
> 协议适配说明：[minimax-video-h3-v2.md](./minimax-video-h3-v2.md)。
> 本文契约逐条对照适配器代码核实。

## 0. 你需要实现的接口总览

| #   | 方法         | 路径                                   | 用途                                 | 必须实现           |
| --- | ------------ | -------------------------------------- | ------------------------------------ | ------------------ |
| 1   | POST         | `/v2/video_generation`                 | 创建视频生成任务                     | ✅                 |
| 2   | GET          | `/v2/query/video_generation/{task_id}` | 查询任务状态与结果                   | ✅                 |
| 3   | （视频下载） | `content.url` 直链                     | 轮询成功后本系统/客户端直接 GET 下载 | ✅（返回可用直链） |

仅需 2 个端点 + 1 个可下载直链。认证为 **HTTP Bearer**：`Authorization: Bearer <你的 API Key>`，即渠道配置中的密钥。

## 1. 创建任务：`POST /v2/video_generation`

适配器调用点：[adaptor.go#L43-L48](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor.go#L43-L48)（`BuildRequestURL` 拼 `baseURL + V2CreateEndpoint`）、[adaptor.go#L57-L75](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor.go#L57-L75)（`BuildRequestBody`）。

### 1.1 请求头

```http
Content-Type: application/json
Accept: application/json
Authorization: Bearer <API_KEY>
```

### 1.2 请求体

来源结构 [models_v2.go#L29-L37](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/models_v2.go#L29-L37)（`V2VideoRequest`）：

```jsonc
{
  "model": "MiniMax-H3", // string，必填。适配器已替换为渠道映射后的上游模型名
  "content": [
    /* 见 1.3 */
  ], // array，必填。多模态输入数组
  "resolution": "768P", // string，必填。仅接受 "768P" | "2K"
  "duration": 5, // int，必填。4~15 的整数秒
  "ratio": "16:9", // string，可选。合法值见 1.4；i2v 场景恒为 "adaptive"
  "callback_url": "...", // string，可选。仅当客户端 metadata 显式传入
  "aigc_watermark": true, // bool，可选。仅当客户端 metadata 显式传入
}
```

**参数取值保证**（适配器提交前已收敛，你可信任但建议仍做防御性校验）：

| 字段         | 收敛规则（[adaptor_v2.go#L198-L254](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L198-L254)） |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| `resolution` | 只会是 `"768P"` 或 `"2K"`（`1080P` 及以上向上归为 `2K`；无法识别默认 `2K`）                                                      |
| `duration`   | 只会是 4~15 整数（非正值默认 5）                                                                                                 |
| `ratio`      | `adaptive` / `21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` 之一                                                               |
| `content`    | 必含至少 1 个非空 `text` 项；首/尾帧与参考素材不混用；数量不超上限（见 1.3）                                                     |

### 1.3 `content[]` 多模态输入

来源结构 [models_v2.go#L18-L25](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/models_v2.go#L18-L25)（`V2ContentItem`）。每个元素按 `type` 取对应字段：

| `type`      | 携带字段             | `role` 取值                                      | 数量上限                 |
| ----------- | -------------------- | ------------------------------------------------ | ------------------------ |
| `text`      | `text`（≤7000 字符） | 无（适配器会清空该字段）                         | —                        |
| `image_url` | `image_url.url`      | `first_frame` / `last_frame` / `reference_image` | 首帧 1、尾帧 1、参考图 9 |
| `video_url` | `video_url.url`      | `reference_video`                                | 3                        |
| `audio_url` | `audio_url.url`      | `reference_audio`                                | 3                        |

媒体元素总数 ≤ 12（`text` 不计入）。`url` 一律为**公网 URL**（本系统不做 Base64 转换；请求体建议按 ≤64MB 设计）。

```jsonc
// 示例：首尾帧插值
"content": [
  { "type": "text", "text": "小女孩长大了" },
  { "type": "image_url", "image_url": { "url": "https://example.com/a.png" }, "role": "first_frame" },
  { "type": "image_url", "image_url": { "url": "https://example.com/b.png" }, "role": "last_frame" }
]

// 示例：多模态参考（图 + 视频 + 音频）
"content": [
  { "type": "text", "text": "角色跟随节奏起舞" },
  { "type": "image_url", "image_url": { "url": "https://example.com/1.png" }, "role": "reference_image" },
  { "type": "video_url", "video_url": { "url": "https://example.com/motion.mp4" }, "role": "reference_video" },
  { "type": "audio_url", "audio_url": { "url": "https://example.com/voice.wav" }, "role": "reference_audio" }
]
```

互斥约束（上游协议规定，适配器已保证）：`first_frame`/`last_frame` 与 `reference_*` 不可同时出现。若违反，适配器会把首/尾帧降级为 `reference_image`，你收到的请求不会违反。

### 1.4 `ratio` 按场景的取值（[adaptor_v2.go#L256-L291](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L256-L291)）

- content 含 `first_frame`/`last_frame` → 恒为 `adaptive`（宽高比跟随输入图）；
- content 含 `reference_*` → 可为合法比例或 `adaptive`；
- 仅 `text` → 必须为具体比例（不会是 `adaptive`），缺省 `16:9`。

### 1.5 成功响应（HTTP 200）

```json
{ "task_id": "your-upstream-task-id" }
```

- **必须返回非空 `task_id`**。适配器读取该值作为上游任务 ID，供轮询使用；客户端看到的是本系统生成的 `task_xxxx` 公开 ID，你的 ID 不会外泄。
- 其余字段可省略（适配器只读 `task_id`）。

### 1.6 错误响应

错误信封结构 [models_v2.go#L44-L54](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/models_v2.go#L44-L54)（`V2ErrorEnvelope`）：

```json
{
  "type": "error",
  "error": {
    "type": "invalid_param",
    "message": "resolution must be 768P or 2K",
    "http_code": "400" // 字符串或数字均可，适配器兼容两种
  },
  "request_id": "req_xxx"
}
```

- **建议配合非 2xx HTTP 状态码**（400/401/402/429/500 等）。`http_code` 不存在时适配器以实际 HTTP 状态码为准。
- `error.type` / `error.message` 至少填一个（message 用于透出给客户端）。
- 语义约定（影响 new-api 侧重试行为）：
  - 4xx 参数/鉴权类错误 → 通常不重试，直接返回客户端；
  - 429 / 5xx → new-api 可能换渠道重试或提示「上游负载饱和」。

> 若 `task_id` 为空且无法解析出 `error.message`，适配器会以 `minimax_v2_error` + 原始 body 报错（[adaptor_v2.go#L396-L428](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L396-L428)）。

### 1.7 模型名称（`model` 字段）具体要求

**你会收到什么值**

- `model` = 渠道**模型映射后**的上游模型名（`info.UpstreamModelName`）。管理员在渠道设置中未配置映射时，即为客户端请求的模型名（通常是 `MiniMax-H3`）。映射链路见 [relay/relay_task.go#L160-L171](file:///home/xufan/trea_project/new-api/relay/relay_task.go#L160-L171)。
- 客户端**无法**通过 `metadata.model` 覆盖它——适配器在透传 metadata 前会剔除 `model` 键，防止绕过计费（[taskcommon/helpers.go#L21](file:///home/xufan/trea_project/new-api/relay/channel/task/taskcommon/helpers.go#L21)）。
- 大小写与命名原样保留：请求体中的 `model` 就是映射后的字符串，不改写、不加前缀。

**v2 协议路由规则（关键约束）**

适配器按模型名前缀判定协议：`UpstreamModelName` 以 `minimax-h3` 开头（**大小写不敏感**）→ v2 协议；否则回退 v1 协议（`IsVideoV2Model`，[constants.go#L126-L130](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/constants.go#L126-L130)）。因此：

- **若渠道配置了模型映射，映射后的名称仍必须以 `minimax-h3` 开头**（如 `minimax-h3-yourbrand-v1`）。若映射成不带该前缀的名字（如 `my-video-pro`），请求会被误路由到 v1 端点 `/v1/video_generation`，你的 v2 接口将收不到流量。
- 前缀规则天然兼容变体命名（`MiniMax-H3-Fast`、`MiniMax-H3-Preview` 等），你可用后缀区分版本/租户。

**Provider 侧要求**

1. **白名单校验**：`model` 不在你支持的范围内时返回 4xx + 错误信封（§1.6 格式），**不要静默替换成其他模型**——客户端按 `model` 计费，替换会造成「钱花了、结果不对」。
2. 查询响应中的 `task.model` 适配器不读取（仅透传展示），建议回显创建时收到的模型名，便于排障。

**new-api 侧配置前提（对接前与渠道管理员确认）**

| 项           | 要求                                                                                                                                                                                                                                |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 渠道模型列表 | 客户端可见的模型名需出现在适配器 `ModelList`（[constants.go#L9-L20](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/constants.go#L9-L20)，已含 `MiniMax-H3`）并配置到渠道的「模型」列表，否则请求无法路由到该渠道 |
| 模型映射     | 若配置，映射目标必须保留 `minimax-h3` 前缀（见上）                                                                                                                                                                                  |
| 模型定价     | 需在「系统设置 → 计费 → 模型定价」配置基础价，未配置按默认倍率计费                                                                                                                                                                  |

**注意：本文档仅适用于「MiniMax 官方渠道」通路。** MiniMax-H3 在系统内有两条互不相干的接入通路：

|              | MiniMax 官方渠道（本文档）                                   | 阿里云百炼渠道（第三方托管）                                                                                                                                             |
| ------------ | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 渠道类型     | `ChannelTypeMiniMax = 35`                                    | `ChannelTypeAli = 17`                                                                                                                                                    |
| 客户端模型名 | `MiniMax-H3`（v2 判定：前缀 `minimax-h3`，大小写不敏感）     | `MiniMax/MiniMax-H3`（判定：模型名含 `minimax` 即路由，[ali/adaptor.go#L354-L359](file:///home/xufan/trea_project/new-api/relay/channel/task/ali/adaptor.go#L354-L359)） |
| 你的上游协议 | **本文档 §1–2**（`/v2/video_generation` + `content[]` 数组） | DashScope 异步协议（`POST /api/v1/services/aigc/video-generation/video-synthesis` + `input.media[]`，见 [ali-bailian-minimax-video.md](./ali-bailian-minimax-video.md)） |

若你的 Provider 后端实现的是百炼 DashScope 协议，请改用渠道类型 17 并参考百炼文档；实现本文档的 v2 协议则用渠道类型 35。

## 2. 查询任务：`GET /v2/query/video_generation/{task_id}`

适配器调用点：[adaptor.go#L127-L148](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor.go#L127-L148)（`FetchTask`）、[adaptor.go#L154-L162](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor.go#L154-L162)（`buildQueryURL`）。

### 2.1 请求

```http
GET {base}/v2/query/video_generation/<task_id>   # task_id 为路径参数
Accept: application/json
Authorization: Bearer <API_KEY>
```

- `task_id` 即你创建任务时返回的 ID。
- 无 query 参数、无请求体。本系统后台轮询**高频调用**（默认数秒一次直至终态），请保证幂等与效率。

### 2.2 响应体（HTTP 200）

来源结构 [models_v2.go#L57-L94](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/models_v2.go#L57-L94)（`V2QueryResponse`/`V2Task`）：

```jsonc
{
  "task": {
    "id": "your-upstream-task-id", // 必填，应与请求的 task_id 一致
    "model": "MiniMax-H3",
    "status": "running", // 必填，见下方枚举
    "created_at": 1756454400, // int64 unix 秒
    "updated_at": 1756454460,
    "content": { "url": "https://cdn.example.com/v/xxx.mp4" }, // 成功时必填
    "resolution": "2K",
    "duration": 5,
    "ratio": "16:9",
    "usage": { "total_seconds": 5.0, "total_tokens": 0 }, // 可选，仅用于日志排障
    "task_type": "generation",
    "modality": "video",
    "error": { "message": "..." }, // 失败时提供，结构不限，优先取 message
  },
}
```

**状态枚举与系统映射**（[adaptor_v2.go#L336-L375](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L336-L375)）：

| 你的 `status` | 含义   | new-api 侧处理                                                                                 |
| ------------- | ------ | ---------------------------------------------------------------------------------------------- |
| `queued`      | 排队中 | 任务进行中（progress 20%）                                                                     |
| `running`     | 处理中 | 任务进行中（progress 50%）                                                                     |
| `succeeded`   | 成功   | **`content.url` 必须为非空直链**，否则按失败处理（reason: `empty content.url in v2 response`） |
| `failed`      | 失败   | 取 `task.error` 作为失败原因；无则报 `task failed`                                             |
| `cancelled`   | 已取消 | 同失败处理（reason: `task cancelled`）                                                         |
| 其他值        | —      | 兜底视为处理中（progress 30%），轮询会继续                                                     |

- `content.url`：**可直接下载的公网视频直链**（mp4）。本系统会把它写入任务结果，客户端通过 `GET /v1/videos/{task_id}/content` 代理下载或直接取 `metadata.url`。若你的直链有时效，请保证有效期足够长（覆盖客户端下载窗口）。
- `error`：结构未固定，适配器优先读 `{"message": "..."}`，取不到时把整段原文透给客户端。
- 顶层必须为 `{"task": {...}}` 信封；若响应无 `task.status` 字段，适配器识别为非 v2 结构并回退 v1 解析（会导致解析异常），**请始终返回该信封**。

## 3. 视频下载（`content.url`）

- 无需专门接口：任务成功后 `content.url` 必须是**无需额外认证的直链**（new-api 及终端客户端直接 GET，部分场景经本系统 `/v1/videos/{id}/content` 反代）。若必须鉴权，请使用长有效期签名 URL。
- 建议返回 `Content-Type: video/mp4`，支持 Range（客户端拖动播放场景）。

## 4. 计费关联（Provider 无需实现，但需知道）

new-api 对该渠道按 `duration × resolution` 计费，与你的请求体字段一一对应（[adaptor_v2.go#L311-L334](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L311-L334)）：

| 键                                  | 来源                                  |
| ----------------------------------- | ------------------------------------- |
| `seconds`                           | 你收到的 `duration`                   |
| `resolution-768P` / `resolution-2K` | 你收到的 `resolution`（2K 按 2 倍计） |

查询响应里的 `usage` **不参与计费**，仅入库排障。因此你的实际用量口径若与请求参数不符（例如实际生成 10s 而请求 5s），请尽量在创建时就拒绝或收敛，不要事后改动。

## 5. 最小实现示例（伪代码）

```python
# POST /v2/video_generation
def create_video(auth, body):
    validate_bearer(auth)                       # Authorization: Bearer
    assert body["model"] == "MiniMax-H3"
    assert body["resolution"] in ("768P", "2K")
    assert 4 <= body["duration"] <= 15
    text_items   = [c for c in body["content"] if c["type"] == "text"]
    assert text_items and text_items[0]["text"].strip()
    task_id = enqueue_generation(body)          # 异步执行
    return 200, {"task_id": task_id}

# GET /v2/query/video_generation/{task_id}
def query_video(auth, task_id):
    t = load_task(task_id)
    if t.state == "done":
        return 200, {"task": {"id": task_id, "status": "succeeded",
                              "content": {"url": t.mp4_public_url}}}
    if t.state == "failed":
        return 200, {"task": {"id": task_id, "status": "failed",
                              "error": {"message": t.reason}}}
    return 200, {"task": {"id": task_id,
                          "status": "queued" if t.pending else "running"}}
```

## 6. 上线前自测清单

- [ ] `POST /v2/video_generation` 返回 200 + 非空 `task_id`
- [ ] 缺 `content.text` / 非法 `resolution` / 越界 `duration` 时返回 4xx + 错误信封
- [ ] `GET /v2/query/video_generation/{task_id}` 依次能返回 `queued → running → succeeded`，且 `content.url` 可直接 GET 下载 mp4
- [ ] `failed` 响应带 `error.message`
- [ ] Bearer 鉴权失败返回 401
- [ ] 任务终态后查询接口持续可用（客户端可能晚些才查询/下载）
