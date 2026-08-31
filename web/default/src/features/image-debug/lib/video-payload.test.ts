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
import { buildVideoPayload, effectiveGenerationMode } from './video-payload'
import type { VideoDebugFormState } from '../types'

function createState(overrides: Partial<VideoDebugFormState>): VideoDebugFormState {
  return {
    model: 'wan3.0-video',
    group: 'default',
    prompt: '一只猫在奔跑',
    images: [],
    generationMode: 'auto',
    ratio: '',
    resolution: '1080P',
    duration: 5,
    ...overrides,
  }
}

describe('buildVideoPayload', () => {
  it('默认状态：仅 model/prompt/duration，无 size/images/metadata', () => {
    expect(buildVideoPayload(createState({}))).toEqual({
      model: 'wan3.0-video',
      prompt: '一只猫在奔跑',
      duration: 5,
    })
  })

  it('指定宽高比时透传 size', () => {
    const payload = buildVideoPayload(createState({ ratio: '9:16' }))
    expect(payload.size).toBe('9:16')
  })

  it('adaptive 宽高比不传 size', () => {
    const payload = buildVideoPayload(createState({ ratio: '' }))
    expect(payload.size).toBeUndefined()
  })

  it('非默认分辨率透传 metadata.parameters.resolution', () => {
    const payload = buildVideoPayload(createState({ resolution: '480P' }))
    expect(payload.metadata).toEqual({ parameters: { resolution: '480P' } })
  })

  it('默认 1080P 分辨率不传 metadata', () => {
    const payload = buildVideoPayload(createState({ resolution: '1080P' }))
    expect(payload.metadata).toBeUndefined()
  })

  it('上传图片时透传 images 列表', () => {
    const images = ['data:image/png;base64,aaa', 'data:image/png;base64,bbb']
    const payload = buildVideoPayload(createState({ images }))
    expect(payload.images).toEqual(images)
  })

  it('未上传图片时不传 images 字段', () => {
    const payload = buildVideoPayload(createState({}))
    expect(payload.images).toBeUndefined()
  })

  it('智能时长 -1 原样透传', () => {
    const payload = buildVideoPayload(createState({ duration: -1 }))
    expect(payload.duration).toBe(-1)
  })

  it('时长 0 回退默认 5 秒', () => {
    const payload = buildVideoPayload(createState({ duration: 0 }))
    expect(payload.duration).toBe(5)
  })

  // ==========================================================================
  // 生成模式（统一具名键适配）
  // ==========================================================================

  it('auto 模式：透传 images，不产生具名键（现状行为）', () => {
    const images = ['data:image/png;base64,aaa', 'data:image/png;base64,bbb']
    const payload = buildVideoPayload(createState({ images }))
    expect(payload.images).toEqual(images)
    expect(payload.metadata).toBeUndefined()
  })

  it('image2video：单图经 first_frame_image 下发', () => {
    const images = ['data:image/png;base64,aaa']
    const payload = buildVideoPayload(
      createState({ images, generationMode: 'image2video' })
    )
    expect(payload.images).toBeUndefined()
    expect(payload.metadata).toEqual({ first_frame_image: images[0] })
  })

  it('first_last_frame：两张图分别映射为首帧/尾帧', () => {
    const images = ['data:image/png;base64,aaa', 'data:image/png;base64,bbb']
    const payload = buildVideoPayload(
      createState({ images, generationMode: 'first_last_frame' })
    )
    expect(payload.images).toBeUndefined()
    expect(payload.metadata).toEqual({
      first_frame_image: images[0],
      last_frame_image: images[1],
    })
  })

  it('reference2video：全部图片经 reference_images 下发', () => {
    const images = ['data:image/png;base64,aaa', 'data:image/png;base64,bbb']
    const payload = buildVideoPayload(
      createState({ images, generationMode: 'reference2video' })
    )
    expect(payload.images).toBeUndefined()
    expect(payload.metadata).toEqual({ reference_images: images })
  })

  it('显式模式但图片不足时回落 auto（防御）', () => {
    const images = ['data:image/png;base64,aaa']
    const payload = buildVideoPayload(
      createState({ images, generationMode: 'first_last_frame' })
    )
    expect(payload.images).toEqual(images)
    expect(payload.metadata).toBeUndefined()
  })

  it('显式模式与非默认分辨率：具名键与 parameters 合并到同一 metadata', () => {
    const images = ['data:image/png;base64,aaa']
    const payload = buildVideoPayload(
      createState({
        images,
        generationMode: 'image2video',
        resolution: '480P',
      })
    )
    expect(payload.metadata).toEqual({
      parameters: { resolution: '480P' },
      first_frame_image: images[0],
    })
  })

  it('无图片时显式模式不产生任何素材字段（文生视频）', () => {
    const payload = buildVideoPayload(
      createState({ generationMode: 'image2video' })
    )
    expect(payload.images).toBeUndefined()
    expect(payload.metadata).toBeUndefined()
  })

  it('text2video：即使有图片也不下发任何素材', () => {
    const images = ['data:image/png;base64,aaa', 'data:image/png;base64,bbb']
    const payload = buildVideoPayload(
      createState({ images, generationMode: 'text2video' })
    )
    expect(payload.images).toBeUndefined()
    expect(payload.metadata).toBeUndefined()
  })

  it('text2video 与非默认分辨率：仅下发 parameters', () => {
    const payload = buildVideoPayload(
      createState({
        images: ['data:image/png;base64,aaa'],
        generationMode: 'text2video',
        resolution: '480P',
      })
    )
    expect(payload.images).toBeUndefined()
    expect(payload.metadata).toEqual({ parameters: { resolution: '480P' } })
  })
})

describe('effectiveGenerationMode', () => {
  it('图片数满足要求时返回所选模式', () => {
    expect(
      effectiveGenerationMode({
        images: ['a', 'b'],
        generationMode: 'first_last_frame',
      })
    ).toBe('first_last_frame')
  })

  it('图片数不足时回落 auto', () => {
    expect(
      effectiveGenerationMode({ images: ['a'], generationMode: 'first_last_frame' })
    ).toBe('auto')
    expect(
      effectiveGenerationMode({ images: [], generationMode: 'image2video' })
    ).toBe('auto')
  })

  it('未知模式回落 auto', () => {
    expect(
      effectiveGenerationMode({
        images: ['a'],
        generationMode: 'bogus' as VideoDebugFormState['generationMode'],
      })
    ).toBe('auto')
  })
})