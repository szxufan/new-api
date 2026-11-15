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
import { CanceledError } from 'axios'
import { describe, it, expect } from 'vitest'
import {
  AudioRequestError,
  isAudioRequestCanceled,
  resolveAudioErrorMessage,
  type TranslateFn,
} from './errors'

const t: TranslateFn = (key) => `T(${key})`

describe('resolveAudioErrorMessage', () => {
  it('优先展示上游返回的原始错误文案', () => {
    const error = new AudioRequestError('Request failed', 'invalid api key')
    expect(resolveAudioErrorMessage(error, t)).toBe('invalid api key')
  })

  it('没有上游原文时翻译内部兜底 key', () => {
    expect(
      resolveAudioErrorMessage(new AudioRequestError('Request failed'), t)
    ).toBe('T(Request failed)')
  })

  it('未知错误回退到通用文案', () => {
    expect(resolveAudioErrorMessage(undefined, t)).toBe(
      'T(Request error occurred)'
    )
    expect(resolveAudioErrorMessage(new Error(''), t)).toBe(
      'T(Request error occurred)'
    )
  })

  it('保留普通 Error 的 message', () => {
    expect(resolveAudioErrorMessage(new Error('boom'), t)).toBe('boom')
  })
})

describe('isAudioRequestCanceled', () => {
  it('识别 axios 取消错误', () => {
    expect(isAudioRequestCanceled(new CanceledError('canceled'))).toBe(true)
  })

  it('普通错误不是取消', () => {
    expect(isAudioRequestCanceled(new Error('boom'))).toBe(false)
  })
})
