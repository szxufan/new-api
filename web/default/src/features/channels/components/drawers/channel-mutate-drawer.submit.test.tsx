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
import { useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../../api'
import type { Channel } from '../../types'
import { ChannelMutateDrawer } from './channel-mutate-drawer'

vi.mock('../../api', () => ({
  createChannel: vi.fn(),
  fetchModels: vi.fn(),
  getAllModels: vi.fn(),
  getChannel: vi.fn(),
  getChannelKey: vi.fn(),
  getGroups: vi.fn(),
  getPrefillGroups: vi.fn(),
  refreshCodexCredential: vi.fn(),
  searchChannels: vi.fn(),
  updateChannel: vi.fn(),
}))

vi.mock('../channels-provider', () => ({
  useChannels: () => ({ setOpen: vi.fn() }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const mockToast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('@/lib/lobe-icon', () => ({ getLobeIcon: () => null }))

// 与提交路径回归无关的重型子组件一律打桩（Sheet 使用真实 Base UI 实现，
// 以覆盖"按钮 form 属性 → 表单提交"这条现有测试用 fireEvent.submit 绕过的路径）
vi.mock('@/components/json-editor', () => ({
  JsonEditor: () => null,
}))
vi.mock('../../model-mapping-editor', () => ({
  ModelMappingEditor: () => null,
}))
vi.mock('../dialogs/codex-oauth-dialog', () => ({
  CodexOAuthDialog: () => null,
}))
vi.mock('../dialogs/fetch-models-dialog', () => ({
  FetchModelsDialog: () => null,
}))
vi.mock('../dialogs/missing-models-confirmation-dialog', () => ({
  MissingModelsConfirmationDialog: () => null,
}))
vi.mock('../dialogs/param-override-editor-dialog', () => ({
  ParamOverrideEditorDialog: () => null,
}))
vi.mock('../dialogs/status-code-risk-dialog', () => ({
  StatusCodeRiskDialog: () => null,
}))
vi.mock('@/features/auth/secure-verification', () => ({
  SecureVerificationDialog: () => null,
  useSecureVerification: () => ({
    open: false,
    methods: [],
    state: {},
    executeVerification: vi.fn(),
    withVerification: (fn: (...args: unknown[]) => unknown) => fn,
    cancel: vi.fn(),
    setCode: vi.fn(),
    switchVerificationMethod: vi.fn(),
  }),
}))

function makeChannel(overrides: Partial<Channel> = {}): Channel {
  return {
    id: 1,
    name: 'Test Channel',
    type: 1,
    status: 1,
    key: 'sk-test',
    models: 'gpt-4',
    group: 'default',
    channel_info: { is_multi_key: false, multi_key_mode: 'random' },
    ...overrides,
  } as unknown as Channel
}

function renderDrawer(queryClient: QueryClient, row: Channel) {
  function Wrapper() {
    const [open, setOpen] = useState(true)
    return (
      <QueryClientProvider client={queryClient}>
        <ChannelMutateDrawer
          open={open}
          onOpenChange={setOpen}
          currentRow={row}
        />
      </QueryClientProvider>
    )
  }
  return render(<Wrapper />)
}

async function waitForFormLoaded() {
  const nameInput = await screen.findByLabelText('Name *', undefined, {
    timeout: 5000,
  })
  await waitFor(() => expect(nameInput).toHaveValue('Test Channel'))
  return nameInput
}

describe('ChannelMutateDrawer - 提交按钮点击路径', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getGroups).mockResolvedValue({ success: true, data: [] })
    vi.mocked(api.getAllModels).mockResolvedValue({ success: true, data: [] })
    vi.mocked(api.getPrefillGroups).mockResolvedValue({
      success: true,
      data: [],
    })
    vi.mocked(api.searchChannels).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, type_counts: {} },
    } as never)
    vi.mocked(api.updateChannel).mockResolvedValue({ success: true })
  })

  it('点击“Update Channel”按钮应通过 form 属性触发表单提交并发起更新请求', async () => {
    vi.mocked(api.getChannel).mockResolvedValue({
      success: true,
      data: makeChannel(),
    })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderDrawer(queryClient, makeChannel())
    await waitForFormLoaded()

    // 按钮位于 <form> 之外，依赖 form='channel-form' 属性关联提交
    const submitBtn = screen.getByRole('button', { name: 'Update Channel' })
    expect(submitBtn.getAttribute('type')).toBe('submit')
    expect(submitBtn.getAttribute('form')).toBe('channel-form')

    fireEvent.click(submitBtn)

    await waitFor(() =>
      expect(api.updateChannel).toHaveBeenCalledTimes(1)
    )
    await waitFor(() => expect(mockToast.success).toHaveBeenCalled())
  })

  it('校验失败时应输出字段级诊断日志并提示，而不是静默无反应', async () => {
    const consoleErrorSpy = vi
      .spyOn(console, 'error')
      .mockImplementation(() => {})
    // 备注超长触发 zod 校验失败（该字段位于高级设置区内）
    vi.mocked(api.getChannel).mockResolvedValue({
      success: true,
      data: makeChannel({ remark: 'x'.repeat(300) }),
    })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderDrawer(queryClient, makeChannel({ remark: 'x'.repeat(300) }))
    await waitForFormLoaded()

    fireEvent.click(
      screen.getByRole('button', { name: 'Update Channel' })
    )

    await waitFor(() =>
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[ChannelForm] validation failed',
        expect.objectContaining({
          errors: expect.arrayContaining([
            expect.objectContaining({ field: 'remark' }),
          ]),
        })
      )
    )
    await waitFor(() => expect(mockToast.error).toHaveBeenCalled())
    expect(api.updateChannel).not.toHaveBeenCalled()

    consoleErrorSpy.mockRestore()
  })
})
