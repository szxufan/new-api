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
import { useEffect, useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
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

// vaul 抽屉在 jsdom 中行为不稳定，用简单实现替代：关闭时卸载内容
vi.mock('@/components/ui/sheet', () => ({
  Sheet: ({ open, children }: { open: boolean; children: React.ReactNode }) =>
    open ? <div data-testid='sheet'>{children}</div> : null,
  SheetContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SheetHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SheetFooter: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SheetTitle: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SheetDescription: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SheetClose: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}))

// 与本次回归无关的重型子组件一律打桩
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
  MissingModelsConfirmationDialog: (props: {
    open: boolean
    missingModels: string[]
    onConfirm: (action: string) => void
  }) =>
    props.open ? (
      <div data-testid='missing-models-dialog'>
        <span data-testid='missing-models-list'>
          {props.missingModels.join(',')}
        </span>
        <button
          data-testid='missing-models-cancel'
          onClick={() => props.onConfirm('cancel')}
        >
          cancel
        </button>
        <button
          data-testid='missing-models-submit'
          onClick={() => props.onConfirm('submit')}
        >
          submit
        </button>
        <button
          data-testid='missing-models-add'
          onClick={() => props.onConfirm('add')}
        >
          add
        </button>
      </div>
    ) : null,
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
    switchMethod: vi.fn(),
  }),
}))

const baseChannel: Channel = {
  id: 1,
  name: 'Test Channel',
  type: 1,
  status: 1,
  key: 'sk-test',
  models: 'gpt-4',
  group: 'default',
  channel_info: { is_multi_key: false, multi_key_mode: 'random' },
} as unknown as Channel

const updatedChannel: Channel = {
  ...baseChannel,
  name: 'Updated Channel',
}

function renderDrawer(queryClient: QueryClient, row: Channel = baseChannel) {
  const setOpenRef: { current: ((open: boolean) => void) | null } = {
    current: null,
  }

  function Wrapper() {
    const [open, setOpen] = useState(true)
    useEffect(() => {
      setOpenRef.current = setOpen
    }, [])
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

  const utils = render(<Wrapper />)
  return { ...utils, reopen: () => act(() => setOpenRef.current?.(true)) }
}

describe('ChannelMutateDrawer - 更新后重新打开的数据新鲜度回归测试', () => {
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
  })

  it('更新渠道成功后再次打开编辑侧栏，应展示最新配置而不是旧缓存', async () => {
    // 模拟服务端数据：更新成功后详情接口返回新名称
    let channelUpdated = false
    vi.mocked(api.getChannel).mockImplementation(async () => ({
      success: true,
      data: channelUpdated ? updatedChannel : baseChannel,
    }))
    vi.mocked(api.updateChannel).mockImplementation(async () => {
      channelUpdated = true
      return { success: true }
    })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const { reopen } = renderDrawer(queryClient)

    // 首次打开：加载并展示旧名称
    const nameInput = await screen.findByLabelText('Name *')
    await waitFor(() =>
      expect(nameInput).toHaveValue('Test Channel')
    )
    expect(api.getChannel).toHaveBeenCalledTimes(1)

    // 提交更新
    const form = document.getElementById('channel-form') as HTMLFormElement
    fireEvent.submit(form)
    await waitFor(() => expect(api.updateChannel).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(mockToast.success).toHaveBeenCalled())

    // 提交成功后侧栏关闭
    await waitFor(() =>
      expect(screen.queryByTestId('sheet')).not.toBeInTheDocument()
    )

    // 再次打开编辑侧栏：必须重新拉取并展示更新后的配置
    reopen()
    const nameInputAfterReopen = await screen.findByLabelText('Name *')
    await waitFor(() =>
      expect(nameInputAfterReopen).toHaveValue('Updated Channel')
    )
    // 详情缓存已失效，必须重新请求详情接口
    expect(vi.mocked(api.getChannel).mock.calls.length).toBeGreaterThan(1)
  })
})

