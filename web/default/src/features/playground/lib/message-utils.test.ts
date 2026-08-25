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
  createUserMessage,
  formatMessageForAPI,
} from './message-utils'
import type { Message } from '../types'

describe('createUserMessage', () => {
  it('无附件时 attachments 为 undefined', () => {
    const msg = createUserMessage('hello')
    expect(msg.from).toBe('user')
    expect(msg.attachments).toBeUndefined()
    expect(msg.versions[0].content).toBe('hello')
  })

  it('有附件时 attachments 被正确设置', () => {
    const attachments = [
      {
        id: '1',
        kind: 'image' as const,
        filename: 'photo.jpg',
        mediaType: 'image/jpeg',
        url: 'data:image/jpeg;base64,abc',
      },
    ]
    const msg = createUserMessage('hello', attachments)
    expect(msg.attachments).toEqual(attachments)
  })

  it('空附件数组时 attachments 为 undefined', () => {
    const msg = createUserMessage('hello', [])
    expect(msg.attachments).toBeUndefined()
  })
})

describe('formatMessageForAPI', () => {
  it('无附件时 content 为纯字符串', () => {
    const message: Message = {
      key: '1',
      from: 'user',
      versions: [{ id: 'v1', content: 'hello' }],
    }
    const result = formatMessageForAPI(message)
    expect(result.role).toBe('user')
    expect(typeof result.content).toBe('string')
    expect(result.content).toBe('hello')
  })

  it('有图片附件时 content 为 ContentPart[]（text + image_url）', () => {
    const message: Message = {
      key: '1',
      from: 'user',
      versions: [{ id: 'v1', content: '看这张图' }],
      attachments: [
        {
          id: '1',
          kind: 'image',
          filename: 'photo.jpg',
          mediaType: 'image/jpeg',
          url: 'data:image/jpeg;base64,abc',
        },
      ],
    }
    const result = formatMessageForAPI(message)
    expect(Array.isArray(result.content)).toBe(true)
    const parts = result.content as Array<{
      type: string
      text?: string
      image_url?: { url: string }
    }>
    expect(parts).toHaveLength(2)
    expect(parts[0].type).toBe('text')
    expect(parts[0].text).toBe('看这张图')
    expect(parts[1].type).toBe('image_url')
    expect(parts[1].image_url?.url).toBe('data:image/jpeg;base64,abc')
  })

  it('多张图片按顺序追加', () => {
    const message: Message = {
      key: '1',
      from: 'user',
      versions: [{ id: 'v1', content: '对比这两张' }],
      attachments: [
        {
          id: '1',
          kind: 'image',
          filename: 'a.jpg',
          mediaType: 'image/jpeg',
          url: 'data:image/jpeg;base64,aaa',
        },
        {
          id: '2',
          kind: 'image',
          filename: 'b.jpg',
          mediaType: 'image/png',
          url: 'data:image/png;base64,bbb',
        },
      ],
    }
    const result = formatMessageForAPI(message)
    const parts = result.content as Array<{ type: string }>
    expect(parts).toHaveLength(3) // 1 text + 2 images
    expect(parts[0].type).toBe('text')
    expect(parts[1].type).toBe('image_url')
    expect(parts[2].type).toBe('image_url')
  })

  it('url 已剥离的历史图片：在文本末尾追加占位行', () => {
    const message: Message = {
      key: '1',
      from: 'user',
      versions: [{ id: 'v1', content: '这是之前的消息' }],
      attachments: [
        {
          id: '1',
          kind: 'image',
          filename: 'old-photo.jpg',
          mediaType: 'image/jpeg',
          // url 被剥离
        },
      ],
    }
    const result = formatMessageForAPI(message)
    expect(typeof result.content).toBe('string')
    expect(result.content).toContain('[image: old-photo.jpg]')
    expect(result.content).toContain('这是之前的消息')
  })

  it('非图片附件不影响 content 格式', () => {
    const message: Message = {
      key: '1',
      from: 'user',
      versions: [{ id: 'v1', content: '这是代码' }],
      attachments: [
        {
          id: '1',
          kind: 'file',
          filename: 'code.py',
          mediaType: 'text/plain',
        },
      ],
    }
    const result = formatMessageForAPI(message)
    expect(typeof result.content).toBe('string')
    expect(result.content).toBe('这是代码')
  })

  it('空文本 + 图片附件时 content 为 ContentPart[]', () => {
    const message: Message = {
      key: '1',
      from: 'user',
      versions: [{ id: 'v1', content: '' }],
      attachments: [
        {
          id: '1',
          kind: 'image',
          filename: 'photo.jpg',
          mediaType: 'image/jpeg',
          url: 'data:image/jpeg;base64,abc',
        },
      ],
    }
    const result = formatMessageForAPI(message)
    expect(Array.isArray(result.content)).toBe(true)
    const parts = result.content as Array<{
      type: string
      text?: string
    }>
    expect(parts).toHaveLength(2)
    expect(parts[0].type).toBe('text')
    expect(parts[0].text).toBe('')
    expect(parts[1].type).toBe('image_url')
  })
})
