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
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GroupModelMapEditor } from './group-model-map-editor'
import type { ComboboxInputOption } from '@/components/ui/combobox-input'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

// jsdom 提供 crypto.getRandomValues 但整包替换 crypto 属性较繁琐，
// 这里直接打桩 combobox，组件交互验证不依赖其内部实现
vi.mock('@/components/ui/combobox-input', () => ({
  ComboboxInput: () => null,
}))

const groupOptions: ComboboxInputOption[] = [
  { value: 'default', label: 'default' },
  { value: 'vip', label: 'vip' },
]
const modelOptions: ComboboxInputOption[] = [
  { value: 'dall-e-3', label: 'dall-e-3' },
]

const baseProps = {
  groupOptions,
  modelOptions,
}

describe('GroupModelMapEditor', () => {
  beforeEach(() => {
    // 模拟非安全上下文（HTTP + IP 访问）：crypto.randomUUID 不存在，
    // 此前该场景下点击"添加"会抛 TypeError 导致按钮无反应
    Object.defineProperty(globalThis.crypto, 'randomUUID', {
      value: undefined,
      configurable: true,
    })
  })

  it('should add a row when Add button clicked without crypto.randomUUID', () => {
    render(<GroupModelMapEditor {...baseProps} value='{}' onChange={vi.fn()} />)

    expect(screen.getByText('No mapping configured. Click "Add" to get started.'))
      .toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Add/ }))

    // 新增空行后占位提示消失
    expect(
      screen.queryByText('No mapping configured. Click "Add" to get started.')
    ).not.toBeInTheDocument()
  })

  it('should emit "{}" for the new empty row via onChange', () => {
    const onChange = vi.fn()
    render(<GroupModelMapEditor {...baseProps} value='{}' onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: /Add/ }))

    // 空行（group 为空）会被 convertGroupModelRowsToJson 过滤，仍序列化为 '{}'
    expect(onChange).toHaveBeenCalledWith('{}')
  })

  it('should emit valid JSON when rows are deleted', () => {
    render(
      <GroupModelMapEditor
        {...baseProps}
        value='{"default":"dall-e-3"}'
        onChange={vi.fn()}
      />
    )
    // 初始渲染解析出 1 行，对应 1 个删除按钮
    const deleteButton = screen.getByRole('button', { name: /Delete/ })
    fireEvent.click(deleteButton)

    expect(screen.getByText('No mapping configured. Click "Add" to get started.'))
      .toBeInTheDocument()
  })

  it('should keep external value in sync after add (roundtrip)', () => {
    const { rerender } = render(
      <GroupModelMapEditor {...baseProps} value='{}' onChange={vi.fn()} />
    )
    fireEvent.click(screen.getByRole('button', { name: /Add/ }))
    // 父组件回写 onChange 产生的值，编辑器不应重置行数
    rerender(
      <GroupModelMapEditor {...baseProps} value='{}' onChange={vi.fn()} />
    )
    expect(
      screen.queryByText('No mapping configured. Click "Add" to get started.')
    ).not.toBeInTheDocument()
  })
})
