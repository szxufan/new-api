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

describe('ModelCapabilityStrip', () => {
  it('未配置任何能力信息时，三项都显示「未设置」占位', () => {
    render(<ModelCapabilityStrip model={makeModel()} />)
    expect(screen.getAllByTitle('Not Set')).toHaveLength(3)
    expect(screen.queryByText('Capabilities follow')).not.toBeInTheDocument()
  })

  it('展示上下文、多模态输入与按强度排序的思考等级', () => {
    render(<ModelCapabilityStrip model={makeModel(configuredCapabilities)} />)
    expect(screen.getByText('200K')).toBeInTheDocument()
    expect(screen.getByTitle('200,000 tokens')).toBeInTheDocument()
    expect(screen.getByText('Image')).toBeInTheDocument()
    expect(screen.queryByText('Video')).not.toBeInTheDocument()
    expect(screen.queryAllByTitle('Not Set')).toHaveLength(0)
    const low = screen.getByText('low')
    const high = screen.getByText('high')
    expect(
      low.compareDocumentPosition(high) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it('能力信息沿用自跟随目标时展示跟随提示', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...configuredCapabilities,
          model_name: 'astron-code-fast',
          billing_mode: 'follow',
          follow_target: 'deepseek-v4-flash',
          capability_source: 'follow',
        })}
      />
    )
    expect(screen.getByText('200K')).toBeInTheDocument()
    expect(screen.getByText('Capabilities follow')).toBeInTheDocument()
    expect(screen.getByText('deepseek-v4-flash')).toBeInTheDocument()
  })

  it('仅计费跟随、能力信息为自身配置时不提示', () => {
    render(
      <ModelCapabilityStrip
        model={makeModel({
          ...configuredCapabilities,
          billing_mode: 'follow',
          follow_target: 'deepseek-v4-flash',
        })}
      />
    )
    expect(screen.queryByText('Capabilities follow')).not.toBeInTheDocument()
  })
})
