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
import axios from 'axios'

/** i18n 翻译函数的最小签名（便于纯函数与单测复用） */
export type TranslateFn = (key: string) => string

/**
 * 音频调试请求错误。
 * translationKey 为内部兜底文案的 i18n key，detail 为上游返回的原始错误文案（可直接展示）。
 */
export class AudioRequestError extends Error {
  readonly translationKey: string
  readonly detail?: string

  constructor(translationKey: string, detail?: string) {
    super(translationKey)
    this.name = 'AudioRequestError'
    this.translationKey = translationKey
    this.detail = detail
  }
}

/** 请求是否被主动取消（切换 Tab / 卸载时 abort），取消不应作为错误展示 */
export function isAudioRequestCanceled(error: unknown): boolean {
  return axios.isCancel(error)
}

/** 解析错误展示文案：优先展示上游原始错误，其次翻译内部兜底 key */
export function resolveAudioErrorMessage(
  error: unknown,
  t: TranslateFn
): string {
  if (error instanceof AudioRequestError) {
    return error.detail || t(error.translationKey)
  }
  if (error instanceof Error && error.message) return error.message
  return t('Request error occurred')
}
