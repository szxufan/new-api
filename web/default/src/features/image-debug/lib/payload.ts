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
import type { ImageDebugFormState, ImageRequest } from '../types'

/** 解析高级参数 JSON 文本；为空或非法 JSON 时返回 undefined */
export function parseExtraParameters(
  raw: string
): Record<string, unknown> | undefined {
  const trimmed = raw.trim()
  if (!trimmed) return undefined
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
    return undefined
  } catch {
    return undefined
  }
}

/**
 * 构建文生图 JSON 请求体。
 * size/n/quality/style/response_format/watermark 仅在非空/非默认时携带；
 * extra.parameters 仅在高级参数 JSON 合法时透传。
 */
export function buildImagePayload(
  state: ImageDebugFormState
): ImageRequest {
  const payload: ImageRequest = {
    model: state.model,
    prompt: state.prompt,
  }

  if (state.size) payload.size = state.size
  if (state.n > 0) payload.n = state.n
  if (state.quality) payload.quality = state.quality
  if (state.style) payload.style = state.style
  if (state.responseFormat) payload.response_format = state.responseFormat
  if (state.watermark) payload.watermark = state.watermark

  const extraParameters = parseExtraParameters(state.extraParameters)
  if (extraParameters) {
    payload.extra = { parameters: extraParameters }
  }

  return payload
}

/**
 * 构建图生图 multipart 请求体。
 * 返回 FormData：image 文件字段 + model/prompt/n/quality/size 等文本字段。
 * 未选择图片时返回 null。
 */
export function buildEditFormData(
  state: ImageDebugFormState,
  imageFiles: File[]
): FormData | null {
  if (imageFiles.length === 0) return null
  const formData = new FormData()
  for (const file of imageFiles) {
    formData.append('image', file)
  }
  formData.append('model', state.model)
  formData.append('prompt', state.prompt)
  if (state.size) formData.append('size', state.size)
  if (state.n > 0) formData.append('n', String(state.n))
  if (state.quality) formData.append('quality', state.quality)
  if (state.style) formData.append('style', state.style)
  if (state.responseFormat) formData.append('response_format', state.responseFormat)
  if (state.watermark) formData.append('watermark', 'true')
  return formData
}