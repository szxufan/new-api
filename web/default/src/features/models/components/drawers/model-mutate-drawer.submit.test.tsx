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
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../../api'
import type { GetModelResponse, Model } from '../../types'
import {
  MODEL_COPY_NAME_SUFFIX,
  ModelMutateDrawer,
} from './model-mutate-drawer'

vi.mock('../../api', () => ({
  createModel: vi.fn(),
  getModel: vi.fn(),
  getVendors: vi.fn(),
  updateModel: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

const mockToast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: mockToast }))

vi.mock('@/components/json-editor', () => ({
  JsonEditor: () => null,
}))

function makeModel(overrides: Partial<Model> = {}): Model {
  return {
    id: 7,
    model_name: 'purpose-model',
    description: '',
    icon: '',
    tags: '',
    endpoints: '',
    name_rule: 0,
    status: 1,
    sync_official: 1,
    prompt_optimize: 0,
    fallback_model: '',
    context_length: 0,
    max_output_tokens: 0,
    support_image_input: false,
    support_video_input: false,
    reasoning_options: '',
    purposes: '',
    created_time: 0,
    updated_time: 0,
    ...overrides,
  }
}

function renderDrawer(currentRow: Model | null = null, duplicate = false) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ModelMutateDrawer
        open
        onOpenChange={vi.fn()}
        currentRow={currentRow}
        duplicate={duplicate}
      />
    </QueryClientProvider>
  )
}

