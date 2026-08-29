# 阿里云百炼 MiniMax-H3 视频生成适配

## 背景

阿里云百炼（Model Studio）除自研万相系列外，还**第三方托管**了 MiniMax 的视频生成模型 `MiniMax/MiniMax-H3`。该接口在协议层面与万相视频生成**完全同构**——同一个提交端点、同一个查询端点、同一套状态枚举，差异仅在请求体字段语义。

因此本次适配**不新增渠道类型、不新增 TaskAdaptor、不新增路由**，只在现有阿里云渠道的视频任务适配器 `relay/channel/task/ali/` 内新增一个请求转换分支与计费档位表。

上游文档：<https://help.aliyun.com/zh/model-studio/minimax-video-generation-api-reference>

客户端使用标准 OpenAI 视频端点，与万相、可灵、豆包等渠道完全一致：

| 操作 | 端点 |
|---|---|
| 提交任务 | `POST /v1/videos` |
| 查询任务 | `GET /v1/videos/{task_id}` |
| 视频代理 | `GET /v1/videos/{task_id}/content` |
| Playground | `POST /pg/videos` / `GET /pg/videos/{task_id}` |

## 复用的协议环节（零改动）

| 环节 | 百炼 MiniMax-H3 | 现有实现 |
|---|---|---|
| 提交 | `POST {base}/api/v1/services/aigc/video-generation/video-synthesis` | `BuildRequestURL` |
| 请求头 | `Authorization: Bearer` + `Content-Type: application/json` + `X-DashScope-Async: enable` | `BuildRequestHeader` |
| 查询 | `GET {base}/api/v1/tasks/{task_id}` | `FetchTask` |
| 提交响应 | `output.task_id` / `output.task_status`，失败为顶层 `code` / `message` | `AliVideoResponse` + `DoResponse` |
| 状态枚举 | `PENDING` / `RUNNING` / `SUCCEEDED` / `FAILED` / `CANCELED` / `UNKNOWN` | `ParseTaskResult` + `convertAliStatus` |
| 结果地址 | `output.video_url`（MP4 直链） | `TaskInfo.Url` |
| OpenAI 转换 | — | `ConvertToOpenAIVideo` |

## 与 MiniMax 官方 v2 协议的差异

MiniMax H3 有两条接入通路，**协议不同，实现互不相干**：

| | 百炼（本文档） | MiniMax 官方 v2 |
|---|---|---|
| 渠道类型 | `ChannelTypeAli = 17` | `ChannelTypeMiniMax = 35` |
| 代码位置 | `relay/channel/task/ali/` | `relay/channel/task/hailuo/`（`adaptor_v2.go`） |
| 创建端点 | `/api/v1/services/aigc/video-generation/video-synthesis` | `/v2/video_generation` |
| 输入结构 | `input.prompt` + `input.media[{type,url}]` | `content[]` 数组，元素含 `type` + `role` |
| 状态枚举 | `PENDING` / `RUNNING` / `SUCCEEDED` / … | `queued` / `running` / `succeeded` / … |
| 查询 | `GET /api/v1/tasks/{task_id}` | `GET /v2/query/video_generation/{task_id}` |
| 结果地址 | `output.video_url` | `task.content.url` |
| 默认分辨率 | `768P` | `2K` |

> 默认分辨率取值不同是有意为之：百炼文档全部示例均为 `768P`，且未指定参数时不应默认落在更贵档位造成意外扣费。

## 素材 type 枚举差异（关键坑）

百炼 MiniMax 与万相3.0 都用 `input.media[{type,url}]`，但**参考素材的 type 名不同**，不可复用 `convertWan3Request`：

| 素材 | 百炼 MiniMax | 万相3.0 |
|---|---|---|
| 首帧 | `first_frame` | `first_frame`（同名） |
| 尾帧 | `last_frame` | `last_frame`（同名） |
| 参考图片 | `image_url` | `reference_image` |
| 参考视频 | `feature` | `reference_video` |
| 参考音频 | `driving_audio` | `reference_audio` |

## 素材映射规则

`input_reference` 在 `ValidateMultipartDirect` 阶段已被并入 `images`，因此映射只依据 `images` 数量与 metadata：

| 输入 | 映射结果 | 生成场景 |
|---|---|---|
| 无图 | `media` 省略 | 文生视频 |
| 1 张图 | `[{first_frame}]` | 图生视频-首帧 |
| 2 张图 | `[{first_frame}, {last_frame}]` | 图生视频-首尾帧 |
| ≥3 张图 | 全部 `image_url` | 多模态参考生视频 |
| `metadata.reference_videos` | `feature` | 多模态参考生视频 |
| `metadata.reference_audios` | `driving_audio` | 多模态参考生视频 |

