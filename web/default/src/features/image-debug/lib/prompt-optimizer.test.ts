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
  IMAGE_PROMPT_OPTIMIZE_SYSTEM_PROMPT,
  VIDEO_PROMPT_OPTIMIZE_SYSTEM_PROMPT,
} from '../constants'
import type { OptimizePromptResponse } from '../types'
import {
  buildOptimizePayload,
  extractOptimizedPrompt,
  getOptimizeErrorMessage,
  sortModelOptions,
} from './prompt-optimizer'

describe('sortModelOptions', () => {
  it('按名称不区分大小写、自然数字序升序排列', () => {
    const options = [
      { label: 'gpt-4o', value: 'gpt-4o' },
      { label: 'GPT-4', value: 'GPT-4' },
      { label: 'claude-3-5-sonnet', value: 'claude-3-5-sonnet' },
      { label: 'gpt-4o-mini', value: 'gpt-4o-mini' },
      { label: 'gpt-4-1106', value: 'gpt-4-1106' },
    ]
    const result = sortModelOptions(options)
    expect(result.map((o) => o.value)).toEqual([
      'claude-3-5-sonnet',
      'GPT-4',
      'gpt-4-1106',
      'gpt-4o',
      'gpt-4o-mini',
    ])
  })

  it('返回新数组，不修改入参', () => {
    const options = [
      { label: 'b', value: 'b' },
      { label: 'a', value: 'a' },
    ]
    const result = sortModelOptions(options)
    expect(result).not.toBe(options)
    expect(options.map((o) => o.value)).toEqual(['b', 'a'])
  })

  it('空数组返回空数组', () => {
    expect(sortModelOptions([])).toEqual([])
  })
})

describe('buildOptimizePayload', () => {
  it('image 类型使用图像系统提示词，video 类型使用视频系统提示词', () => {
    const imagePayload = buildOptimizePayload({
      prompt: 'a cat',
      type: 'image',
      model: 'gpt-4o',
      group: 'default',
    })
    expect(imagePayload.messages[0].content).toBe(
      IMAGE_PROMPT_OPTIMIZE_SYSTEM_PROMPT
    )

    const videoPayload = buildOptimizePayload({
      prompt: 'a cat running',
      type: 'video',
      model: 'gpt-4o',
      group: 'default',
    })
    expect(videoPayload.messages[0].content).toBe(
      VIDEO_PROMPT_OPTIMIZE_SYSTEM_PROMPT
    )
  })

  it('构建 system + user 消息、model、group 与 stream: false', () => {
    const payload = buildOptimizePayload({
      prompt: 'a cat',
      type: 'image',
      model: 'gpt-4o',
      group: 'my-group',
    })
    expect(payload.model).toBe('gpt-4o')
    expect(payload.group).toBe('my-group')
    expect(payload.stream).toBe(false)
    expect(payload.messages).toEqual([
      { role: 'system', content: IMAGE_PROMPT_OPTIMIZE_SYSTEM_PROMPT },
      { role: 'user', content: 'a cat' },
    ])
  })

  it('group 为空白时不携带 group 字段', () => {
    const payload = buildOptimizePayload({
      prompt: 'a cat',
      type: 'image',
      model: 'gpt-4o',
      group: '  ',
    })
    expect(payload.group).toBeUndefined()
  })
})

describe('extractOptimizedPrompt', () => {
  function createResponse(content?: string): OptimizePromptResponse {
    return { choices: [{ message: { content } }] }
  }

  it('提取优化后的提示词并去除首尾空白', () => {
    expect(extractOptimizedPrompt(createResponse('  a beautiful cat  '))).toBe(
      'a beautiful cat'
    )
  })

  it('剥离 markdown 代码块围栏', () => {
    expect(
      extractOptimizedPrompt(createResponse('```\na beautiful cat\n```'))
    ).toBe('a beautiful cat')
    expect(
      extractOptimizedPrompt(
        createResponse('```markdown\na beautiful cat\n```')
      )
    ).toBe('a beautiful cat')
  })

  it('缺失内容返回 null', () => {
    expect(extractOptimizedPrompt({})).toBeNull()
    expect(extractOptimizedPrompt({ choices: [] })).toBeNull()
    expect(extractOptimizedPrompt(createResponse(undefined))).toBeNull()
    expect(extractOptimizedPrompt(createResponse(''))).toBeNull()
    expect(extractOptimizedPrompt(createResponse('   '))).toBeNull()
    expect(extractOptimizedPrompt(createResponse('```\n```'))).toBeNull()
  })
})

describe('getOptimizeErrorMessage', () => {
  it('提取 axios 错误结构中的 response.data.error.message', () => {
    const err = {
      response: { data: { error: { message: 'insufficient quota' } } },
    }
    expect(getOptimizeErrorMessage(err, 'fallback')).toBe('insufficient quota')
  })

  it('普通 Error 取 message', () => {
    expect(getOptimizeErrorMessage(new Error('boom'), 'fallback')).toBe('boom')
  })

  it('未知错误类型返回兜底文案', () => {
    expect(getOptimizeErrorMessage(null, 'fallback')).toBe('fallback')
    expect(getOptimizeErrorMessage({ foo: 'bar' }, 'fallback')).toBe('fallback')
  })
})
