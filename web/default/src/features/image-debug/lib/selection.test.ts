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
import { describe, it, expect } from 'vitest'
import { getModeEndpoints, resolveSelection } from './selection'

describe('getModeEndpoints', () => {
  it('文生图模式返回 image-generation 端点', () => {
    expect(getModeEndpoints('generations')).toEqual(['image-generation'])
  })

  it('图生图模式返回 image-edit 端点', () => {
    expect(getModeEndpoints('edits')).toEqual(['image-edit'])
  })
})

describe('resolveSelection', () => {
  const options = [{ value: 'default' }, { value: 'vip' }]

  it('选中值在选项中时保持不变', () => {
    expect(resolveSelection('vip', options, '')).toBe('vip')
  })

  it('选中值不在选项中时回退到第一个选项', () => {
    expect(resolveSelection('svip', options, '')).toBe('default')
  })

  it('选项为空时返回 fallback', () => {
    expect(resolveSelection('svip', [], 'default')).toBe('default')
    expect(resolveSelection('', [], 'default')).toBe('default')
  })

  it('选中值为空时回退到第一个选项', () => {
    expect(resolveSelection('', options, 'fallback')).toBe('default')
  })
})