数量上限（超出按顺序保留靠前者）：首帧 1、尾帧 1、参考图 9、参考视频 3、参考音频 3。

`reference_videos` / `reference_audios` 兼容 `[]string`、`[]any`、单个 `string` 三种写法。

### 互斥降级

百炼规定 `first_frame` / `last_frame` 与 `image_url` / `feature` / `driving_audio` **互斥，不可混用**。

客户端同时传入时，后端**自动降级为多模态参考生视频模式**（剔除首尾帧），并写一条系统日志：

```
ali minimax video: first_frame/last_frame dropped in favor of reference media (model=MiniMax/MiniMax-H3)
```

不返回 400，避免中断请求。该策略与 `hailuo` 渠道 `sanitizeV2Content` 的处理惯例一致。

## 参数自动收敛

客户端传入不满足约束的参数时，后端收敛到合法值而非报错。

### resolution（必填，仅 `768P` / `2K`）

优先级：`metadata.parameters.resolution` → `size` 推导 → 默认 `768P`。

| 输入 `size` | 结果 |
|---|---|
| `2K` / `2k` / `4K` / `1080P` / `1440P` / `2160P` | `2K` |
| `480P` / `512P` / `720P` / `768P` | `768P` |
| 像素尺寸，**短边** ≥ 1080（如 `1920*1080`、`2560x1440`） | `2K` |
| 像素尺寸，短边 < 1080（如 `1280*720`、`832*480`） | `768P` |
| 比例串（含 `:`，如 `16:9`） | 不决定档位，回落默认 |
| 空 / 无法识别 | `768P` |

> 按**短边**而非长边判档：`1280*720` 属于 720P 内容，若按长边判断会被误升到 `2K` 档并多扣费。

### ratio

| 场景 | 规则 |
|---|---|
| 图生视频（含首/尾帧） | 恒为 `adaptive`（文档：由输入图片决定，传其他值会被忽略） |
| 文生视频 | **必填且不得为 `adaptive`**；默认 `16:9` |
| 多模态参考生视频 | 默认 `adaptive`，可显式指定 |

`size` 参与推导：含 `:` 时校验后透传；像素尺寸经 `sizeToMiniMaxRatio` 换算为最接近的官方比例；非法值回落该场景默认值。

官方支持比例：`21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` / `adaptive`（比万相3.0 多 `21:9`）。

### duration

| 输入 | 结果 |
|---|---|
| `<= 0`（含 `-1` 智能时长，MiniMax 不支持） | 默认 `5` |
| `< 4` | `4` |
| `4` ~ `15` | 原值 |
| `> 15` | `15` |

`duration` 为 0 时回落读取 `seconds` 字段（OpenAI 视频 API 用 `seconds` 表达时长）。

### 不输出的字段

MiniMax 不接受万相专有参数，转换时**一律不设置**（依赖 `omitempty` 省略）：
`parameters.size`、`parameters.prompt_extend`、`parameters.audio`、`parameters.seed`、`input.img_url`、`input.first_frame_url`、`input.last_frame_url`、`input.audio_url`、`input.negative_prompt`、`input.template`、`input.video_url`。

`watermark` 保持 `false`（`bool` + `omitempty`，序列化时自动省略，与上游默认值一致）。

`prompt` **不做截断**：超过 7000 字符交由上游报错，避免静默篡改用户输入。

## metadata 逃生通道

`convertToAliRequest` 在分支转换**之后**统一用 `metadata` 覆盖请求体（并禁止改写 `model`，防止计费绕过）。因此客户端可精确指定任意素材组合，绕过自动映射：

```json
{
  "model": "MiniMax/MiniMax-H3",
  "prompt": "尾帧生视频",
  "metadata": {
    "input": {
      "media": [{ "type": "last_frame", "url": "https://example.com/last.png" }]
    },
    "parameters": { "resolution": "2K", "ratio": "16:9", "duration": 10 }
  }
}
```

- `metadata.input.media`：**原样透传，优先级最高**，可用于尾帧单独生视频等自动映射无法表达的組合
- `metadata.parameters.*`：字段级合并，只覆盖出现的键
- `metadata.model`：会被拒绝（`can't change model with metadata`）

## 计费

沿用任务计费三段式，`EstimateBilling` 产出 `OtherRatios`：

| key | 取值 |
|---|---|
| `seconds` | 收敛后的 `duration` |
| `resolution-768P` | `1` |
| `resolution-2K` | `2` |

