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
import type { AxiosError } from 'axios'
import { api } from '@/lib/api'
import { API_ENDPOINTS } from './constants'
import { AudioRequestError, isAudioRequestCanceled } from './lib/errors'
import type {
  ASRResponse,
  GroupOption,
  ModelOption,
  TTSRequest,
  TTSAudioResult,
} from './types'

/** 从错误体（JSON 对象或 blob）中提取 {"error":{"message"}} 文案 */
async function extractErrorMessage(error: unknown): Promise<string> {
  const axiosError = error as AxiosError<{ error?: { message?: string } }>
  const data = axiosError.response?.data
  if (data instanceof Blob) {
    // blob 响应（TTS）下的错误体仍是 JSON 文本
    try {
      const parsed = JSON.parse(await data.text()) as {
        error?: { message?: string }
      }
      if (parsed.error?.message) return parsed.error.message
    } catch {
      /* fallthrough */
    }
  } else if (data?.error?.message) {
    return data.error.message
  }
  if (error instanceof Error && error.message) return error.message
  return ''
}

/**
 * 语音合成（TTS）— POST /pg/audio/speech（JSON，响应为音频二进制流）
 * 成功返回音频 Blob 与播放用 object URL；失败抛出携带后端 error.message 的 Error
 * @param signal 用于切换 Tab / 卸载时取消请求
 */
export async function synthesizeSpeech(
  payload: TTSRequest,
  signal?: AbortSignal
): Promise<TTSAudioResult> {
  try {
    const res = await api.post(API_ENDPOINTS.AUDIO_SPEECH, payload, {
      responseType: 'blob',
      skipErrorHandler: true,
      signal,
    } as Record<string, unknown>)
    const blob = res.data as Blob
    // 正常响应 Content-Type 应为 audio/*；若上游以 200 返回 JSON 错误体则按错误处理
    if (blob.type.includes('json')) {
      const parsed = JSON.parse(await blob.text()) as {
        error?: { message?: string }
      }
      throw new AudioRequestError(
        'Unexpected response from server',
        parsed.error?.message
      )
    }
    return {
      blob,
      url: URL.createObjectURL(blob),
      format: payload.response_format || 'mp3',
      size: blob.size,
    }
  } catch (error) {
    if (isAudioRequestCanceled(error)) throw error
    if (error instanceof AudioRequestError) throw error
    throw new AudioRequestError(
      'Request failed',
      (await extractErrorMessage(error)) || undefined
    )
  }
}

/**
 * 语音识别（ASR）— POST /pg/audio/transcriptions|translations（multipart/form-data）
 * 响应可能为 JSON（json/verbose_json）或纯文本（text/srt/vtt），统一返回给调用方解析
 * @param signal 用于切换 Tab / 卸载时取消请求
 */
export async function transcribeAudio(
  path: string,
  formData: FormData,
  signal?: AbortSignal
): Promise<ASRResponse | string> {
  try {
    const res = await api.post(path, formData, {
      skipErrorHandler: true,
      signal,
    } as Record<string, unknown>)
    return res.data as ASRResponse | string
  } catch (error) {
    if (isAudioRequestCanceled(error)) throw error
    throw new AudioRequestError(
      'Request failed',
      (await extractErrorMessage(error)) || undefined
    )
  }
}

/**
 * 获取用户可用模型。
 * @param endpoints 按端点类型过滤（tts / asr）
 */
export async function getUserModels(
  endpoints: string[] = []
): Promise<ModelOption[]> {
  const params: Record<string, string> = {}
  if (endpoints.length > 0) params.endpoint = endpoints.join(',')
  const res = await api.get(API_ENDPOINTS.USER_MODELS, {
    params: Object.keys(params).length > 0 ? params : undefined,
  })
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data.map((model: string) => ({
    label: model,
    value: model,
  }))
}

/**
 * 获取用户分组。
 * @param endpoints 仅返回包含支持对应端点模型的分组
 */
export async function getUserGroups(
  endpoints: string[] = []
): Promise<GroupOption[]> {
  const params =
    endpoints.length > 0 ? { endpoint: endpoints.join(',') } : undefined
  const res = await api.get(API_ENDPOINTS.USER_GROUPS, { params })
  const { data } = res

  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>

  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: info.ratio,
    desc: info.desc,
  }))
}
