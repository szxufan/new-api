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
import { type Row } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Model } from '../types'
import { DataTableRowActions } from './data-table-row-actions'
import * as provider from './models-provider'

vi.mock('./models-provider', () => ({
  useModels: vi.fn(),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function makeModel(): Model {
  return {
    id: 7,
    model_name: 'row-action-model',
    name_rule: 0,
    status: 1,
    sync_official: 1,
    created_time: 0,
    updated_time: 0,
  }
}

describe('DataTableRowActions', () => {
  const setOpen = vi.fn()
  const setCurrentRow = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(provider.useModels).mockReturnValue({
      setOpen,
      setCurrentRow,
    } as unknown as ReturnType<typeof provider.useModels>)
  })

  function renderRowActions(model: Model) {
    return render(
      <QueryClientProvider client={new QueryClient()}>
        <DataTableRowActions row={{ original: model } as Row<Model>} />
      </QueryClientProvider>
    )
  }

  it('点击「复制新增」应以当前行作为源模型打开复制侧栏', async () => {
    const user = userEvent.setup()
    const model = makeModel()
    renderRowActions(model)

    await user.click(screen.getByRole('button', { name: 'Open menu' }))
    await user.click(
      await screen.findByRole('menuitem', { name: 'Copy as New' })
    )

    expect(setCurrentRow).toHaveBeenCalledTimes(1)
    expect(setCurrentRow).toHaveBeenCalledWith(model)
    expect(setOpen).toHaveBeenCalledTimes(1)
    expect(setOpen).toHaveBeenCalledWith('copy-model')
  })
})
