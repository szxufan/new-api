# /audio-debug 音频调试

## 功能说明

「音频调试」（`/audio-debug`）用于在浏览器内联调 TTS 与 ASR 两类端点：

- **文本转语音（TTS）**：输入文本、音色、语速、响应格式与可选指令，`POST /pg/audio/speech`，返回音频二进制流，可在页面内播放/下载；
- **语音转文字（ASR）**：上传音频或直接麦克风录音，选择 `transcriptions` / `translations` 端点与 `json|verbose_json|text|srt|vtt` 响应格式，`POST /pg/audio/transcriptions|translations`（multipart）。

页面由侧边栏模块开关 `audiodebug`（分组 `chat`，系统设置 → 维护 → 侧边栏模块）控制是否可见，路由在 `beforeLoad` 中校验。

## 请求与分组

- 端点：`POST /pg/audio/speech`、`POST /pg/audio/transcriptions`、`POST /pg/audio/translations`（`router/relay-router.go` 的 `/pg` 分组，`UserAuth` + `Distribute`）；
- 端点识别：`relay/constant/relay_mode.go` 的 `Path2RelayMode` 同时识别 `/v1/audio/*` 与 `/pg/audio/*`；`controller.Playground` 据此选择 `RelayFormatOpenAIAudio`；
- 路径归一化：`middleware/distributor.go` 的 `normalizeAudioRelayPath` 把 `/pg/audio/x` 归一为 `/v1/audio/x`，与标准端点共用同一套模型/relay mode 判定；
- **分组覆盖**：请求体携带 `group` 字段（TTS 为 JSON 字段，ASR 为表单字段），`Distribute` 校验用户可用分组后覆盖本次请求的分组；该字段属于网关内部字段：
  - TTS：经 `dto.AudioRequest` 反序列化后自然丢弃；
  - ASR：`relay/channel/openai` 重建上游 multipart 时显式跳过 `group`，不会透传上游；
- 分组解析只做一次：`getModelRequest` 解析出的 `ModelRequest.Group` 直接供 `Distribute` 复用，避免 multipart 音频体被重复整包解析。

## 模型与分组候选

- 模型：`GET /api/user/models?endpoint=tts` / `?endpoint=asr`（与用户可用模型取交集）；
- 分组：`GET /api/user/self/groups?endpoint=tts|asr`（仅返回包含支持对应端点模型的分组）；
- 候选变化后若当前选中值失效，前端自动回退到第一个有效选项（`lib/payload.ts` 的 `resolveSelection`）。

## 限制与约定

- 音频文件上限 25MB（上传与录音一致，与 OpenAI 限制对齐）；
- TTS 文本上限 4096 字符；`speed` 取值范围 `[0.25, 4]`（输入框保留文本中间态，提交时解析并钳制，空值/非法值不携带、由上游使用默认 1）；
- `pcm` 为无封装裸流，浏览器无法直接播放，页面提示改为下载；
- 切换 TTS/ASR Tab 或离开页面会取消在飞请求（`AbortController`）并释放音频 object URL。

## 前端文件

- `web/default/src/features/audio-debug/index.tsx`：页面容器、Tab 切换、TTS/ASR 独立提交状态与错误；
- `components/tts-debug-form.tsx` / `tts-result.tsx`、`components/asr-debug-form.tsx` / `asr-result.tsx`、`components/asr-audio-source.tsx`（上传 + `MediaRecorder` 录音）；
- `hooks/use-audio-recorder.ts`：录音 hook，错误以 i18n key 返回；
- `lib/payload.ts`：`buildTTSPayload` / `buildASRFormData` / `resolveSpeed` / `resolveSelection`（纯函数，单测 `lib/payload.test.ts`）；
- `lib/errors.ts`：`AudioRequestError` / `resolveAudioErrorMessage` / `isAudioRequestCanceled`（单测 `lib/errors.test.ts`）；
- `routes/_authenticated/audio-debug/index.tsx`：路由入口与侧边栏开关校验。

## 测试

- `relay/constant/relay_mode_test.go`：`/v1` 与 `/pg` 音频路径的 relay mode 映射；
- `middleware/distributor_audio_test.go`：`normalizeAudioRelayPath`、JSON/multipart 音频的模型与分组解析；
- `relay/channel/openai/audio_test.go`：ASR multipart 重建时不透传内部字段 `group`；
- `web/default/src/features/audio-debug/lib/payload.test.ts`、`lib/errors.test.ts`：语速解析/钳制、表单构建与错误文案解析。
