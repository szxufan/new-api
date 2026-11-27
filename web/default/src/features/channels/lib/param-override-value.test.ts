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
import { describe, expect, it } from 'vitest'
import {
  describeValueText,
  isStructuredValueMode,
  parseLooseValue,
  toValueText,
} from './param-override-value'

describe('parseLooseValue', () => {
  it('returns empty string for blank input', () => {
    expect(parseLooseValue('')).toBe('')
    expect(parseLooseValue('   ')).toBe('')
  })

  it('parses valid JSON literals', () => {
    expect(parseLooseValue('0.7')).toBe(0.7)
    expect(parseLooseValue('true')).toBe(true)
    expect(parseLooseValue('null')).toBeNull()
    expect(parseLooseValue('"abc"')).toBe('abc')
    expect(parseLooseValue('{"type":"enabled"}')).toEqual({
      type: 'enabled',
    })
    expect(parseLooseValue('[{"type":"deepseek_web_search"}]')).toEqual([
      { type: 'deepseek_web_search' },
    ])
  })

  it('falls back to plain text when JSON is invalid', () => {
    expect(parseLooseValue('abc')).toBe('abc')
    expect(parseLooseValue('4K')).toBe('4K')
    expect(parseLooseValue('Bearer sk-xxx')).toBe('Bearer sk-xxx')
    expect(parseLooseValue('[{"type":"x"},]')).toBe('[{"type":"x"},]')
  })

  it('trims surrounding whitespace before parsing', () => {
    expect(parseLooseValue('  0.7  ')).toBe(0.7)
    expect(parseLooseValue('  abc  ')).toBe('abc')
  })
})

describe('describeValueText', () => {
  it('detects empty input', () => {
    expect(describeValueText('')).toEqual({ kind: 'empty' })
  })

  it('reports the JSON value type', () => {
    expect(describeValueText('0.7')).toEqual({
      kind: 'json',
      jsonType: 'number',
    })
    expect(describeValueText('true')).toEqual({
      kind: 'json',
      jsonType: 'boolean',
    })
    expect(describeValueText('null')).toEqual({
      kind: 'json',
      jsonType: 'null',
    })
    expect(describeValueText('"abc"')).toEqual({
      kind: 'json',
      jsonType: 'string',
    })
    expect(describeValueText('{"a":1}')).toEqual({
      kind: 'json',
      jsonType: 'object',
    })
    expect(describeValueText('[1,2]')).toEqual({
      kind: 'json',
      jsonType: 'array',
    })
  })

  it('reports plain text for non-JSON input', () => {
    expect(describeValueText('abc')).toEqual({ kind: 'text' })
    expect(describeValueText('4K')).toEqual({ kind: 'text' })
  })

  it('flags invalid JSON-looking input', () => {
    expect(describeValueText('[{"type":"x"},]')).toEqual({
      kind: 'invalid-json',
    })
    expect(describeValueText('{"type":')).toEqual({ kind: 'invalid-json' })
  })
})

describe('toValueText', () => {
  it('keeps plain strings unquoted', () => {
    expect(toValueText('abc')).toBe('abc')
    expect(toValueText('')).toBe('')
    expect(toValueText(undefined)).toBe('')
  })

  it('serializes non-string values', () => {
    expect(toValueText(0.7)).toBe('0.7')
    expect(toValueText(true)).toBe('true')
    expect(toValueText(null)).toBe('null')
    expect(toValueText([{ type: 'deepseek_web_search' }])).toBe(
      '[{"type":"deepseek_web_search"}]'
    )
  })

  it('quotes strings that would be reinterpreted as JSON', () => {
    expect(toValueText('0.7')).toBe('"0.7"')
    expect(toValueText('true')).toBe('"true"')
    expect(toValueText('null')).toBe('"null"')
    expect(toValueText('[1]')).toBe('"[1]"')
    expect(toValueText(' 0.7 ')).toBe('" 0.7 "')
  })

  it('round-trips every supported value through the editor text', () => {
    const values: unknown[] = [
      'abc',
      '0.7',
      'true',
      'null',
      '[1]',
      '{"a":1}',
      ' 0.7 ',
      '',
      0.7,
      true,
      null,
      [{ type: 'deepseek_web_search' }],
      { thinking: { type: 'enabled' } },
    ]
    for (const value of values) {
      expect(parseLooseValue(toValueText(value))).toEqual(value)
    }
  })
})

describe('isStructuredValueMode', () => {
  it('only treats set/append/prepend as structured value modes', () => {
    expect(isStructuredValueMode('set')).toBe(true)
    expect(isStructuredValueMode('append')).toBe(true)
    expect(isStructuredValueMode('prepend')).toBe(true)
    expect(isStructuredValueMode('trim_prefix')).toBe(false)
    expect(isStructuredValueMode('set_header')).toBe(false)
  })
})
