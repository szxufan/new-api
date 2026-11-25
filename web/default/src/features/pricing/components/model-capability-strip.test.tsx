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
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { PricingModel } from '../types'
import { ModelCapabilityStrip } from './model-capability-strip'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function makeModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'deepseek-v4-flash',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

const configuredCapabilities = {
  context_length: 200000,
  max_output_tokens: 32768,
  support_image_input: true,
  reasoning_options: ['high', 'low'],
} satisfies Partial<PricingModel>

const billingFollow = {
  model_name: 'astron-code-fast',
  billing_mode: 'follow',
  follow_target: 'deepseek-v4-flash',
} satisfies Partial<PricingModel>

function followsLabel(name: string) {
  const label = screen.getByText(name)
  const target = screen.getByText('deepseek-v4-flash')
  expect(
    label.compareDocumentPosition(target) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
}

describe('ModelCapabilityStrip', () => {
  it('未配置任何信息时，四项都显示「未设置」占位', () => {
    render(<ModelCapabilityStrip model={makeModel()} />)
    expect(screen.getAllByTitle('Not Set')).toHaveLength(4)
    expect(screen.queryByText('Capabilities follow')).not.toBeInTheDocument()
    expect(screen.queryByText('Purposes follow')).not.toBeInTheDocument()
  })

  it('展示上下文、多模态输入与按强度排序的思考等级', () => {
    render(<ModelCapabilityStrip model={makeModel(configuredCapabilities)} />)
    expect(screen.getByText('200K')).toBeInTheDocument()
    expect(screen.getByTitle('200,000 tokens')).toBeInTheDocument()
    expect(screen.getByText('Image')).toBeInTheDocument()
    expect(screen.queryByText('Video')).not.toBeInTheDocument()
    expect(screen.queryAllByTitle('Not Set')).toHaveLength(1)
    const low = screen.getByText('low')
    const high = screen.getByText('high')
    expect(
      low.compareDocumentPosition(high) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it('用途按预设顺序展示，而不是照搬存储顺序', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...configuredCapabilities,
          purposes: ['writing', 'chat'],
        })}
      />
    )
    expect(screen.queryAllByTitle('Not Set')).toHaveLength(0)
    const chat = screen.getByText('Chat')
    const writing = screen.getByText('Writing')
    expect(
      chat.compareDocumentPosition(writing) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it('能力信息沿用自跟随目标时提示跟随', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...configuredCapabilities,
          ...billingFollow,
          capability_source: 'follow',
        })}
      />
    )
    expect(screen.getByText('200K')).toBeInTheDocument()
    followsLabel('Capabilities follow')
  })

  it('用途沿用自跟随目标时提示跟随', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...configuredCapabilities,
          ...billingFollow,
          purposes: ['chat'],
          purpose_source: 'follow',
        })}
      />
    )
    expect(screen.getByText('Chat')).toBeInTheDocument()
    followsLabel('Purposes follow')
    expect(screen.queryByText('Capabilities follow')).not.toBeInTheDocument()
  })

  it('能力信息与用途都沿用自跟随目标时合并为一条提示', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...billingFollow,
          capability_source: 'follow',
          purpose_source: 'follow',
        })}
      />
    )
    followsLabel('Capabilities and purposes follow')
    expect(screen.queryByText('Capabilities follow')).not.toBeInTheDocument()
    expect(screen.queryByText('Purposes follow')).not.toBeInTheDocument()
  })

  it('仅计费跟随、信息为自身配置时不提示', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...configuredCapabilities,
          ...billingFollow,
          purposes: ['chat'],
        })}
      />
    )
    expect(screen.queryByText(/follow$/i)).not.toBeInTheDocument()
  })
})