describe('ModelMutateDrawer - 模型用途提交', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getVendors).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 1000 },
    })
    vi.mocked(api.createModel).mockResolvedValue({ success: true })
    vi.mocked(api.updateModel).mockResolvedValue({ success: true })
  })

  it('创建模型时应提交多选用途的 JSON 数组字符串', async () => {
    const user = userEvent.setup()
    renderDrawer()

    const nameInput = await screen.findByLabelText('Model Name *')
    await user.type(nameInput, 'purpose-model')
    await user.click(screen.getByPlaceholderText('Select purposes...'))
    await user.click(await screen.findByRole('option', { name: 'Chat' }))
    await user.click(screen.getByRole('option', { name: 'Programming' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(api.createModel).toHaveBeenCalledTimes(1))
    expect(api.createModel).toHaveBeenCalledWith(
      expect.objectContaining({
        model_name: 'purpose-model',
        purposes: '["chat","programming"]',
      })
    )
    expect(mockToast.error).not.toHaveBeenCalled()
  })

  it('编辑时应回显已知及历史用途，清空后应提交空字符串', async () => {
    const user = userEvent.setup()
    const model = makeModel({
      purposes: '["chat","legacy-purpose"]',
    })
    vi.mocked(api.getModel).mockResolvedValue({ success: true, data: model })
    renderDrawer(model)

    const nameInput = await screen.findByLabelText('Model Name *')
    await waitFor(() => expect(nameInput).toHaveValue('purpose-model'))
    expect(await screen.findByText('Chat')).toBeInTheDocument()
    expect(screen.getByText('legacy-purpose')).toBeInTheDocument()

    for (const button of screen.getAllByRole('button', {
      name: /^Remove$/,
    })) {
      await user.click(button)
    }
    expect(
      screen.getByPlaceholderText('Select purposes...')
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Update Model' }))

    await waitFor(() => expect(api.updateModel).toHaveBeenCalledTimes(1))
    expect(api.updateModel).toHaveBeenCalledWith(
      expect.objectContaining({ id: 7, purposes: '' })
    )
    expect(mockToast.error).not.toHaveBeenCalled()
  })
})

describe('ModelMutateDrawer - 复制新增', () => {
  const endpoints = '{"openai":{"path":"/v1/chat/completions","method":"POST"}}'
  const copiedName = `purpose-model${MODEL_COPY_NAME_SUFFIX}`

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getVendors).mockResolvedValue({
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 1000 },
    })
    vi.mocked(api.createModel).mockResolvedValue({ success: true })
    vi.mocked(api.updateModel).mockResolvedValue({ success: true })
  })

  it('复制时应回显源模型全部配置、名称追加后缀，并以新建（不带 id）方式提交', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getModel).mockResolvedValue({
      success: true,
      data: makeModel({
        description: 'source description',
        icon: 'openai',
        tags: 'chat,vision',
        vendor_id: 3,
        endpoints,
        name_rule: 1,
        status: 0,
        sync_official: 0,
        prompt_optimize: 1,
        fallback_model: 'gpt-4o',
        context_length: 128000,
        max_output_tokens: 4096,
        support_image_input: true,
        reasoning_options: '["low","high"]',
        purposes: '["chat"]',
      }),
    })
    renderDrawer(makeModel(), true)

    const nameInput = await screen.findByLabelText('Model Name *')
    await waitFor(() => expect(nameInput).toHaveValue(copiedName))
    expect(screen.getByText('Copy Model')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save changes' })
    ).toBeInTheDocument()

    await user.clear(nameInput)
    await user.type(nameInput, 'purpose-model-v2')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(api.createModel).toHaveBeenCalledTimes(1))
    expect(api.updateModel).not.toHaveBeenCalled()
    expect(api.createModel).toHaveBeenCalledWith(
      expect.objectContaining({
        model_name: 'purpose-model-v2',
        description: 'source description',
        icon: 'openai',
        tags: 'chat,vision',
        vendor_id: 3,
        endpoints,
        name_rule: 1,
        status: 0,
        sync_official: 0,
        prompt_optimize: 1,
        fallback_model: 'gpt-4o',
        context_length: 128000,
        max_output_tokens: 4096,
        support_image_input: true,
        support_video_input: false,
        reasoning_options: '["low","high"]',
        purposes: '["chat"]',
      })
    )
    expect(vi.mocked(api.createModel).mock.calls[0][0]?.id).toBeUndefined()
    expect(mockToast.error).not.toHaveBeenCalled()
  })

  /** 让源模型详情在用例中手动返回，以便控制「回显」发生的时机 */
  function deferModelDetail() {
    let resolveDetail: (response: GetModelResponse) => void = () => {}
    vi.mocked(api.getModel).mockReturnValue(
      new Promise<GetModelResponse>((resolve) => {
        resolveDetail = resolve
      })
    )
    return (response: GetModelResponse) =>
      act(async () => {
        resolveDetail(response)
      })
  }

  it('回显时焦点在模型名称输入框，应全选预填名称以便直接输入新名称', async () => {
    const user = userEvent.setup()
    const resolveDetail = deferModelDetail()
    renderDrawer(makeModel(), true)

    const nameInput = (await screen.findByLabelText(
      'Model Name *'
    )) as HTMLInputElement
    expect(nameInput).toHaveValue('')
    // 侧栏打开后的默认初始焦点落在模型名称输入框
    await user.click(nameInput)
    expect(nameInput).toHaveFocus()

    await resolveDetail({ success: true, data: makeModel() })

    await waitFor(() => expect(nameInput).toHaveValue(copiedName))
    await waitFor(() => {
      expect(nameInput).toHaveFocus()
      expect(nameInput.selectionStart).toBe(0)
      expect(nameInput.selectionEnd).toBe(copiedName.length)
    })
  })

  it('回显时焦点在其它字段，不应把焦点抢到模型名称输入框', async () => {
    const user = userEvent.setup()
    const resolveDetail = deferModelDetail()
    renderDrawer(makeModel(), true)

    const descriptionInput = await screen.findByPlaceholderText(
      'Describe this model...'
    )
    await user.click(descriptionInput)

    await resolveDetail({ success: true, data: makeModel() })

    const nameInput = screen.getByLabelText('Model Name *') as HTMLInputElement
    await waitFor(() => expect(nameInput).toHaveValue(copiedName))
    expect(nameInput).not.toHaveFocus()
    expect(descriptionInput).toHaveFocus()
  })
})
