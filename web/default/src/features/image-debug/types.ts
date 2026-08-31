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
// 与后端 dto/openai_image.go 的 ImageRequest/ImageResponse 对应
export interface ImageRequest {
  model: string
  prompt: string
  n?: number
  size?: string
  quality?: string
  response_format?: string
  style?: string
  watermark?: boolean
  /** 透传给阿里原生参数（如 parameters/input） */
  extra?: Record<string, unknown>
}

export interface ImageData {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface ImageResponse {
  data: ImageData[]
  created?: number
  error?: {
    message?: string
    type?: string
    code?: string
  }
}

export type ImageMode = 'generations' | 'edits'

/** 调试页模式：图像（文生图/图生图）或视频（异步任务） */
export type DebugMode = ImageMode | 'video'

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

/** 表单状态（不含上传图片） */
export interface ImageDebugFormState {
  mode: ImageMode
  model: string
  group: string
  prompt: string
  size: string
  n: number
  quality: string
  style: string
  responseFormat: string
  watermark: boolean
  /** 高级参数 JSON 文本（透传到 extra.parameters） */
  extraParameters: string
}

// ============================================================================
// 视频调试（异步视频生成任务）
// ============================================================================

export type VideoStatus =
  | 'queued'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'unknown'

/** 与后端 dto.OpenAIVideo 对应（POST /pg/videos 创建、GET /pg/videos/:task_id 查询） */
export interface VideoResponse {
  id: string
  task_id?: string
  object?: string
  model?: string
  status: VideoStatus
  progress?: number
  created_at?: number
  completed_at?: number
  seconds?: string
  size?: string
  error?: {
    message?: string
    code?: string
  }
  /** 任务完成后的视频地址等元数据 */
  metadata?: {
    url?: string
  }
}

/** 视频生成模式：与后端统一解析层（docs/video-generation-mode-design.md）对应。
 * - auto：不指定，由后端按 images 数量隐式推导（现状行为）
 * - text2video：文生视频（不下发任何素材；已上传图片仅用于提示词优化）
 * - image2video：首帧生视频（metadata.first_frame_image）
 * - first_last_frame：首尾帧插值（metadata.first_frame_image + last_frame_image）
 * - reference2video：参考生视频（metadata.reference_images）
 */
export type VideoGenerationMode =
  | 'auto'
  | 'text2video'
  | 'image2video'
  | 'first_last_frame'
  | 'reference2video'

/** 视频调试表单状态（图片以 Base64 dataURL 保存在内存） */
export interface VideoDebugFormState {
  model: string
  group: string
  prompt: string
  /** 上传图片的 dataURL 列表：0 张 = 文生视频；1 张 = 首帧生视频；多张 = 参考生视频 */
  images: string[]
  /** 生成模式：'auto' 时按图片数量由后端隐式推导 */
  generationMode: VideoGenerationMode
  /** 宽高比：''(adaptive) / 16:9 / 9:16 / 1:1 / 4:3 / 3:4 */
  ratio: string
  /** 分辨率：480P / 720P / 1080P */
  resolution: string
  /** 时长（秒）：2-30，-1 表示智能时长 */
  duration: number
}

// ============================================================================
// 提示词 AI 优化（POST /pg/chat/completions，非流式）
// ============================================================================

/** 待优化的媒体类型 */
export type PromptOptimizeType = 'image' | 'video'

/** 多模态消息内容片段（视频优化携带输入图片时使用） */
export interface OptimizeContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: { url: string }
}

export interface OptimizePromptMessage {
  role: 'system' | 'user'
  content: string | OptimizeContentPart[]
}

export interface OptimizePromptRequest {
  model: string
  /** 使用分组；缺省时由后端使用会话默认分组 */
  group?: string
  messages: OptimizePromptMessage[]
  stream: false
}

export interface OptimizePromptResponse {
  choices?: { message?: { content?: string } }[]
  error?: { message?: string }
}
