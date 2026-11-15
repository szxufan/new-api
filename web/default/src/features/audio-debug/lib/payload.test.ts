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
import type { ASRFormState, TTSFormState } from '../types'
import {
  buildASRFormData,
  buildTTSPayload,
  resolveSelection,
  resolveSpeed,
} from './payload'

function createTTSState(overrides: Partial<TTSFormState> = {}): TTSFormState {
  return {
    model: 'gpt-4o-mini-tts',
    group: 'default',
    input: 'hello',
    voice: 'alloy',
    speed: '1',
    responseFormat: 'mp3',
    instructions: '',
    ...overrides,
  }
}

function createASRState(overrides: Partial<ASRFormState> = {}): ASRFormState {
  return {
    model: 'whisper-1',
    group: 'default',
    endpoint: 'transcriptions',
    language: '',
    prompt: '',
    responseFormat: 'json',
    ...overrides,
  }
}

describe('resolveSpeed', () => {
  it('空值与非法值返回 undefined（交给上游默认值）', () => {
    expect(resolveSpeed('')).toBeUndefined()
    expect(resolveSpeed('   ')).toBeUndefined()
    expect(resolveSpeed('abc')).toBeUndefined()
  })

  it('保留合法小数（修复输入 0 被重置为 1）', () => {
    expect(resolveSpeed('0.5')).toBe(0.5)
    expect(resolveSpeed('0.25')).toBe(0.25)
  })

  it('钳制到 [0.25, 4]', () => {
    expect(resolveSpeed('0.1')).toBe(0.25)
    expect(resolveSpeed('9')).toBe(4)
    expect(resolveSpeed('-2')).toBe(0.25)
  })
})

describe('buildTTSPayload', () => {
  it('默认不携带 speed', () => {
    expect(buildTTSPayload(createTTSState()).speed).toBeUndefined()
    expect(buildTTSPayload(createTTSState({ speed: '' })).speed).toBeUndefined()
  })

  it('非默认 speed 按数值携带', () => {
    expect(buildTTSPayload(createTTSState({ speed: '0.5' })).speed).toBe(0.5)
    expect(buildTTSPayload(createTTSState({ speed: '3' })).speed).toBe(3)
  })

  it('越界 speed 被钳制后携带', () => {
    expect(buildTTSPayload(createTTSState({ speed: '9' })).speed).toBe(4)
    expect(buildTTSPayload(createTTSState({ speed: '0.1' })).speed).toBe(0.25)
  })

  it('携带 group 与去空格后的 instructions', () => {
    const payload = buildTTSPayload(
      createTTSState({ group: 'vip', instructions: '  轻快  ' })
    )
    expect(payload.group).toBe('vip')
    expect(payload.instructions).toBe('轻快')
    expect(payload.response_format).toBe('mp3')
  })
})

describe('buildASRFormData', () => {
  const file = new File(['fake-audio'], 'recording.webm', {
    type: 'audio/webm',
  })

  it('无音频文件时返回 null', () => {
    expect(buildASRFormData(createASRState(), null)).toBeNull()
  })

  it('包含文件、模型、分组与格式字段', () => {
    const formData = buildASRFormData(createASRState({ language: 'zh' }), file)
    expect(formData).not.toBeNull()
    expect(formData!.get('file')).toBe(file)
    expect(formData!.get('model')).toBe('whisper-1')
    expect(formData!.get('group')).toBe('default')
    expect(formData!.get('language')).toBe('zh')
    expect(formData!.get('response_format')).toBe('json')
  })

  it('空白 language/prompt 不追加', () => {
    const formData = buildASRFormData(
      createASRState({ language: '  ', prompt: ' ' }),
      file
    )
    expect(formData!.get('language')).toBeNull()
    expect(formData!.get('prompt')).toBeNull()
  })
})

describe('resolveSelection', () => {
  const options = [{ value: 'a' }, { value: 'b' }]

  it('选中值仍有效时保持不变', () => {
    expect(resolveSelection('b', options, 'b')).toBe('b')
  })

  it('选中值失效时回退到第一个选项', () => {
    expect(resolveSelection('missing', options, 'missing')).toBe('a')
  })

  it('选项为空时返回 fallback', () => {
    expect(resolveSelection('b', [], 'b')).toBe('b')
  })
})
