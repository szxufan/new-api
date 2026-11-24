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
import { describe, expect, it } from 'vitest'
import type { PricingModel } from '../types'
import { formatTokenCount, inferModelMetadata } from './model-metadata'

function makeModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'gpt-4o-128k',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('formatTokenCount', () => {
  it('未配置（0 或非法值）时显示占位符', () => {
    expect(formatTokenCount(0)).toBe('—')
    expect(formatTokenCount(-1)).toBe('—')
    expect(formatTokenCount(Number.NaN)).toBe('—')
  })

  it('十进制单位保留一位小数', () => {
    expect(formatTokenCount(999)).toBe('999')
    expect(formatTokenCount(128_000)).toBe('128K')
    expect(formatTokenCount(1_500_000)).toBe('1.5M')
    expect(formatTokenCount(2_000_000)).toBe('2M')
  })

  it('1024 进制的上下文显示为整数，而不是 131.1K', () => {
    expect(formatTokenCount(8_192)).toBe('8K')
    expect(formatTokenCount(131_072)).toBe('128K')
    expect(formatTokenCount(1_048_576)).toBe('1M')
  })

  it('两个进制都是整数时优先十进制，128000 不会被显示成 125K', () => {
    expect(formatTokenCount(128_000)).toBe('128K')
    expect(formatTokenCount(256_000)).toBe('256K')
  })
})

describe('inferModelMetadata', () => {
  it('上下文与最大输出只取后端元数据，未配置时为 0', () => {
    const metadata = inferModelMetadata(makeModel())
    expect(metadata.context_length).toBe(0)
    expect(metadata.max_output_tokens).toBe(0)
  })

  it('原样返回后端配置的上下文与最大输出', () => {
    const metadata = inferModelMetadata(
      makeModel({ context_length: 200_000, max_output_tokens: 32_768 })
    )
    expect(metadata.context_length).toBe(200_000)
    expect(metadata.max_output_tokens).toBe(32_768)
  })

  it('元数据启用图片/视频输入时计入输入模态与视觉能力', () => {
    const metadata = inferModelMetadata(
      makeModel({
        model_name: 'custom-model',
        support_image_input: true,
        support_video_input: true,
      })
    )
    expect(metadata.input_modalities).toEqual(['text', 'image', 'video'])
    expect(metadata.capabilities).toContain('vision')
  })
})
