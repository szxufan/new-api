# 阿里云百炼 Qwen-Omni-Realtime 实时语音适配

## 背景

阿里云百炼的 Qwen-Omni-Realtime（`qwen3.8-omni-flash-realtime` 等）通过 WebSocket 提供全双工实时语音/视频对话。其事件协议与 OpenAI Realtime 基本同构（`session.update`、`input_audio_buffer.*`、`response.*`、`response.done` 中的 usage 字段名完全一致），差异集中在：

| 差异点 | OpenAI Realtime | 百炼 Realtime | 网关适配 |
|---|---|---|---|
| 接入地址 | `wss://{host}/v1/realtime?model=` | `wss://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime?model=` | `buildAliRealtimeURL` |
| 鉴权 | `Authorization: Bearer` 或 Sec-WebSocket-Protocol | `Authorization: Bearer`（DashScope API Key） | `SetupRequestHeader` |
| 音频格式 | 平铺 `input_audio_format: "pcm16"` | 嵌套 `audio.input.format: {type:"pcm", sample_rate, ...}` | `adaptClientSessionUpdate` |
| 最大输出 token | `max_response_output_tokens`（数值或 `"inf"`） | `max_tokens`（整数） | `adaptClientSessionUpdate` |
| 输入转写模型 | `whisper-1` / `gpt-4o-transcribe` | `qwen3-asr-flash-realtime` | `adaptClientSessionUpdate` |
| 单次响应覆盖 | `response.create.response`（instructions/modalities/tools 等） | 仅接受 `type` / `event_id` | `adaptClientResponseCreate` |
| 图片输入 | `conversation.item.create` 内 `input_image` | 独立事件 `input_image_buffer.append`（仅 base64） | `adaptClientConversationItemCreate` |
| 会话回显 | 平铺音频格式字段 | 嵌套 audio 字段 | `adaptUpstreamSessionEcho` |

因此本适配**不新增渠道类型、不新增路由**：复用现有 `/v1/realtime` WebSocket 中继链路，在阿里渠道适配器内新增 realtime 分支，并把 OpenAI 处理器中的双向泵抽取为可注入转换钩子的 `RealtimePump`。

上游文档：<https://docs.bailian.console.aliyun.com/zh/model-studio/realtime>

## 代码结构

| 文件 | 职责 |
|---|---|
| `relay/channel/openai/realtime_pump.go` | `RealtimePump`：双向事件泵 + 计数/计费（上游 usage 优先），支持注入 `RealtimeEventTransformer` |
| `relay/channel/openai/relay-openai.go` | `OpenaiRealtimeHandler` = `RealtimePump(c, info, nil, nil)` 薄包装，OpenAI 渠道行为不变 |
| `relay/channel/ali/adaptor.go` | `RelayModeRealtime` 分支：URL 构造、握手头、`DoWssRequest`、`AliRealtimeHandler` |
| `relay/channel/ali/realtime.go` | 字段适配（双 schema 兼容：检测到百炼原生嵌套 `audio` 字段时不再转换） |
| `types/relay_format_mapping.go` | realtime 首选渠道类型增加 `ChannelTypeAli` |

## 渠道配置

渠道 base URL **必须配成百炼业务空间域名**（与 MiniMax-H3 视频任务同一约定），不能使用默认的 `https://dashscope.aliyuncs.com`：

```
https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com     # 北京地域
https://{WorkspaceId}.ap-southeast-1.maas.aliyuncs.com # 新加坡地域
```

