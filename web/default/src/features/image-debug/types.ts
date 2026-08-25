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