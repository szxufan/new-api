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
import { getVideoResolutions, VIDEO_RESOLUTIONS } from './constants'

describe('getVideoResolutions', () => {
  it('MiniMax 模型仅支持 768P / 2K（大小写与命名空间变体均命中）', () => {
    expect(getVideoResolutions('MiniMax/MiniMax-H3')).toEqual(['768P', '2K'])
    expect(getVideoResolutions('MiniMax-H3')).toEqual(['768P', '2K'])
    expect(getVideoResolutions('minimax-h3')).toEqual(['768P', '2K'])
  })

  it('HappyHorse 模型仅支持 720P / 1080P', () => {
    expect(getVideoResolutions('happyhorse-1.1-r2v')).toEqual([
      '720P',
      '1080P',
    ])
    expect(getVideoResolutions('happyhorse-1.0-t2v')).toEqual([
      '720P',
      '1080P',
    ])
  })

  it('其他模型返回通用档位', () => {
    const defaults = VIDEO_RESOLUTIONS.map((r) => r.value)
    expect(getVideoResolutions('wan3.0-video')).toEqual(defaults)
    expect(getVideoResolutions('viduq2')).toEqual(defaults)
    expect(getVideoResolutions('')).toEqual(defaults)
  })
})
