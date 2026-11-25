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
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'
import { MODEL_PURPOSES } from '@/features/models/constants'
import type { PricingModel } from '../types'
import {
  getConfiguredInputModalities,
  getContextMeterRatio,
  getInheritedCapabilityTarget,
  getInheritedPurposeTarget,
  getOrderedPurposes,
  getPurposeLabels,
  getThinkingLevels,
} from './model-capabilities'

function makeModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('getConfiguredInputModalities', () => {
  it('未配置图片/视频输入时不报告任何多模态输入', () => {
    expect(getConfiguredInputModalities(makeModel())).toEqual([])
  })

  it('仅返回元数据中启用的输入类型，并保持图片在前的顺序', () => {
    expect(
      getConfiguredInputModalities(
        makeModel({ support_video_input: true, support_image_input: true })
      )
    ).toEqual(['image', 'video'])
    expect(
      getConfiguredInputModalities(makeModel({ support_video_input: true }))
    ).toEqual(['video'])
  })
})

describe('getInheritedCapabilityTarget', () => {
  it('后端标记能力信息沿用自跟随目标时，返回该目标模型', () => {
    expect(
      getInheritedCapabilityTarget(
        makeModel({
          capability_source: 'follow',
          follow_target: 'deepseek-v4-flash',
        })
      )
    ).toBe('deepseek-v4-flash')
  })

  it('自身配置或未配置能力信息时没有继承来源', () => {
    expect(getInheritedCapabilityTarget(makeModel())).toBeUndefined()
    // 仅为计费跟随、能力信息仍是自身配置的模型不提示
    expect(
      getInheritedCapabilityTarget(
        makeModel({ follow_target: 'deepseek-v4-flash' })
      )
    ).toBeUndefined()
  })
})

describe('getInheritedPurposeTarget', () => {
  it('后端标记用途沿用自跟随目标时，返回该目标模型', () => {
    expect(
      getInheritedPurposeTarget(
        makeModel({
          purpose_source: 'follow',
          follow_target: 'deepseek-v4-flash',
        })
      )
    ).toBe('deepseek-v4-flash')
  })

  it('用途的继承来源与能力信息的继承来源相互独立', () => {
    const model = makeModel({
      follow_target: 'deepseek-v4-flash',
      capability_source: 'follow',
    })
    expect(getInheritedPurposeTarget(model)).toBeUndefined()
    expect(getInheritedCapabilityTarget(model)).toBe('deepseek-v4-flash')
    expect(getInheritedPurposeTarget(makeModel())).toBeUndefined()
  })
})

describe('getOrderedPurposes', () => {
  it('未配置用途时返回空数组', () => {
    expect(getOrderedPurposes(undefined)).toEqual([])
    expect(getOrderedPurposes([])).toEqual([])
  })

  it('按预设顺序排列并去重，而不是照搬存储顺序', () => {
    expect(getOrderedPurposes(['writing', 'chat', 'writing'])).toEqual([
      'chat',
      'writing',
    ])
    expect(getOrderedPurposes(['image_generation', 'asr', 'chat'])).toEqual([
      'chat',
      'asr',
      'image_generation',
    ])
  })

  it('未知用途保持原有相对顺序并排在预设之后', () => {
    expect(getOrderedPurposes(['custom-b', 'writing', 'custom-a'])).toEqual([
      'writing',
      'custom-b',
      'custom-a',
    ])
  })
})

describe('getPurposeLabels', () => {
  it('每个预设用途都有对应翻译标签', () => {
    const labels = getPurposeLabels(
      ((key: string) => key) as unknown as TFunction
    )
    for (const purpose of MODEL_PURPOSES) {
      expect(labels[purpose]).toBeTruthy()
    }
    expect(labels.chat).toBe('Chat')
    expect(labels.image_generation).toBe('Image Generation')
  })
})

describe('getThinkingLevels', () => {
  it('未配置思考等级时返回空数组', () => {
    expect(getThinkingLevels(makeModel())).toEqual([])
    expect(
      getThinkingLevels(makeModel({ reasoning_options: [' ', ''] }))
    ).toEqual([])
  })

  it('按思考强度从低到高排序，而不是照搬配置顺序', () => {
    expect(
      getThinkingLevels(
        makeModel({ reasoning_options: ['high', 'low', 'medium'] })
      )
    ).toEqual(['low', 'medium', 'high'])
  })

  it('排序时忽略大小写并保留首次出现的写法，同时去重与去除首尾空白', () => {
    expect(
      getThinkingLevels(
        makeModel({ reasoning_options: [' High ', 'low', 'high'] })
      )
    ).toEqual(['low', 'High'])
  })

  it('自定义等级保持原有相对顺序，排在已知等级之后', () => {
    expect(
      getThinkingLevels(
        makeModel({ reasoning_options: ['auto', 'high', 'deep', 'low'] })
      )
    ).toEqual(['low', 'high', 'auto', 'deep'])
  })

  it('忽略非字符串的脏数据', () => {
    const model = makeModel({
      reasoning_options: ['low', 42 as unknown as string, 'high'],
    })
    expect(getThinkingLevels(model)).toEqual(['low', 'high'])
  })
})

describe('getContextMeterRatio', () => {
  it('未配置上下文时进度为 0', () => {
    expect(getContextMeterRatio(0)).toBe(0)
    expect(getContextMeterRatio(Number.NaN)).toBe(0)
  })

  it('8K 与 1M 分别对应刻度两端，中间按对数分布', () => {
    expect(getContextMeterRatio(8_192)).toBeCloseTo(0, 10)
    expect(getContextMeterRatio(1_048_576)).toBeCloseTo(1, 10)
    // 对数刻度的中点：8K × 2^3.5
    expect(getContextMeterRatio(8_192 * 2 ** 3.5)).toBeCloseTo(0.5, 10)
  })

  it('超出刻度范围时被钳制在 0–1 之间', () => {
    expect(getContextMeterRatio(4_096)).toBe(0)
    expect(getContextMeterRatio(2_097_152)).toBe(1)
  })
})
