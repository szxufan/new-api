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
import { buildModelNameOptions } from './model-name-options'

describe('buildModelNameOptions', () => {
  it('returns empty array for undefined or empty channel models', () => {
    expect(buildModelNameOptions(undefined, new Set())).toEqual([])
    expect(buildModelNameOptions([], new Set())).toEqual([])
  })

  it('builds options with value equal to label', () => {
    const options = buildModelNameOptions(['gpt-4', 'claude-3'], new Set())
    expect(options).toEqual([
      { value: 'claude-3', label: 'claude-3' },
      { value: 'gpt-4', label: 'gpt-4' },
    ])
  })

  it('excludes models that already have pricing', () => {
    const priced = new Set(['gpt-4'])
    const options = buildModelNameOptions(['gpt-4', 'gpt-4o'], priced)
    expect(options).toEqual([{ value: 'gpt-4o', label: 'gpt-4o' }])
  })

  it('returns empty array when all models already have pricing', () => {
    const priced = new Set(['gpt-4', 'claude-3'])
    expect(buildModelNameOptions(['gpt-4', 'claude-3'], priced)).toEqual([])
  })

  it('trims whitespace and skips empty entries', () => {
    const options = buildModelNameOptions(
      ['  gpt-4  ', '', '   ', 'claude-3'],
      new Set()
    )
    expect(options).toEqual([
      { value: 'claude-3', label: 'claude-3' },
      { value: 'gpt-4', label: 'gpt-4' },
    ])
  })

  it('deduplicates repeated model names', () => {
    const options = buildModelNameOptions(
      ['gpt-4', 'gpt-4', ' gpt-4 ', 'claude-3'],
      new Set()
    )
    expect(options).toEqual([
      { value: 'claude-3', label: 'claude-3' },
      { value: 'gpt-4', label: 'gpt-4' },
    ])
  })

  it('sorts options by locale order', () => {
    const options = buildModelNameOptions(['zeta', 'alpha', 'mid'], new Set())
    expect(options.map((o) => o.value)).toEqual(['alpha', 'mid', 'zeta'])
  })

  it('matches priced names case-sensitively (exact identifiers)', () => {
    const priced = new Set(['GPT-4'])
    const options = buildModelNameOptions(['gpt-4', 'GPT-4'], priced)
    expect(options).toEqual([{ value: 'gpt-4', label: 'gpt-4' }])
  })
})