档位表按 `aliReq.Model` **精确匹配**，因此同时登记 `MiniMax/MiniMax-H3` 与 `MiniMax-H3` 两个 key，覆盖模型映射与中转改写两种情况。

> **倍率为相对档位，需按实际价格校准**：基础模型倍率应按 `768P` 档定价，`2K` 档取其 2 倍。上线前请对照[百炼控制台模型市场](https://bailian.console.aliyun.com/cn-beijing?tab=model#/model-market/all)的实际单价调整 `ProcessAliOtherRatios` 中的数值。

### 顺带修复的既有缺陷

`ProcessAliOtherRatios` 的分辨率归一化原本对所有非 `P` 结尾的值补 `P`，会把 `2K` 变成 `2KP`，导致查表永远落空、档位倍率静默丢失。现已豁免 `K` 结尾（`TestProcessAliOtherRatios_WanUnaffected` 保护万相档位不受影响）。

## 已知限制

1. **参考视频与超量参考图不计费（少收）**：百炼实际计费为 `duration = input_seconds + output_seconds`，且输入图片超过 5 张按超出数量计费（`usage.image_count`）。本实现沿用 ali 渠道既有的 `taskcommon.BaseBilling`（完成时不做差额结算），**仅按输出秒数 × 分辨率档位预估扣费**。若业务需要精确计费，需为 ali TaskAdaptor 实现 `AdjustBillingOnComplete` 并解析 `usage.input_seconds` / `usage.image_count`。
2. **仅华北2（北京）地域**，且模型、Endpoint URL、API Key 必须同属该地域，跨地域调用失败。
3. 模型需先在百炼控制台**开通并授权**（搜索 "MiniMax" → 模型卡片 → 立即开通）。
4. `task_id` 有效期 24 小时，超时后查询返回 `UNKNOWN`，无法再取回结果。
5. 视频链接有效期 30 天，建议及时下载，不作为长期存储依赖。
6. 查询接口默认 RPS 为 20；本项目后台轮询间隔 15 秒且渠道内任务间 sleep 1 秒，正常不会触限。

## 部署配置

由于 `BuildRequestURL` 采用 `{base}/api/v1/...` 直接拼接，**渠道 base URL 必须配成百炼业务空间域名**，不能使用默认的 `https://dashscope.aliyuncs.com`：

```
https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com
```

`{WorkspaceId}` 为百炼业务空间 ID，获取方式见[百炼文档](https://help.aliyun.com/zh/model-studio/obtain-the-app-id-and-workspace-id)。渠道密钥填该地域的 DashScope API Key。

渠道模型列表中登记 `MiniMax/MiniMax-H3`（已加入 `relay/channel/task/ali/constants.go` 的 `ModelList`）。

## 验证方式

文生视频：

```bash
curl -X POST http://localhost:3000/v1/videos \
  -H "Authorization: Bearer $NEW_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "MiniMax/MiniMax-H3",
    "prompt": "史诗级太空歌剧院线预告：女舰长独自站在巨大观景窗前，最后一支舰队正在集结并跃迁离去",
    "size": "16:9",
    "duration": 5
  }'
```

图生视频（首帧）：

```bash
curl -X POST http://localhost:3000/v1/videos \
  -H "Authorization: Bearer $NEW_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "MiniMax/MiniMax-H3",
    "prompt": "让图片中的人物动起来，头发被微风吹动",
    "input_reference": "https://example.com/portrait.webp",
    "duration": 5
  }'
```

查询（返回 OpenAI `Video` 对象，`status` 终态为 `completed` / `failed`）：

```bash
curl http://localhost:3000/v1/videos/task_xxxx \
  -H "Authorization: Bearer $NEW_API_TOKEN"
```

排障时可检查提交响应头 `X-New-Api-Other-Ratios`（应含 `seconds` 与 `resolution-768P` / `resolution-2K`），以及任务详情中的 `PollRecord`（记录每次轮询的原始请求与响应）。

## 测试

`relay/channel/task/ali/adaptor_test.go` 中的 MiniMax 用例：

```bash
go test ./relay/channel/task/ali/... -run MiniMax -v
```

覆盖：模型识别、文生视频/首帧/首尾帧/参考图四种场景映射、互斥降级、metadata 透传优先级、素材数量截断、时长与分辨率与比例三张收敛表、`2K` 档位倍率（回归 `2KP` 缺陷）、万相档位不受影响（回归保护）、序列化后不含万相专有字段、分支路由与模型映射、metadata 改写 model 被拒。
