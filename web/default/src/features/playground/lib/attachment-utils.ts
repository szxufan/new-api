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
  ATTACHMENT_IMAGE_JPEG_QUALITY,
  ATTACHMENT_IMAGE_MAX_DIMENSION,
} from '../constants'

/**
 * 是否为图片类型（按 MIME 判断）
 */
export function isImageMediaType(mediaType: string): boolean {
  return typeof mediaType === 'string' && mediaType.startsWith('image/')
}

// 文本类文件 MIME 白名单
const TEXT_MEDIA_TYPE_PREFIXES = ['text/']
const TEXT_MEDIA_TYPE_EXACT = new Set([
  'application/json',
  'application/xml',
  'application/javascript',
  'application/typescript',
  'application/x-yaml',
  'application/x-sh',
  'application/yaml',
])

// 文本类文件扩展名白名单
const TEXT_FILE_EXTENSIONS = new Set([
  '.md',
  '.markdown',
  '.txt',
  '.json',
  '.csv',
  '.log',
  '.py',
  '.js',
  '.jsx',
  '.ts',
  '.tsx',
  '.java',
  '.go',
  '.rs',
  '.c',
  '.cpp',
  '.h',
  '.hpp',
  '.yml',
  '.yaml',
  '.toml',
  '.ini',
  '.sql',
  '.sh',
  '.xml',
  '.html',
  '.css',
  '.scss',
  '.less',
  '.php',
  '.rb',
  '.kt',
  '.swift',
  '.scala',
  '.lua',
  '.r',
  '.pl',
])

/**
 * 是否为文本类文件（MIME 或扩展名命中白名单）
 */
export function isTextLikeFile(
  mediaType: string,
  filename: string
): boolean {
  if (typeof mediaType === 'string') {
    if (TEXT_MEDIA_TYPE_PREFIXES.some((p) => mediaType.startsWith(p))) {
      return true
    }
    if (TEXT_MEDIA_TYPE_EXACT.has(mediaType)) {
      return true
    }
  }
  if (typeof filename === 'string') {
    const lower = filename.toLowerCase()
    const dotIndex = lower.lastIndexOf('.')
    if (dotIndex >= 0) {
      const ext = lower.slice(dotIndex)
      if (TEXT_FILE_EXTENSIONS.has(ext)) {
        return true
      }
    }
  }
  return false
}

/**
 * 计算缩放后的尺寸，保持比例，最长边不超过 maxDimension
 */
export function computeResizedDimensions(
  width: number,
  height: number,
  maxDimension: number = ATTACHMENT_IMAGE_MAX_DIMENSION
): { width: number; height: number } {
  if (width <= 0 || height <= 0) {
    return { width, height }
  }
  const longest = Math.max(width, height)
  if (longest <= maxDimension) {
    return { width, height }
  }
  const scale = maxDimension / longest
  return {
    width: Math.round(width * scale),
    height: Math.round(height * scale),
  }
}

/**
 * 压缩图片 dataURL：缩放 + 输出 JPEG
 * 输入非图片或解码失败时原样返回
 */
export function compressImageDataUrl(
  dataUrl: string,
  maxDimension: number = ATTACHMENT_IMAGE_MAX_DIMENSION,
  quality: number = ATTACHMENT_IMAGE_JPEG_QUALITY
): Promise<string> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      try {
        const { width, height } = computeResizedDimensions(
          img.naturalWidth || img.width,
          img.naturalHeight || img.height,
          maxDimension
        )
        const canvas = document.createElement('canvas')
        canvas.width = width
        canvas.height = height
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          resolve(dataUrl)
          return
        }
        ctx.drawImage(img, 0, 0, width, height)
        resolve(canvas.toDataURL('image/jpeg', quality))
      } catch (error) {
        reject(error)
      }
    }
    img.onerror = () => resolve(dataUrl)
    img.src = dataUrl
  })
}

/**
 * 读取文件文本内容
 */
export function readFileAsText(file: File): Promise<string> {
  return file.text()
}

/**
 * 将 dataURL 转为 File 对象
 */
export function dataUrlToFile(
  dataUrl: string,
  filename: string,
  mediaType: string = 'image/jpeg'
): File {
  const arr = dataUrl.split(',')
  const mimeMatch = arr[0].match(/:(.*?);/)
  const mime = mimeMatch && mimeMatch[1] ? mimeMatch[1] : mediaType
  const bstr = atob(arr[1] || '')
  let n = bstr.length
  const u8arr = new Uint8Array(n)
  while (n--) {
    u8arr[n] = bstr.charCodeAt(n)
  }
  return new File([u8arr], filename, { type: mime })
}

/**
 * 截屏：调用 getDisplayMedia 捕获屏幕首帧并返回 File
 * 用户取消或浏览器不支持时抛错，由调用方处理
 */
export async function captureScreenshot(): Promise<File> {
  if (
    !navigator.mediaDevices ||
    typeof navigator.mediaDevices.getDisplayMedia !== 'function'
  ) {
    throw new Error('SCREEN_CAPTURE_NOT_SUPPORTED')
  }

  const stream = await navigator.mediaDevices.getDisplayMedia({
    video: true,
    audio: false,
  })

  try {
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.srcObject = stream

    await new Promise<void>((resolve, reject) => {
      video.onloadeddata = () => resolve()
      video.onerror = () => reject(new Error('VIDEO_LOAD_FAILED'))
      // 保险超时
      setTimeout(() => resolve(), 2000)
    })

    // 确保有帧可绘
    try {
      await video.play()
    } catch {
      // 忽略 play 中断
    }

    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth || 1280
    canvas.height = video.videoHeight || 720
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      throw new Error('CANVAS_CONTEXT_FAILED')
    }
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height)

    const dataUrl = canvas.toDataURL('image/jpeg', ATTACHMENT_IMAGE_JPEG_QUALITY)
    return dataUrlToFile(dataUrl, `screenshot-${Date.now()}.jpg`)
  } finally {
    stream.getTracks().forEach((track) => track.stop())
  }
}
