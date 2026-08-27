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
import { TASK_STATUS } from '../constants'
import type { TaskLog } from '../types'

export interface TaskDownloadEntry {
  url: string
  filename: string
}

/**
 * Parse task data payload which may arrive as a parsed array/object
 * (json.RawMessage embedded by the backend) or as a JSON string.
 */
export function parseTaskDataArray(data: unknown): unknown[] {
  if (Array.isArray(data)) return data
  if (typeof data === 'string') {
    try {
      const parsed: unknown = JSON.parse(data)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }
  return []
}

function filenameFromUrl(url: string, fallback: string): string {
  try {
    const pathname = new URL(url).pathname
    const lastSegment = pathname.split('/').filter(Boolean).pop()
    if (lastSegment && lastSegment.includes('.')) {
      return lastSegment
    }
  } catch {
    // ignore non-standard URLs
  }
  return fallback
}

/**
 * Resolve download entries for a completed task result.
 * - Suno: one entry per audio clip found in task data (`audio_url`).
 * - Other platforms: the same-origin video proxy endpoint, which handles all
 *   channel types server-side (owner + SUCCESS enforced by the backend).
 */
export function resolveTaskDownloads(log: TaskLog): TaskDownloadEntry[] {
  if (log.platform === 'suno') {
    return parseTaskDataArray(log.data)
      .filter((item): item is Record<string, unknown> =>
        Boolean(
          item &&
          typeof item === 'object' &&
          typeof (item as Record<string, unknown>).audio_url === 'string'
        )
      )
      .map((item, index) => {
        const url = String(item.audio_url)
        return {
          url,
          filename: filenameFromUrl(url, `${log.task_id}_${index + 1}.mp3`),
        }
      })
  }
  return [
    {
      url: `/v1/videos/${log.task_id}/content`,
      filename: `${log.task_id}.mp4`,
    },
  ]
}

/**
 * Download is only allowed for tasks submitted by the current user that have
 * finished successfully (the backend proxy enforces the same constraints).
 */
export function canDownloadTaskResult(
  log: TaskLog,
  currentUserId: number | undefined
): boolean {
  return (
    log.status === TASK_STATUS.SUCCESS &&
    currentUserId != null &&
    log.user_id === currentUserId
  )
}

const CONTENT_TYPE_EXTENSIONS: Record<string, string> = {
  'video/mp4': '.mp4',
  'video/webm': '.webm',
  'video/quicktime': '.mov',
  'audio/mpeg': '.mp3',
  'audio/mp4': '.m4a',
  'audio/wav': '.wav',
  'audio/ogg': '.ogg',
  'image/png': '.png',
  'image/jpeg': '.jpg',
  'image/webp': '.webp',
}

/**
 * Map a blob content-type to a file extension (including the leading dot).
 * Returns an empty string when the original filename should be kept.
 */
export function extFromContentType(contentType: string): string {
  if (!contentType) return ''
  const normalized = contentType.split(';')[0]?.trim().toLowerCase() ?? ''
  return CONTENT_TYPE_EXTENSIONS[normalized] ?? ''
}

function replaceExtension(filename: string, ext: string): string {
  const dotIndex = filename.lastIndexOf('.')
  const base = dotIndex > 0 ? filename.slice(0, dotIndex) : filename
  return `${base}${ext}`
}

function triggerBlobDownload(blob: Blob, filename: string): void {
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(objectUrl)
}

/**
 * Download all result entries of a task.
 * Prefers fetch -> blob (allows setting a filename); falls back to opening the
 * URL in a new tab when the fetch fails (e.g. cross-origin CORS restrictions
 * on direct Suno CDN URLs).
 * Throws when the task has no downloadable entries.
 */
export async function downloadTaskResults(log: TaskLog): Promise<void> {
  const entries = resolveTaskDownloads(log)
  if (entries.length === 0) {
    throw new Error('No downloadable result entries')
  }
  for (const entry of entries) {
    try {
      const response = await fetch(entry.url, { credentials: 'include' })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const blob = await response.blob()
      const ext = extFromContentType(blob.type)
      const filename = ext
        ? replaceExtension(entry.filename, ext)
        : entry.filename
      triggerBlobDownload(blob, filename)
    } catch {
      window.open(entry.url, '_blank', 'noopener,noreferrer')
    }
  }
}
