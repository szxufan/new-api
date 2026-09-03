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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../api'
import { McpSettingsCard } from './mcp-settings-card'

vi.mock('../api', () => ({
  updateSystemOption: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

// 下拉数据源打桩，聚焦保存行为本身
vi.mock('@/features/channels/api', () => ({
  getEnabledModels: vi.fn().mockResolvedValue({ success: true, data: [] }),
}))
vi.mock('@/features/users/api', () => ({
  getGroups: vi.fn().mockResolvedValue({ data: ['default'] }),
}))

// 注意：这里不 mock GroupModelMapEditor —— 必须走真实编辑器组件，
// 端到端验证「添加行 → 输入 → 保存」发出的请求体内容。
// 此前（1）打桩编辑器、（2）字段名带点号被 RHF 解析为嵌套路径，
// 两个问题叠加导致测试全绿但线上保存丢失数据。

const mockUpdate = vi.mocked(api.updateSystemOption)

const baseDefaults = {
  'mcp_setting.group_image_models': '{"default":"dall-e-3"}',
  'mcp_setting.group_i2i_models': '{}',
  'mcp_setting.group_video_t2v_models': '{}',
  'mcp_setting.group_video_i2v_models': '{}',
  'mcp_setting.group_video_kf2v_models': '{}',
  'mcp_setting.group_video_r2v_models': '{}',
}

const renderCard = (defaultValues: typeof baseDefaults = baseDefaults) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <McpSettingsCard defaultValues={defaultValues} />
    </QueryClientProvider>
  )

const clickSave = async () => {
  fireEvent.click(screen.getByRole('button', { name: /Save Changes/ }))
  await waitFor(() => {
    expect(mockUpdate).toHaveBeenCalledTimes(6)
  })
}

const sentValue = (key: string) =>
  mockUpdate.mock.calls.find((call) => call[0].key === key)?.[0].value

describe('McpSettingsCard 保存行为', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUpdate.mockResolvedValue({ success: true, message: '' })
  })

  it('点击保存应无条件提交全部 6 个配置项（即使与服务器值相同）', async () => {
    renderCard()
    await clickSave()

    // 不应出现"没有需要保存的更改"拦截
    expect(mockUpdate).toHaveBeenCalledWith({
      key: 'mcp_setting.group_image_models',
      value: '{\n  "default": "dall-e-3"\n}',
    })
    expect(sentValue('mcp_setting.group_video_r2v_models')).toBe('{}')
  })

  it('端到端：真实编辑器中添加行并输入分组/模型后保存，请求体应包含新值', async () => {
    renderCard()

    // 在第一个编辑器（文生图模型）中添加一行
    const addButtons = screen.getAllByRole('button', { name: /Add/ })
    fireEvent.click(addButtons[0])

    // 新行出现两个 combobox 输入框（分组 + 模型）
    const comboboxes = screen.getAllByRole('combobox')
    const groupInput = comboboxes[comboboxes.length - 2]
    const modelInput = comboboxes[comboboxes.length - 1]

    fireEvent.change(groupInput, { target: { value: 'vip' } })
    fireEvent.change(modelInput, { target: { value: 'gpt-image-1' } })

    await clickSave()

    // 发出的请求体必须包含用户刚输入的内容（此前 bug：发出的是初始值）
    expect(sentValue('mcp_setting.group_image_models')).toBe(
      JSON.stringify({ default: 'dall-e-3', vip: 'gpt-image-1' }, null, 2)
    )
  })

  it('端到端：defaultValues 引用变化（refetch/重渲染）后，用户编辑不被覆盖', async () => {
    const { rerender } = renderCard()

    // 用户先添加一行并输入
    fireEvent.click(screen.getAllByRole('button', { name: /Add/ })[0])
    // 此时只有这一个编辑器有内容行，其行输入框是全部 combobox 的最后两个
    const comboboxes = screen.getAllByRole('combobox')
    fireEvent.change(comboboxes[comboboxes.length - 2], {
      target: { value: 'vip' },
    })
    fireEvent.change(comboboxes[comboboxes.length - 1], {
      target: { value: 'flux-1' },
    })

    // 模拟父组件重渲染（refetch 后新引用、内容相同）
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <McpSettingsCard defaultValues={{ ...baseDefaults }} />
      </QueryClientProvider>
    )

    await clickSave()

    // 提交的必须是用户编辑后的值，而不是被重置回的初始值
    expect(sentValue('mcp_setting.group_image_models')).toBe(
      JSON.stringify({ default: 'dall-e-3', vip: 'flux-1' }, null, 2)
    )
  })
})
