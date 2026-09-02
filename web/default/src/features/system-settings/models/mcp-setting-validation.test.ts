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
import {
  parseGroupModelMap,
  convertGroupModelRowsToJson,
  type GroupModelRow,
} from './mcp-setting-validation'

describe('parseGroupModelMap', () => {
  it('should parse valid JSON object to rows', () => {
    const json = '{"default":"dall-e-3","vip":"gpt-image-1"}'
    const rows = parseGroupModelMap(json)
    expect(rows).toEqual([
      { id: 'row-0-default', group: 'default', model: 'dall-e-3' },
      { id: 'row-1-vip', group: 'vip', model: 'gpt-image-1' },
    ])
  })

  it('should return empty array for empty string', () => {
    expect(parseGroupModelMap('')).toEqual([])
    expect(parseGroupModelMap('   ')).toEqual([])
  })

  it('should return empty array for empty object', () => {
    expect(parseGroupModelMap('{}')).toEqual([])
  })

  it('should return null for invalid JSON', () => {
    expect(parseGroupModelMap('{invalid}')).toBeNull()
    expect(parseGroupModelMap('not json')).toBeNull()
  })

  it('should return null for non-object JSON', () => {
    expect(parseGroupModelMap('[]')).toBeNull()
    expect(parseGroupModelMap('"string"')).toBeNull()
    expect(parseGroupModelMap('42')).toBeNull()
  })

  it('should convert non-string values to strings', () => {
    const json = '{"default": 123}'
    const rows = parseGroupModelMap(json)
    expect(rows).toEqual([{ id: 'row-0-default', group: 'default', model: '123' }])
  })

  it('should produce stable row ids', () => {
    const json = '{"default":"dall-e-3"}'
    expect(parseGroupModelMap(json)).toEqual(parseGroupModelMap(json))
  })
})

describe('convertGroupModelRowsToJson', () => {
  it('should convert rows to formatted JSON', () => {
    const rows: GroupModelRow[] = [
      { id: 'row-0-default', group: 'default', model: 'dall-e-3' },
      { id: 'row-1-vip', group: 'vip', model: 'gpt-image-1' },
    ]
    const json = convertGroupModelRowsToJson(rows)
    expect(json).toBe(
      JSON.stringify({ default: 'dall-e-3', vip: 'gpt-image-1' }, null, 2)
    )
  })

  it('should skip rows with empty group', () => {
    const rows: GroupModelRow[] = [
      { id: 'row-0', group: '', model: 'dall-e-3' },
      { id: 'row-1', group: 'vip', model: 'gpt-image-1' },
    ]
    const json = convertGroupModelRowsToJson(rows)
    expect(json).toBe(JSON.stringify({ vip: 'gpt-image-1' }, null, 2))
  })

  it('should trim group and model values', () => {
    const rows: GroupModelRow[] = [
      { id: 'row-0', group: '  vip  ', model: '  dall-e-3  ' },
    ]
    const json = convertGroupModelRowsToJson(rows)
    expect(json).toBe(JSON.stringify({ vip: 'dall-e-3' }, null, 2))
  })

  it('should return empty object for empty rows', () => {
    expect(convertGroupModelRowsToJson([])).toBe('{}')
  })

  it('should return empty object when all rows are empty', () => {
    const rows: GroupModelRow[] = [{ id: 'row-0', group: '', model: '' }]
    expect(convertGroupModelRowsToJson(rows)).toBe('{}')
  })
})

describe('parse/convert roundtrip', () => {
  it('should keep data consistent across roundtrips', () => {
    const original = { default: 'dall-e-3', vip: 'gpt-image-1' }
    const rows = parseGroupModelMap(JSON.stringify(original))
    expect(rows).not.toBeNull()
    const json = convertGroupModelRowsToJson(rows!)
    expect(JSON.parse(json)).toEqual(original)
  })
})
