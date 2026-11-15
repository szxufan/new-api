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
// 默认值
// ============================================================================

export const DEFAULT_GROUP = 'default'
