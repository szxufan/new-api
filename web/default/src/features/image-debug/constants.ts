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