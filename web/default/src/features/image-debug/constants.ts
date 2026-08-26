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
// API 端点
export const API_ENDPOINTS = {
  IMAGES_GENERATIONS: '/pg/images/generations',
  IMAGES_EDITS: '/pg/images/edits',
  VIDEOS: '/pg/videos',
  USER_MODELS: '/api/user/models',
  USER_GROUPS: '/api/user/self/groups',
} as const

// 尺寸、质量、风格、响应格式选项（label 为 i18n 键，与英文源字符串一致）
export const IMAGE_SIZES = [
  { label: '1024 * 1024', value: '1024x1024' },
  { label: '1024 * 1792', value: '1024x1792' },
  { label: '1792 * 1024', value: '1792x1024' },
  { label: '512 * 512', value: '512x512' },
  { label: '256 * 256', value: '256x256' },
  { label: '1280 * 720', value: '1280x720' },
  { label: '720 * 1280', value: '720x1280' },
] as const

export const IMAGE_QUALITIES = [
  { label: 'Standard', value: 'standard' },
  { label: 'HD', value: 'hd' },
] as const

export const IMAGE_STYLES = [
  { label: 'Vivid', value: 'vivid' },
  { label: 'Natural', value: 'natural' },
] as const

export const RESPONSE_FORMATS = [
  { label: 'URL', value: 'url' },
  { label: 'Base64 JSON', value: 'b64_json' },
] as const

// 默认值
export const DEFAULT_MODE = 'generations' as const
export const DEFAULT_SIZE = '1024x1024'
export const DEFAULT_GROUP = 'default'
export const DEFAULT_N = 1
export const DEFAULT_QUALITY = 'standard'
export const DEFAULT_STYLE = 'vivid'
export const DEFAULT_RESPONSE_FORMAT = 'url'

export const N_MIN = 1
export const N_MAX = 10
export const PROMPT_MAX_LENGTH = 4000

// ============================================================================
// 视频调试常量
// ============================================================================

/** 万相3.0 等支持自定义宽高比；'' 表示 adaptive（模型自动推荐） */
export const VIDEO_RATIOS = [
  { label: 'Adaptive', value: '' },
  { label: '16:9', value: '16:9' },
  { label: '9:16', value: '9:16' },
  { label: '1:1', value: '1:1' },
  { label: '4:3', value: '4:3' },
  { label: '3:4', value: '3:4' },
] as const

export const VIDEO_RESOLUTIONS = [
  { label: '480P', value: '480P' },
  { label: '720P', value: '720P' },
  { label: '1080P', value: '1080P' },
] as const

export const DEFAULT_VIDEO_RATIO = ''
export const DEFAULT_VIDEO_RESOLUTION = '1080P'
export const DEFAULT_VIDEO_DURATION = 5
export const VIDEO_DURATION_MIN = 2
export const VIDEO_DURATION_MAX = 30
/** -1 表示智能时长（模型自动推荐） */
export const VIDEO_DURATION_SMART = -1
/** 允许上传的图片数量上限 */
export const VIDEO_IMAGE_MAX = 3
/** 任务轮询间隔（毫秒） */
export const VIDEO_POLL_INTERVAL_MS = 3000