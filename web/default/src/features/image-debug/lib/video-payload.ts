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
import {
  DEFAULT_VIDEO_DURATION,
  DEFAULT_VIDEO_RESOLUTION,
  VIDEO_GENERATION_MODES,
} from '../constants'
import type { VideoDebugFormState, VideoGenerationMode } from '../types'

/** 统一素材具名键（与后端 relaycommon.MetadataKey* 对应，见 docs/video-api.md） */
const METADATA_KEY_FIRST_FRAME = 'first_frame_image'
const METADATA_KEY_LAST_FRAME = 'last_frame_image'
const METADATA_KEY_REFERENCE_IMAGES = 'reference_images'

export type VideoPayloadMetadata = {
  parameters?: { resolution: string }
  first_frame_image?: string
  last_frame_image?: string
  reference_images?: string[]
}

/** 实际生效的模式：图片数不满足所选模式最低要求时回落 auto（防御；
 * UI 层已禁用不可用选项）。 */
export function effectiveGenerationMode(
  state: Pick<VideoDebugFormState, 'images' | 'generationMode'>
): VideoGenerationMode {
  const option = VIDEO_GENERATION_MODES.find(
    (m) => m.value === state.generationMode
  )
  if (!option || state.images.length < option.minImages) return 'auto'
  return state.generationMode
}

/**
 * 构建视频任务提交请求体（与后端 relay/common.TaskSubmitReq 对应）。
 * - size：宽高比（'' 表示 adaptive，不传）
 * - duration：秒数（2-30，-1 智能时长）
 * - generationMode：
 *   - 'auto'：透传 images，后端按数量自动映射首帧/首尾帧/参考图（现状行为）
 *   - 'text2video'：不下发任何素材（纯文本生成）
 *   - 其他显式模式：经 metadata 统一具名键指定
 *     （first_frame_image / last_frame_image / reference_images）
 * - metadata.parameters.resolution：非默认分辨率时透传给万相3.0 等支持方
 */
export function buildVideoPayload(state: VideoDebugFormState): {
  model: string
  prompt: string
  size?: string
  duration: number
  images?: string[]
  metadata?: VideoPayloadMetadata
} {
  const payload: {
    model: string
    prompt: string
    size?: string
    duration: number
    images?: string[]
    metadata?: VideoPayloadMetadata
  } = {
    model: state.model,
    prompt: state.prompt,
    // -1 智能时长原样透传；0/NaN 回退默认；范围收敛由后端负责
    duration: state.duration || DEFAULT_VIDEO_DURATION,
  }

  if (state.ratio) {
    payload.size = state.ratio
  }

  const metadata: VideoPayloadMetadata = {}
  if (state.resolution && state.resolution !== DEFAULT_VIDEO_RESOLUTION) {
    metadata.parameters = { resolution: state.resolution }
  }

  if (state.images.length > 0) {
    const mode = effectiveGenerationMode(state)
    switch (mode) {
      case 'text2video':
        // 文生视频：不下发任何素材（已上传图片仅用于提示词优化）
        break
      case 'image2video':
        metadata[METADATA_KEY_FIRST_FRAME] = state.images[0]
        break
      case 'first_last_frame':
        metadata[METADATA_KEY_FIRST_FRAME] = state.images[0]
        metadata[METADATA_KEY_LAST_FRAME] = state.images[1]
        break
      case 'reference2video':
        metadata[METADATA_KEY_REFERENCE_IMAGES] = state.images
        break
      default:
        // auto：保持现状，透传 images 由后端按数量推导
        payload.images = state.images
    }
  }

  if (Object.keys(metadata).length > 0) {
    payload.metadata = metadata
  }

  return payload
}