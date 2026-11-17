/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { AsrEndpoint } from './types'

// API 端点（走 /pg 专用代理，与图像调试一致）
export const API_ENDPOINTS = {
  AUDIO_SPEECH: '/pg/audio/speech',
  AUDIO_TRANSCRIPTIONS: '/pg/audio/transcriptions',
  AUDIO_TRANSLATIONS: '/pg/audio/translations',
  USER_MODELS: '/api/user/models',
  USER_GROUPS: '/api/user/self/groups',
} as const

/** 按端点类型返回 ASR 请求地址 */
export function getAsrRequestPath(endpoint: AsrEndpoint): string {
  return endpoint === 'translations'
    ? API_ENDPOINTS.AUDIO_TRANSLATIONS
    : API_ENDPOINTS.AUDIO_TRANSCRIPTIONS
}

// ============================================================================
// 模型/分组过滤（与后端 constant/endpoint_type.go 的 EndpointType 对应）
// ============================================================================

/** TTS → tts 端点；ASR（transcriptions/translations）→ asr 端点 */
export const TTS_MODEL_ENDPOINTS = ['tts'] as const
export const ASR_MODEL_ENDPOINTS = ['asr'] as const

// ============================================================================
// TTS 常量
// ============================================================================

/** 常用 OpenAI 音色；输入框可填写任意自定义音色（火山/阿里等渠道各自定义） */
export const TTS_VOICES = [
  'alloy',
  'ash',
  'ballad',
  'coral',
  'echo',
  'fable',
  'nova',
  'onyx',
  'sage',
  'shimmer',
  'verse',
] as const

/** TTS 音频格式（label 为 i18n 键，与英文源字符串一致） */
export const TTS_FORMATS = [
  { label: 'MP3', value: 'mp3' },
  { label: 'OPUS', value: 'opus' },
  { label: 'AAC', value: 'aac' },
  { label: 'FLAC', value: 'flac' },
  { label: 'WAV', value: 'wav' },
  { label: 'PCM', value: 'pcm' },
] as const

export const DEFAULT_TTS_VOICE = 'alloy'
export const DEFAULT_TTS_FORMAT = 'mp3'
/** 语速输入框默认值（字符串，便于保留用户输入的中间态） */
export const DEFAULT_TTS_SPEED = '1'
export const TTS_SPEED_MIN = 0.25
export const TTS_SPEED_MAX = 4
export const TTS_SPEED_STEP = 0.05
/** OpenAI TTS 单次合成的文本长度上限 */
export const TTS_INPUT_MAX_LENGTH = 4096

/** pcm 为无封装裸流，浏览器无法直接播放 */
export function isPlayableTtsFormat(format: string): boolean {
  return format !== 'pcm'
}

// ============================================================================
// ASR 常量
// ============================================================================

/** ASR 端点选项：transcriptions（听写原文）/ translations（翻译为英文） */
export const ASR_ENDPOINTS = [
  { label: 'Transcriptions', value: 'transcriptions' },
  { label: 'Translations', value: 'translations' },
] as const

/** ASR 响应格式（json/verbose_json 为 JSON；text/srt/vtt 为纯文本） */
export const ASR_FORMATS = [
  { label: 'JSON', value: 'json' },
  { label: 'Verbose JSON', value: 'verbose_json' },
  { label: 'Text', value: 'text' },
  { label: 'SRT', value: 'srt' },
  { label: 'VTT', value: 'vtt' },
] as const

export const DEFAULT_ASR_FORMAT = 'json'

/** 常见音频上传格式提示（OpenAI whisper 支持的输入格式） */
export const ASR_ACCEPT =
  'audio/*,.mp3,.mp4,.mpeg,.mpga,.m4a,.wav,.webm,.ogg,.opus,.flac,.aac'

/** 录音与上传的音频大小上限（25MB，与 OpenAI 限制一致） */
export const ASR_FILE_MAX_SIZE = 25 * 1024 * 1024

// ============================================================================
// 模型联动（DeepSeek 虚拟模型的能力限制，与 relay/channel/deepseek 适配器一致）
// ============================================================================

/** DeepSeek TTS 官方音色（上游仅支持这两个） */
export const DEEPSEEK_TTS_VOICES = ['stella', 'tide'] as const