describe('ChannelMutateDrawer - model_mapping 缺失模型自动追加', () => {
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

  it('models 为空且 model_mapping 有缺失模型：选择追加则补全并保存', async () => {
    const channel: Channel = {
      ...baseChannel,
      models: '',
      model_mapping: JSON.stringify({ 'gpt-4': 'gpt-4-turbo' }),
    } as Channel
    vi.mocked(api.getChannel).mockResolvedValue({ success: true, data: channel })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderDrawer(queryClient, channel)

    // 等待表单加载完成（确保 form.reset 已执行，model_mapping 已填入）
    const nameInput = await screen.findByLabelText('Name *')
    await waitFor(() => expect(nameInput).toHaveValue('Test Channel'))

    const form = document.getElementById('channel-form') as HTMLFormElement
    fireEvent.submit(form)

    // models 为空但有可自动追加的模型时，应弹出确认对话框
    const dialog = await screen.findByTestId('missing-models-dialog')
    expect(dialog).toBeInTheDocument()
    expect(screen.getByTestId('missing-models-list').textContent).toBe('gpt-4')

    // 选择追加：补全 models 后正常保存
    fireEvent.click(screen.getByTestId('missing-models-add'))
    await waitFor(() =>
      expect(api.updateChannel).toHaveBeenCalledTimes(1)
    )
    const payload = vi.mocked(api.updateChannel).mock.calls[0][1]
    expect(payload.models).toBe('gpt-4')
  })

  it('models 为空且 model_mapping 有缺失模型：选择直接提交则正常保存', async () => {
    const channel: Channel = {
      ...baseChannel,
      models: '',
      model_mapping: JSON.stringify({ 'gpt-4': 'gpt-4-turbo' }),
    } as Channel
    vi.mocked(api.getChannel).mockResolvedValue({ success: true, data: channel })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderDrawer(queryClient, channel)

    const nameInput2 = await screen.findByLabelText('Name *')
    await waitFor(() => expect(nameInput2).toHaveValue('Test Channel'))
    const form = document.getElementById('channel-form') as HTMLFormElement
    fireEvent.submit(form)

    await screen.findByTestId('missing-models-dialog')
    // 用户拒绝追加：models 仍为空，但不再阻止空 models 提交，应正常保存
    fireEvent.click(screen.getByTestId('missing-models-submit'))

    await waitFor(() =>
      expect(api.updateChannel).toHaveBeenCalledTimes(1)
    )
    const payload = vi.mocked(api.updateChannel).mock.calls[0][1]
    // 空字符串 models 在 payload 中被清理为 null
    expect(payload.models).toBe(null)
  })

  it('未做修改且 model_mapping 有缺失模型：选择直接提交则正常保存', async () => {
    const channel: Channel = {
      ...baseChannel,
      models: 'gpt-3.5-turbo',
      model_mapping: JSON.stringify({ 'gpt-4': 'gpt-4-turbo' }),
    } as Channel
    vi.mocked(api.getChannel).mockResolvedValue({ success: true, data: channel })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderDrawer(queryClient, channel)

    const nameInput3 = await screen.findByLabelText('Name *')
    await waitFor(() => expect(nameInput3).toHaveValue('Test Channel'))
    const form = document.getElementById('channel-form') as HTMLFormElement
    // 不修改任何字段直接提交
    fireEvent.submit(form)

    // 即使未修改，只要有可自动追加的模型也应弹出对话框
    await screen.findByTestId('missing-models-dialog')
    // 用户拒绝追加：models 非空，应正常保存（与原“未修改正常保存”一致）
    fireEvent.click(screen.getByTestId('missing-models-submit'))

    await waitFor(() =>
      expect(api.updateChannel).toHaveBeenCalledTimes(1)
    )
    const payload = vi.mocked(api.updateChannel).mock.calls[0][1]
    expect(payload.models).toBe('gpt-3.5-turbo')
  })

  it('models 为空且无 model_mapping：直接正常保存', async () => {
    const channel: Channel = {
      ...baseChannel,
      models: '',
      model_mapping: '',
    } as Channel
    vi.mocked(api.getChannel).mockResolvedValue({ success: true, data: channel })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    renderDrawer(queryClient, channel)

    const nameInput4 = await screen.findByLabelText('Name *')
    await waitFor(() => expect(nameInput4).toHaveValue('Test Channel'))
    const form = document.getElementById('channel-form') as HTMLFormElement
    fireEvent.submit(form)

    // 没有 model_mapping 不会弹窗，models 为空也允许直接保存
    await waitFor(() =>
      expect(screen.queryByTestId('missing-models-dialog')).not.toBeInTheDocument()
    )
    await waitFor(() =>
      expect(api.updateChannel).toHaveBeenCalledTimes(1)
    )
    const payload = vi.mocked(api.updateChannel).mock.calls[0][1]
    // 空字符串 models 在 payload 中被清理为 null
    expect(payload.models).toBe(null)
  })
})