`{WorkspaceId}` 为百炼业务空间 ID，获取方式见[百炼文档](https://help.aliyun.com/zh/model-studio/obtain-the-app-id-and-workspace-id)。渠道密钥填对应地域的 DashScope API Key。

渠道模型列表登记实时模型（已加入 `relay/channel/ali/constants.go` 的 `ModelList`）：

```
qwen3.8-omni-flash-realtime
qwen3.5-omni-plus-realtime
qwen3.5-omni-flash-realtime
```

若运营者在 base_url 中自带路径（如历史 `api-ws/v1/inference` 端点），网关会保留该路径，仅补 `?model=` 参数。

## 客户端使用

按 OpenAI Realtime 协议连接网关：

```bash
wss://your-gateway/v1/realtime?model=qwen3.8-omni-flash-realtime
# 鉴权二选一：Authorization: Bearer sk-xxx，或
# Sec-WebSocket-Protocol: realtime, openai-insecure-api-key.sk-xxx
```

事件流示例（Manual 模式）：`session.update`（可用 OpenAI 平铺字段，网关自动转换）→ `input_audio_buffer.append`（16kHz/24kHz 16-bit PCM，Base64）→ `input_audio_buffer.commit` → `response.create` → 接收 `response.audio.delta` / `response.audio_transcript.delta` / `response.done`。

计费说明：每次 `response.done` 携带的 `usage`（`input_tokens` / `output_tokens` 及 `input_token_details.audio_tokens` 等）为权威用量，网关按其扣费；文本/音频 token 分别按文本倍率、音频倍率（`音频倍率`×`音频补全倍率`）计算。

## 管理员须知

**新模型无默认倍率**，未配置时会落到兜底倍率（模型倍率 37.5、音频倍率 1），对实时模型会严重高估成本。请在运营设置中为每个 realtime 模型配置：

| 参数 | 含义 | 参考锚点（qwen3-omni-flash-realtime 国内价） |
|---|---|---|
| 模型倍率 | 文本输入单价基准（倍率 × $0.002/1K） | 输入文本 2.2 元/M tokens |
| 补全倍率 | 输出文本 ÷ 输入文本 | 输出文本 8.3 元/M → ≈3.8 |
| 音频倍率 | 输入音频 ÷ 输入文本 | 输入音频 18.9 元/M → ≈8.6 |
| 音频补全倍率 | 输出音频 ÷（输入音频 × 音频倍率） | 输出音频（多模态）15.2 元/M → ≈0.8 |

> 输入音频 token 换算：秒数 × 7；输出音频：秒数 × 12.5。qwen3.8/3.5 实时版的最新单价以[百炼定价页](https://help.aliyun.com/zh/model-studio/model-pricing)为准。

## 已知限制

- 仅支持 WebSocket 中继；百炼的 WebRTC / AOQ 接入不经过网关（其信令与媒体面无法代理）。
- 音频字节层透传：客户端按 OpenAI 协议发送 24kHz 16-bit PCM（网关将 `pcm16` 映射为百炼 `audio.input.format: {type:"pcm", sample_rate:24000}`）；`g711_*` 等其他 OpenAI 音频格式不做转换，原样透传由上游报错。
  - **采样率注意**：百炼文档中 `qwen3.5` 系列输入可选 8000/16000/24000/48000 Hz，而 `qwen3.8-omni-flash-realtime` 的 `session.created` 说明输入要求 16 kHz（多通道输入只能 16000 Hz）。网关不做重采样，固定按 OpenAI 语义声明 24000 Hz；若上游因此报采样率错误，请让客户端改用 `qwen3.5` 实时模型，或由客户端直接下发百炼原生 `session.audio.input.format`（此时网关不再改写，按客户端声明的采样率透传，客户端需自行保证音频字节与声明一致）。
- 图片输入仅支持 base64 data URI（转为 `input_image_buffer.append`）；http(s) URL 图片不做服务端取图，该部分被丢弃并记录告警。图片需 JPG/JPEG、Base64 后 ≤256KB、建议 1 帧/秒。
- `response.create` 的 `response` 覆盖对象（instructions、modalities、tools、max_output_tokens 等）会被丢弃并记录告警，百炼仅接受 `type` / `event_id`，本次响应沿用 `session.update` 的会话级配置。
- `session.update` 中 `max_response_output_tokens: "inf"`（不限制）会被丢弃，改用百炼默认上限；`voice` 需使用百炼音色（如 `Tina`/`Cherry`），OpenAI 音色名会由上游校验拒绝。
- 适配仅覆盖字段级差异，不做事件语义转换；百炼扩展字段（`enable_search`、`video`、多通道音频等）由客户端直接下发。
- 渠道"测试"功能不支持 realtime 模型（WebSocket 长连接无法走单次测试请求）。

## 验证方式

1. 单元测试：`go test ./relay/channel/ali/... ./types/... ./relay/channel/openai/...`
2. 端到端（需真实业务空间）：
   - 创建阿里渠道，base_url 填业务空间域名，密钥为 DashScope API Key，模型列表添加 `qwen3.8-omni-flash-realtime`。
   - 用 WebSocket 客户端连 `wss://网关/v1/realtime?model=qwen3.8-omni-flash-realtime`，发送 `session.update`（含 `input_audio_format: "pcm16"`）、PCM 音频与 `response.create`。
   - 预期：收到 `session.created`（客户端侧可见平铺 `input_audio_format: "pcm16"`）、音频/文本响应；网关日志显示上游地址为 `wss://…maas.aliyuncs.com/api-ws/v1/realtime`，`response.done` 后按上游 usage 生成计费日志。