export const DEFAULT_DEEPSEEK_TTS_VOICE = 'stella'
export const DEFAULT_DEEPSEEK_TTS_FORMAT = 'wav'

/** 是否为 DeepSeek TTS 虚拟模型（仅支持 stella/tide 音色与 wav/pcm 格式） */
export function isDeepSeekTTSModel(model: string): boolean {
  const name = model.trim().toLowerCase()
  return name.startsWith('deepseek') && name.endsWith('-tts')
}

/** 是否为 DeepSeek ASR 虚拟模型（仅支持 transcriptions 端点、json 格式与 wav/ogg/opus 音频） */
export function isDeepSeekASRModel(model: string): boolean {
  const name = model.trim().toLowerCase()
  return name.startsWith('deepseek') && name.endsWith('-asr')
}

/** 按模型返回音色建议列表 */
export function getTTSVoiceOptions(model: string): string[] {
  return isDeepSeekTTSModel(model) ? [...DEEPSEEK_TTS_VOICES] : [...TTS_VOICES]
}

/** 按模型返回默认音色 */
export function getDefaultTTSVoice(model: string): string {
  return isDeepSeekTTSModel(model)
    ? DEFAULT_DEEPSEEK_TTS_VOICE
    : DEFAULT_TTS_VOICE
}

/** 音色对模型是否有效（DeepSeek 限定官方音色；其它模型允许任意自定义音色） */
export function isValidTTSVoice(model: string, voice: string): boolean {
  if (!isDeepSeekTTSModel(model)) return voice.trim().length > 0
  return (DEEPSEEK_TTS_VOICES as readonly string[]).includes(
    voice.trim().toLowerCase()
  )
}

/** 按模型返回可选音频格式 */
export function getTTSFormatOptions(model: string): {
  label: string
  value: string
}[] {
  return isDeepSeekTTSModel(model)
    ? TTS_FORMATS.filter((format) => format.value === 'wav' || format.value === 'pcm')
    : [...TTS_FORMATS]
}

/** 按模型返回默认音频格式 */
export function getDefaultTTSFormat(model: string): string {
  return isDeepSeekTTSModel(model)
    ? DEFAULT_DEEPSEEK_TTS_FORMAT
    : DEFAULT_TTS_FORMAT
}

/** 音色/格式按模型归一：无效值回退到模型默认值（所见即所发，渲染与提交共用） */
export function resolveTTSVoiceForModel(model: string, voice: string): string {
  return isValidTTSVoice(model, voice) ? voice : getDefaultTTSVoice(model)
}

export function resolveTTSFormatForModel(
  model: string,
  format: string
): string {
  return getTTSFormatOptions(model).some((item) => item.value === format)
    ? format
    : getDefaultTTSFormat(model)
}

/** 按模型返回可选 ASR 端点（DeepSeek 无翻译语义，仅支持 transcriptions） */
export function getASREndpointOptions(model: string): {
  label: string
  value: string
}[] {
  return isDeepSeekASRModel(model)
    ? ASR_ENDPOINTS.filter((endpoint) => endpoint.value === 'transcriptions')
    : [...ASR_ENDPOINTS]
}

/** 按模型返回可选响应格式（DeepSeek 仅支持 json） */
export function getASRFormatOptions(model: string): {
  label: string
  value: string
}[] {
  return isDeepSeekASRModel(model)
    ? ASR_FORMATS.filter((format) => format.value === 'json')
    : [...ASR_FORMATS]
}

/** 端点/格式按模型归一：无效值回退到第一个有效选项 */
export function resolveASREndpointForModel(
  model: string,
  endpoint: string
): string {
  const options = getASREndpointOptions(model)
  return options.some((option) => option.value === endpoint)
    ? endpoint
    : (options[0]?.value ?? endpoint)
}

export function resolveASRFormatForModel(
  model: string,
  format: string
): string {
  const options = getASRFormatOptions(model)
  return options.some((option) => option.value === format)
    ? format
    : (options[0]?.value ?? format)
}

/** 按模型返回音频文件选择器的 accept 提示（DeepSeek 仅支持 wav/ogg/opus） */
export function getASRAcceptForModel(model: string): string {
  return isDeepSeekASRModel(model) ? '.wav,.ogg,.opus' : ASR_ACCEPT
}

// ============================================================================
// 默认值
// ============================================================================

export const DEFAULT_GROUP = 'default'
