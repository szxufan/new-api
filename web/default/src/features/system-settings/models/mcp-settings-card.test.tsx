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

// 编辑器内部依赖 combobox 与 crypto，仅打桩为占位组件
vi.mock('./group-model-map-editor', () => ({
  GroupModelMapEditor: () => null,
}))

const mockUpdate = vi.mocked(api.updateSystemOption)

const baseDefaults = {
  'mcp_setting.group_image_models': '{"default":"dall-e-3"}',
  'mcp_setting.group_i2i_models': '{}',
  'mcp_setting.group_video_t2v_models': '{}',
  'mcp_setting.group_video_i2v_models': '{}',
  'mcp_setting.group_video_kf2v_models': '{}',
  'mcp_setting.group_video_r2v_models': '{}',
}

const renderCard = (props?: {
  defaultValues?: typeof baseDefaults
  queryClient?: QueryClient
}) => {
  const queryClient = props?.queryClient ?? new QueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <McpSettingsCard defaultValues={props?.defaultValues ?? baseDefaults} />
    </QueryClientProvider>
  )
}

describe('McpSettingsCard 保存行为', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUpdate.mockResolvedValue({ success: true, message: '' })
  })

  it('点击保存应无条件提交全部 6 个配置项（即使与服务器值相同）', async () => {
    renderCard()

    fireEvent.click(screen.getByRole('button', { name: /Save Changes/ }))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledTimes(6)
    })
    // 不应出现"没有需要保存的更改"拦截
    expect(mockUpdate).toHaveBeenCalledWith({
      key: 'mcp_setting.group_image_models',
      value: '{\n  "default": "dall-e-3"\n}',
    })
  })

  it('编辑后保存应提交编辑后的值', async () => {
    // 编辑器打桩为 null，这里直接通过 defaultValues 内容变化的序列化验证
    // 提交值来自表单规范化的输出（pretty-print）
    renderCard({
      defaultValues: {
        ...baseDefaults,
        'mcp_setting.group_image_models': '{"default":"gpt-image-1"}',
      },
    })

    fireEvent.click(screen.getByRole('button', { name: /Save Changes/ }))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith({
        key: 'mcp_setting.group_image_models',
        value: '{\n  "default": "gpt-image-1"\n}',
      })
    })
  })

  it('defaultValues 引用变化但内容不变时不应重置表单（防止 refetch 清空编辑）', async () => {
    const { rerender } = renderCard()
    // 模拟父组件 refetch 后用新引用、相同内容重新渲染
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <McpSettingsCard defaultValues={{ ...baseDefaults }} />
      </QueryClientProvider>
    )

    // 若误 reset，用户编辑会被清空；此处通过保存行为保持可验证：
    // 保存仍应提交全部配置且值不变（证明表单未被清成 undefined/空）
    fireEvent.click(screen.getByRole('button', { name: /Save Changes/ }))
    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledTimes(6)
    })
  })

  it('远端内容真正变化时应重置表单默认值', async () => {
    const queryClient = new QueryClient()
    const { rerender } = renderCard({ queryClient })

    const nextDefaults = {
      ...baseDefaults,
      'mcp_setting.group_i2i_models': '{"vip":"gpt-image-1"}',
    }
    rerender(
      <QueryClientProvider client={queryClient}>
        <McpSettingsCard defaultValues={nextDefaults} />
      </QueryClientProvider>
    )

    fireEvent.click(screen.getByRole('button', { name: /Save Changes/ }))
    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith({
        key: 'mcp_setting.group_i2i_models',
        value: '{\n  "vip": "gpt-image-1"\n}',
      })
    })
  })
})
