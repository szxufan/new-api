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
import { TTS_SPEED_MAX, TTS_SPEED_MIN } from '../constants'
import type { ASRFormState, TTSFormState, TTSRequest } from '../types'

/**
 * 解析语速输入框文本：空值/非法值返回 undefined（由上游使用默认 1），
 * 合法值钳制到 OpenAI 支持的 [0.25, 4] 区间。
 */
export function resolveSpeed(raw: string): number | undefined {
  const trimmed = raw.trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  if (!Number.isFinite(parsed)) return undefined
  return Math.min(TTS_SPEED_MAX, Math.max(TTS_SPEED_MIN, parsed))
}

/**
 * 构建 TTS JSON 请求体。
 * voice/input/model 必填；speed 仅在解析后非默认值时携带；
 * group 字段由后端 /pg 代理读取后剔除，不会透传给上游。
 */
export function buildTTSPayload(state: TTSFormState): TTSRequest {
  const payload: TTSRequest = {
    model: state.model,
    input: state.input,
    voice: state.voice,
  }

  if (state.responseFormat) payload.response_format = state.responseFormat
  const speed = resolveSpeed(state.speed)
  if (speed !== undefined && speed !== 1) {
    payload.speed = speed
  }
  if (state.instructions.trim())
    payload.instructions = state.instructions.trim()
  if (state.group) payload.group = state.group

  return payload
}

/**
 * 构建 ASR multipart 请求体。
 * file 文件字段 + model/language/prompt/response_format 文本字段；
 * group 字段由后端 /pg 代理读取后剔除，不会透传给上游。
 * 无音频文件时返回 null。
 */
export function buildASRFormData(
  state: ASRFormState,
  file: File | null
): FormData | null {
  if (!file) return null
  const formData = new FormData()
  formData.append('file', file)
  formData.append('model', state.model)
  if (state.language.trim()) formData.append('language', state.language.trim())
  if (state.prompt.trim()) formData.append('prompt', state.prompt.trim())
  if (state.responseFormat) {
    formData.append('response_format', state.responseFormat)
  }
  if (state.group) formData.append('group', state.group)
  return formData
}

/** 当前选中值不在选项中时回退到第一个选项；选项为空时返回 fallback */
export function resolveSelection(
  selected: string,
  options: { value: string }[],
  fallback: string
): string {
  if (options.some((option) => option.value === selected)) return selected
  return options[0]?.value ?? fallback
}
