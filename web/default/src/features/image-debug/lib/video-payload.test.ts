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
import { buildVideoPayload } from './video-payload'
import type { VideoDebugFormState } from '../types'

function createState(overrides: Partial<VideoDebugFormState>): VideoDebugFormState {
  return {
    model: 'wan3.0-video',
    group: 'default',
    prompt: '一只猫在奔跑',
    images: [],
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
})