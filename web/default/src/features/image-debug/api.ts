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
import { api } from '@/lib/api'
import { API_ENDPOINTS } from './constants'
import type {
  GroupOption,
  ImageRequest,
  ImageResponse,
  ModelOption,
  OptimizePromptRequest,
  OptimizePromptResponse,
  VideoResponse,
} from './types'

/**
 * 文生图 — POST /pg/images/generations（JSON）
 */
export async function generateImage(
  payload: ImageRequest
): Promise<ImageResponse> {
  const res = await api.post(API_ENDPOINTS.IMAGES_GENERATIONS, payload, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * 图生图 — POST /pg/images/edits（multipart/form-data）
 */
export async function editImage(formData: FormData): Promise<ImageResponse> {
  const res = await api.post(API_ENDPOINTS.IMAGES_EDITS, formData, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * 创建视频生成任务 — POST /pg/videos（JSON）
 * 返回任务信息（含 task_id/id），通过 fetchVideo 轮询结果
 */
export async function createVideo(
  payload: Record<string, unknown>
): Promise<VideoResponse> {
  const res = await api.post(API_ENDPOINTS.VIDEOS, payload, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * 查询视频任务状态 — GET /pg/videos/:task_id
 */
export async function fetchVideo(taskId: string): Promise<VideoResponse> {
  const res = await api.get(`${API_ENDPOINTS.VIDEOS}/${taskId}`, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * 提示词 AI 优化 — POST /pg/chat/completions（JSON，非流式）
 * 以用户选择的文本模型对提示词进行润色，走 playground 常规计费
 */
export async function optimizePrompt(
  payload: OptimizePromptRequest
): Promise<OptimizePromptResponse> {
  const res = await api.post(API_ENDPOINTS.CHAT_COMPLETIONS, payload, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * 获取用户可用模型。
 * @param endpoints 可选；按端点类型过滤（如 image-generation），空数组表示不过滤
 */
export async function getUserModels(
  endpoints: string[] = []
): Promise<ModelOption[]> {
  const params =
    endpoints.length > 0 ? { endpoint: endpoints.join(',') } : undefined
  const res = await api.get(API_ENDPOINTS.USER_MODELS, { params })
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
 * @param endpoints 可选；仅返回包含支持对应端点模型的分组，空数组表示不过滤
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
