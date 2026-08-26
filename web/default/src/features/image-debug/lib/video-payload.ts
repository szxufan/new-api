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
import { DEFAULT_VIDEO_DURATION, DEFAULT_VIDEO_RESOLUTION } from '../constants'
import type { VideoDebugFormState } from '../types'

/**
 * 构建视频任务提交请求体（与后端 relay/common.TaskSubmitReq 对应）。
 * - size：宽高比（'' 表示 adaptive，不传）
 * - duration：秒数（2-30，-1 智能时长）
 * - images：Base64 dataURL（0 张为文生视频；后端按 1 张/多张自动映射首帧/参考图）
 * - metadata.parameters.resolution：非默认分辨率时透传给万相3.0 等支持方
 */
export function buildVideoPayload(state: VideoDebugFormState): {
  model: string
  prompt: string
  size?: string
  duration: number
  images?: string[]
  metadata?: { parameters: { resolution: string } }
} {
  const payload: {
    model: string
    prompt: string
    size?: string
    duration: number
    images?: string[]
    metadata?: { parameters: { resolution: string } }
  } = {
    model: state.model,
    prompt: state.prompt,
    // -1 智能时长原样透传；0/NaN 回退默认；范围收敛由后端负责
    duration: state.duration || DEFAULT_VIDEO_DURATION,
  }

  if (state.ratio) {
    payload.size = state.ratio
  }

  if (state.images.length > 0) {
    payload.images = state.images
  }

  if (state.resolution && state.resolution !== DEFAULT_VIDEO_RESOLUTION) {
    payload.metadata = { parameters: { resolution: state.resolution } }
  }

  return payload
}