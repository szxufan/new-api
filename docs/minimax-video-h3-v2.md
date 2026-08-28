# MiniMax-H3 视频生成 v2 接口适配

## 背景

MiniMax 的视频生成有**两套互不兼容的协议**，但都挂在同一个渠道类型上（`ChannelTypeMiniMax = 35` → `hailuo.TaskAdaptor`）：

| | v1 | v2 |
|---|---|---|
| 代表模型 | `MiniMax-Hailuo-2.3`、`Hailuo-02`、`T2V-01*`、`I2V-01*`、`S2V-01` | `MiniMax-H3` |
| 文档 | [video-generation-i2v](https://platform.minimaxi.com/docs/api-reference/video-generation-i2v) | [v2-create](https://platform.minimaxi.com/docs/api-reference/video-generation-v2-create) / [v2-query](https://platform.minimaxi.com/docs/api-reference/video-generation-v2-query) |

本次适配在 `relay/channel/task/hailuo/` 内新增 v2 协议，v1 行为保持不变。客户端仍使用标准 OpenAI 视频端点：`POST /v1/videos` 提交、`GET /v1/videos/{task_id}` 查询（playground 对应 `/pg/videos`）。

## 协议差异

| 环节 | v1 | v2（H3） |
|---|---|---|
| 创建 | `POST {base}/v1/video_generation` | `POST {base}/v2/video_generation` |
| 输入 | 扁平 `prompt` / `first_frame_image` / `last_frame_image` / `subject_reference` | `content[]` 多模态数组，元素含 `type` + `role` |
| 分辨率 | `512P` / `720P` / `768P` / `1080P` | 仅 `768P` / `2K`（必填） |
| 时长 | 6 / 10 秒 | `4`~`15` 整数秒（必填） |
| 宽高比 | 无该字段 | `ratio`：`adaptive` / `21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` |
| 创建响应 | `{"task_id","base_resp":{...}}`，`status_code==0` 为成功 | `{"task_id"}`；失败为 `{"type":"error","error":{...},"request_id"}` + HTTP 状态码 |
| 查询 | `GET {base}/v1/query/video_generation?task_id=X` | `GET {base}/v2/query/video_generation/{task_id}`（路径参数） |
| 查询响应 | `{"task_id","status","file_id","base_resp"}` | `{"task":{"id","status","content":{"url"},"usage",...}}` |
| 状态枚举 | `Preparing` / `Queueing` / `Processing` / `Success` / `Fail` | `queued` / `running` / `succeeded` / `failed` / `cancelled` |
| 结果地址 | 需再用 `file_id` 请求 `GET {base}/v1/files/retrieve` 换 `download_url` | `task.content.url` 即直链，无需二次请求 |

## 协议分流机制

`GetTaskAdaptor` 只按渠道类型号分发，同一渠道无法注册两个 `TaskAdaptor`，因此 v1/v2 在 `hailuo.TaskAdaptor` 内部按协议分流：

- **提交期**：以 `info.UpstreamModelName` 判定（`IsVideoV2Model`，前缀 `minimax-h3` 大小写不敏感），影响 `BuildRequestURL` / `BuildRequestBody` / `DoResponse`。
- **轮询期**：后台轮询调用 `FetchTask(baseUrl, key, body, proxy)` 时，`body` 只有 `task_id` 与 `action` 两个键，**拿不到模型名**（`service/task_polling.go` 的 `updateVideoSingleTask`）。因此协议版本必须随任务落库：`EstimateBilling` 把 `info.Action` 改写为 `constant.TaskActionVideoV2Generate`（`"videoV2Generate"`），该值经 `controller/relay.go` 写入 `task.Action`，轮询时回传给 `FetchTask` 选择查询端点。
  - 之所以放在 `EstimateBilling`：它是提交路径上**最早能拿到 `UpstreamModelName` 且必然被调用**的适配器钩子（`ValidateRequestAndSetAction` 执行时模型名尚未解析）。
- **兜底**：`ParseTaskResult` 额外按响应结构自适应识别（顶层 `task.status` 非空即视为 v2），因此即使 `action` 判据失效（历史任务、remix 路径覆写 action），结果解析仍然正确。

## 参数自动收敛

客户端传入不满足 v2 约束的参数时，后端自动收敛到合法值，而非直接报错。

### resolution

| 输入 | 结果 |
|---|---|
| `2K` / `2k` / `1440` / `2048` / `2560` | `2K` |
| `1080P` 及以上 | `2K`（向上取，避免降低画质） |
| `768P` / `720P` / `512P` / `480P` | `768P` |
| 空 / 无法识别 | 默认 `2K` |

取值优先级：`metadata.resolution` → `metadata.parameters.resolution`（兼容 `/image-debug` 视频调试台的嵌套写法）→ `size` → 默认值。

### duration

| 输入 | 结果 |
|---|---|
| `<= 0`（含调试台的 `-1` 智能时长） | 默认 `5` |
| `< 4` | `4` |
| `> 15` | `15` |
| `4`~`15` | 原值 |

`metadata.duration` 优先于顶层 `duration`，支持数字与数字字符串。

### ratio

先取候选值（`metadata.ratio` → `size`），再按生成场景收敛：

| 场景 | 判定依据 | 规则 |
|---|---|---|
| 图生视频 i2va | content 含 `first_frame` / `last_frame` | 恒为 `adaptive`（宽高比由输入图片决定） |
| 多模态参考 r2va | content 含 `reference_*` | 候选在合法列表内则用，否则 `adaptive` |
| 文生视频 t2va | 仅 `text` | 候选在合法列表内则用，否则 `16:9`（`adaptive` 也回退 `16:9`） |

### content 组装

未显式提供 `metadata.content` 时，按以下规则自动映射：

| 输入 | 生成的 content 元素 |
|---|---|
| `prompt` | `{"type":"text","text":...}`（超过 7000 字符按字符截断） |
| 1 张图（`images` / `image` / `input_reference`） | + `image_url` `role=first_frame` |
| 2 张图 | + `image_url` `role=first_frame` 与 `role=last_frame` |
| ≥3 张图 | + 全部 `image_url` `role=reference_image` |
| `metadata.first_frame_image` / `last_frame_image` | 显式指定首/尾帧，此时 `images` 并入参考图 |
| `metadata.reference_images` / `reference_videos` / `reference_audios` | 对应 `role=reference_image` / `reference_video` / `reference_audio` |

收敛规则：

- **互斥**：文档规定 `first_frame`/`last_frame` 与 `reference_*` 不可混用。若同时出现，把首/尾帧**降级为 `reference_image`**（保留用户全部素材，不静默丢弃）。
- **数量上限**：首帧 ≤1、尾帧 ≤1、参考图 ≤9、参考视频 ≤3、参考音频 ≤3，媒体总数 ≤12；超出部分按顺序丢弃靠后的。
- **必填校验**：收敛后若无非空 `text` 项，返回错误（上游同样会 400，提前失败更清晰）。
- **逃生通道**：`metadata.content` 给出合法数组时**原样透传**（仍做上述数量/空值收敛），便于上游新增字段时无需改代码。
- `metadata` 中的 `model` 会被剔除（复用 `taskcommon.UnmarshalMetadata`），避免改写模型名绕过计费；metadata 覆盖后仍会**再跑一遍全部收敛**，客户端无法把非法值送到上游。

## 计费

`EstimateBilling` 覆写 `taskcommon.BaseBilling` 的 no-op，返回 OtherRatios（会连乘进预扣额度，见 `relay/relay_task.go`）：

| 键 | 值 |
|---|---|
| `seconds` | 收敛后的 `duration` |
| `resolution-2K` | `2.0` |
| `resolution-768P` | `1.0` |

- 分辨率倍率为内置占位值，管理员可在「系统设置 → 计费 → 模型定价」通过 `MiniMax-H3` 基础价整体调整。
- `AdjustBillingOnSubmit` / `AdjustBillingOnComplete` 未覆写：`duration` 与 `resolution` 在提交时即最终确定，上游不会改变，无需差额结算。
- 上游 `usage`（`total_seconds` / `total_tokens` 等）**不参与计费**，仅随 `task.Data` 保存用于排障。若把 `TaskInfo.TotalTokens` 填上，轮询终态会改走按 token 重算（`service/task_polling.go` 的 `settleTaskBillingOnComplete`），与本次的按次口径冲突。

## 与 OpenAI 视频接口的对应关系

- 提交成功：返回 `dto.OpenAIVideo`（`id` / `task_id` 为平台公开的 `task_xxxx`，`status: "queued"`），v1/v2 一致。
- 查询 `GET /v1/videos/{task_id}`：由 `ConvertToOpenAIVideo` 生成，**状态与视频地址取数据库实时字段**（`task.Status` / `task.GetResultURL()`），不再解析 `task.Data` 中的 `base_resp`。
  - 原因：`task.Data` 在提交阶段是创建响应原文、轮询后才被覆盖为查询响应原文；且 v2 创建响应无 `base_resp`，按 v1 结构解析会把缺省的 `status_code=0` 误判为成功。
  - 失败时错误信息优先级：`task.FailReason` → v2 `task.error` 原文 → v1 `base_resp.status_msg` → 兜底文案。
  - 该改动同时使 v1 不再依赖 `Data` 结构，修掉「提交后立即查询被误判」的一类问题。

## 渠道配置要求

1. **BaseURL**：官方 v2 端点为 `https://api.minimaxi.com`（国内）或 `https://api.minimax.io`（国际）。渠道默认 BaseURL 是 `https://api.minimax.chat`，启用 H3 前请在渠道设置中填写上述域名之一，并实测确认。
2. **模型**：`MiniMax-H3` 已加入 `hailuo.ModelList`；渠道模型列表与 `models` 表仍需按常规方式配置。
3. **定价**：`MiniMax-H3` 需在「系统设置 → 计费 → 模型定价」配置基础价，否则按默认倍率计费。
4. **视频调试台**：`/image-debug` 的视频 Tab 只列出支持 `openai-video` 端点的模型，需为该模型配置自定义端点才会出现；其默认参数（`1080P`、`adaptive`、时长 2-30）会由后端按上表收敛，无需前端改动。

## 调用示例

文生视频：

```json
POST /v1/videos
{ "model": "MiniMax-H3", "prompt": "史诗级太空歌剧院预告：女舰长独自站在巨大观景窗前", "duration": 5, "size": "16:9" }
```

图生视频（首帧）：

```json
POST /v1/videos
{ "model": "MiniMax-H3", "prompt": "画中人在跳现代舞", "images": ["https://example.com/a.png"], "duration": 5 }
```

首尾帧：

```json
POST /v1/videos
{ "model": "MiniMax-H3", "prompt": "小女孩长大了", "images": ["https://example.com/a.png", "https://example.com/b.png"] }
```

多模态参考（图 + 视频 + 音频）：

```json
POST /v1/videos
{
  "model": "MiniMax-H3",
  "prompt": "角色跟随参考视频的动作跳舞，外观参考两张图",
  "metadata": {
    "resolution": "2K",
    "duration": 8,
    "ratio": "16:9",
    "reference_images": ["https://example.com/1.png", "https://example.com/2.png"],
    "reference_videos": ["https://example.com/motion.mp4"],
    "reference_audios": ["https://example.com/voice.wav"]
  }
}
```

完全自定义 content（逃生通道）：

```json
POST /v1/videos
{
  "model": "MiniMax-H3",
  "prompt": "会被 metadata.content 覆盖",
  "metadata": {
    "content": [
      { "type": "text", "text": "自定义提示词" },
      { "type": "image_url", "image_url": { "url": "https://example.com/a.png" }, "role": "first_frame" }
    ]
  }
}
```

查询任务：`GET /v1/videos/{task_id}`，成功后视频直链位于 `metadata.url`。

媒体素材建议用公网 URL；文档提示请求体总大小 ≤64 MB 且「大文件请用公网 URL，勿用 Base64」，适配器不做 Base64 与 URL 的转换，由客户端负责。

## 未接入的 v2 接口

以下 v2 端点本次未实现：

- `video-generation-v2-list`：列出近 7 天任务；
- `video-generation-v2-delete`：取消排队中任务 / 删除任务记录；
- `video-generation-v2-regeneration`：768P 结果再生成为 2K（需 `role=base_video` 的源视频项）；
- `video-generation-v2-h3-context-ir`：多模态上下文解读，返回增强提示词（`content.prompt`）。

## 涉及文件

| 文件 | 变更 |
|---|---|
| `constant/task.go` | 新增 `TaskActionVideoV2Generate` |
| `relay/channel/task/hailuo/constants.go` | `ModelList` 追加 `MiniMax-H3`；v2 端点/状态/规格/角色常量与 `IsVideoV2Model` |
| `relay/channel/task/hailuo/models_v2.go` | v2 请求与响应 DTO |
| `relay/channel/task/hailuo/adaptor_v2.go` | content 组装、参数收敛、`EstimateBilling`、`parseV2TaskResult`、`doV2Response` |
| `relay/channel/task/hailuo/adaptor.go` | `BuildRequestURL` / `BuildRequestBody` / `DoResponse` / `FetchTask` / `ParseTaskResult` 增加 v2 分支；`ConvertToOpenAIVideo` 改读数据库实时字段 |
| `relay/channel/task/hailuo/adaptor_v2_test.go` | 单元测试 |

## 测试

```bash
go test ./relay/channel/task/hailuo/... -count=1
```

覆盖：协议判定、三类参数收敛、content 各场景组装与互斥降级、数量截断、提示词截断、metadata 透传与复收敛、metadata 无法改写模型、v2 各状态解析与 v1 回退、查询端点选择、创建响应成功与错误信封、计费 ratios 与 action 标记、OpenAI 视频转换的三种状态。
