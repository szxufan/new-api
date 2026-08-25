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
import { describe, it, expect } from 'vitest'
import {
  isImageMediaType,
  isTextLikeFile,
  computeResizedDimensions,
  dataUrlToFile,
} from './attachment-utils'

describe('isImageMediaType', () => {
  it('识别 image/* 类型', () => {
    expect(isImageMediaType('image/jpeg')).toBe(true)
    expect(isImageMediaType('image/png')).toBe(true)
    expect(isImageMediaType('image/gif')).toBe(true)
    expect(isImageMediaType('image/webp')).toBe(true)
  })

  it('拒绝非图片类型', () => {
    expect(isImageMediaType('text/plain')).toBe(false)
    expect(isImageMediaType('application/pdf')).toBe(false)
    expect(isImageMediaType('application/json')).toBe(false)
    expect(isImageMediaType('')).toBe(false)
    expect(isImageMediaType('video/mp4')).toBe(false)
  })
})

describe('isTextLikeFile', () => {
  it('按 MIME 前缀识别 text/*', () => {
    expect(isTextLikeFile('text/plain', 'file.txt')).toBe(true)
    expect(isTextLikeFile('text/html', 'file.html')).toBe(true)
    expect(isTextLikeFile('text/csv', 'file.csv')).toBe(true)
    expect(isTextLikeFile('text/markdown', 'file.md')).toBe(true)
  })

  it('按 MIME 精确匹配 application/json 等', () => {
    expect(isTextLikeFile('application/json', 'data.json')).toBe(true)
    expect(isTextLikeFile('application/xml', 'data.xml')).toBe(true)
    expect(isTextLikeFile('application/javascript', 'app.js')).toBe(true)
    expect(isTextLikeFile('application/x-yaml', 'config.yaml')).toBe(true)
    expect(isTextLikeFile('application/x-sh', 'script.sh')).toBe(true)
  })

  it('按文件扩展名识别', () => {
    expect(isTextLikeFile('', 'readme.md')).toBe(true)
    expect(isTextLikeFile('', 'script.py')).toBe(true)
    expect(isTextLikeFile('', 'app.tsx')).toBe(true)
    expect(isTextLikeFile('', 'config.toml')).toBe(true)
    expect(isTextLikeFile('', 'query.sql')).toBe(true)
    expect(isTextLikeFile('', 'main.go')).toBe(true)
    expect(isTextLikeFile('', 'app.rs')).toBe(true)
  })

  it('拒绝非文本二进制类型', () => {
    expect(isTextLikeFile('application/pdf', 'doc.pdf')).toBe(false)
    expect(isTextLikeFile('application/zip', 'archive.zip')).toBe(false)
    expect(isTextLikeFile('image/png', 'photo.png')).toBe(false)
    expect(isTextLikeFile('video/mp4', 'video.mp4')).toBe(false)
    expect(isTextLikeFile('', 'file.bin')).toBe(false)
    expect(isTextLikeFile('', 'photo.jpg')).toBe(false)
  })

  it('扩展名大小写不敏感', () => {
    expect(isTextLikeFile('', 'README.MD')).toBe(true)
    expect(isTextLikeFile('', 'App.PY')).toBe(true)
  })
})

describe('computeResizedDimensions', () => {
  it('不超过限制时保持原尺寸', () => {
    expect(computeResizedDimensions(800, 600, 1568)).toEqual({
      width: 800,
      height: 600,
    })
  })

  it('宽度超限时按比例缩放', () => {
    const result = computeResizedDimensions(3136, 2352, 1568)
    expect(result.width).toBe(1568)
    expect(result.height).toBe(1176)
  })

  it('高度超限时按比例缩放', () => {
    const result = computeResizedDimensions(2352, 3136, 1568)
    expect(result.width).toBe(1176)
    expect(result.height).toBe(1568)
  })

  it('默认 maxDimension 为 1568', () => {
    const result = computeResizedDimensions(3136, 2352)
    expect(result.width).toBe(1568)
    expect(result.height).toBe(1176)
  })

  it('宽高为零或负数时原样返回', () => {
    expect(computeResizedDimensions(0, 0, 1568)).toEqual({ width: 0, height: 0 })
    expect(computeResizedDimensions(-1, -1, 1568)).toEqual({
      width: -1,
      height: -1,
    })
  })
})

describe('dataUrlToFile', () => {
  it('将 dataURL 转为 File 对象', () => {
    const dataUrl = 'data:text/plain;base64,SGVsbG8='
    const file = dataUrlToFile(dataUrl, 'test.txt', 'text/plain')
    expect(file).toBeInstanceOf(File)
    expect(file.name).toBe('test.txt')
    expect(file.type).toBe('text/plain')
  })

  it('从 dataURL 中提取 MIME 类型', () => {
    const dataUrl = 'data:image/jpeg;base64,abc'
    const file = dataUrlToFile(dataUrl, 'photo.jpg')
    expect(file.type).toBe('image/jpeg')
  })

  it('无 MIME 头时使用默认 mediaType', () => {
    const dataUrl = 'data:;base64,abc'
    const file = dataUrlToFile(dataUrl, 'file.bin', 'application/octet-stream')
    expect(file.type).toBe('application/octet-stream')
  })
})
