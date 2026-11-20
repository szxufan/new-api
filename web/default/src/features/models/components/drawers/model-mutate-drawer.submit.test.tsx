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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../../api'
import type { Model } from '../../types'
import { ModelMutateDrawer } from './model-mutate-drawer'

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

function renderDrawer(currentRow: Model | null = null) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ModelMutateDrawer open onOpenChange={vi.fn()} currentRow={currentRow} />
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
