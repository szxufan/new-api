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
import { buildEditFormData, buildImagePayload, parseExtraParameters } from './payload'
import type { ImageDebugFormState } from '../types'

function createState(overrides: Partial<ImageDebugFormState> = {}): ImageDebugFormState {
  return {
    mode: 'generations',
    model: 'wan2.7-image-pro',
    group: 'default',
    prompt: 'a cat',
    size: '1024x1024',
    n: 1,
    quality: 'standard',
    style: 'vivid',
    responseFormat: 'url',
    watermark: false,
    extraParameters: '',
    ...overrides,
  }
}

describe('parseExtraParameters', () => {
  it('空文本返回 undefined', () => {
    expect(parseExtraParameters('')).toBeUndefined()
    expect(parseExtraParameters('   ')).toBeUndefined()
  })

  it('合法 JSON 对象返回原对象', () => {
    expect(parseExtraParameters('{"size":"1024*1024","prompt_extend":true}')).toEqual({
      size: '1024*1024',
      prompt_extend: true,
    })
  })

  it('非法 JSON 返回 undefined', () => {
    expect(parseExtraParameters('{invalid')).toBeUndefined()
  })

  it('非对象 JSON（数组/标量）返回 undefined', () => {
    expect(parseExtraParameters('[1,2,3]')).toBeUndefined()
    expect(parseExtraParameters('"text"')).toBeUndefined()
  })
})

describe('buildImagePayload', () => {
  it('包含 model 与 prompt 基础字段', () => {
    const payload = buildImagePayload(createState())
    expect(payload.model).toBe('wan2.7-image-pro')
    expect(payload.prompt).toBe('a cat')
  })

  it('携带全部已配置参数', () => {
    const payload = buildImagePayload(
      createState({
        n: 4,
        size: '1024x1792',
        quality: 'hd',
        style: 'natural',
        responseFormat: 'b64_json',
        watermark: true,
      })
    )
    expect(payload).toMatchObject({
      size: '1024x1792',
      n: 4,
      quality: 'hd',
      style: 'natural',
      response_format: 'b64_json',
      watermark: true,
    })
  })

  it('空参数不写入 payload', () => {
    const payload = buildImagePayload(
      createState({ size: '', quality: '', style: '', responseFormat: '', n: 0 })
    )
    expect(payload.size).toBeUndefined()
    expect(payload.quality).toBeUndefined()
    expect(payload.style).toBeUndefined()
    expect(payload.response_format).toBeUndefined()
    expect(payload.n).toBeUndefined()
    expect(payload.watermark).toBeUndefined()
  })

  it('合法高级参数透传到 extra.parameters', () => {
    const payload = buildImagePayload(
      createState({ extraParameters: '{"prompt_extend":true,"watermark":false}' })
    )
    expect(payload.extra).toEqual({
      parameters: { prompt_extend: true, watermark: false },
    })
  })

  it('非法高级参数不产生 extra', () => {
    const payload = buildImagePayload(createState({ extraParameters: '{bad json' }))
    expect(payload.extra).toBeUndefined()
  })
})

describe('buildEditFormData', () => {
  const file = new File(['fake-image-bytes'], 'photo.png', { type: 'image/png' })

  it('未选择图片时返回 null', () => {
    expect(buildEditFormData(createState(), [])).toBeNull()
  })

  it('包含 image 文件字段与文本参数', () => {
    const formData = buildEditFormData(
      createState({
        mode: 'edits',
        n: 2,
        size: '1024x1024',
        quality: 'hd',
        style: 'natural',
        responseFormat: 'b64_json',
        watermark: true,
      }),
      [file]
    )
    expect(formData).not.toBeNull()
    expect(formData!.get('image')).toBe(file)
    expect(formData!.get('model')).toBe('wan2.7-image-pro')
    expect(formData!.get('prompt')).toBe('a cat')
    expect(formData!.get('n')).toBe('2')
    expect(formData!.get('quality')).toBe('hd')
    expect(formData!.get('style')).toBe('natural')
    expect(formData!.get('response_format')).toBe('b64_json')
    expect(formData!.get('watermark')).toBe('true')
  })

  it('多张图片全部追加到 image 字段', () => {
    const secondFile = new File(['bytes'], 'photo2.jpg', { type: 'image/jpeg' })
    const formData = buildEditFormData(createState({ mode: 'edits' }), [
      file,
      secondFile,
    ])
    const images = formData!.getAll('image')
    expect(images).toHaveLength(2)
  })

  it('空参数不追加对应文本字段', () => {
    const formData = buildEditFormData(
      createState({ mode: 'edits', size: '', n: 0 }),
      [file]
    )
    expect(formData!.get('size')).toBeNull()
    expect(formData!.get('n')).toBeNull()
  })
})