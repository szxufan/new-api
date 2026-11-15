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
// 与后端 dto/audio.go 的 AudioRequest 对应
export interface TTSRequest {
  model: string
  input: string
  voice: string
  response_format?: string
  speed?: number
  instructions?: string
  /** 使用分组；后端 /pg 代理按该字段覆盖会话默认分组 */
  group?: string
}

/** TTS 结果：音频二进制 Blob + 元信息 */
export interface TTSAudioResult {
  blob: Blob
  /** object URL，供 <audio> 播放与下载 */
  url: string
  /** 音频格式（mp3/wav/...），用于下载扩展名 */
  format: string
  /** 音频大小（字节） */
  size: number
}

/** ASR 端点：transcriptions（听写）/ translations（翻译为英文） */
export type AsrEndpoint = 'transcriptions' | 'translations'

/** ASR 响应：json/verbose_json 为 JSON，text/srt/vtt 为纯文本 */
export interface ASRResponse {
  text?: string
  language?: string
  duration?: number
  segments?: unknown[]
  error?: {
    message?: string
    type?: string
    code?: string
  }
  [key: string]: unknown
}

export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}

/** TTS 表单状态 */
export interface TTSFormState {
  model: string
  group: string
  input: string
  voice: string
  /** 语速输入框原始文本（允许空/中间态，提交时再解析并钳制范围） */
  speed: string
  responseFormat: string
  instructions: string
}

/** ASR 音频来源方式 */
export type AsrSource = 'upload' | 'record'

/** ASR 表单状态（不含音频文件） */
export interface ASRFormState {
  model: string
  group: string
  endpoint: AsrEndpoint
  language: string
  prompt: string
  responseFormat: string
}
