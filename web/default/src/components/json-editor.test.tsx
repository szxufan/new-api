import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { JsonEditor } from './json-editor'
import {
  buildEditorParseState,
  shouldOpenInJsonMode,
} from './json-editor-utils'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

describe('buildEditorParseState', () => {
  it('returns empty rows for blank input', () => {
    expect(buildEditorParseState('')).toEqual({
      rows: [],
      invalidJson: false,
      unsupportedRoot: false,
    })
  })

  it('parses flat object into rows', () => {
    const { rows, invalidJson } = buildEditorParseState(
      '{"temperature":0.7,"top_p":1}'
    )
    expect(invalidJson).toBe(false)
    expect(rows.map((row) => row.key)).toEqual(['temperature', 'top_p'])
    expect(rows.map((row) => row.value)).toEqual(['0.7', '1'])
  })

  it('keeps nested values in a single row', () => {
    const { rows, invalidJson } = buildEditorParseState(
      '{"operations":[{"path":"tools","mode":"append"}]}'
    )
    expect(invalidJson).toBe(false)
    expect(rows).toHaveLength(1)
    expect(rows[0].key).toBe('operations')
    expect(JSON.parse(rows[0].value)).toEqual([
      { path: 'tools', mode: 'append' },
    ])
  })

  it('flags syntax errors separately from non-object roots', () => {
    const syntaxError = buildEditorParseState('{"a":')
    expect(syntaxError.invalidJson).toBe(true)
    expect(syntaxError.unsupportedRoot).toBe(false)

    for (const value of ['[1,2]', '"text"', '42', 'null']) {
      const state = buildEditorParseState(value)
      expect(state.invalidJson).toBe(false)
      expect(state.unsupportedRoot).toBe(true)
    }
  })

  it('keeps string values intact for valueType=any', () => {
    const { rows } = buildEditorParseState(
      '{"stop":"true","top_p":"0.7"}',
      'any'
    )
    expect(rows.map((row) => row.value)).toEqual(['"true"', '"0.7"'])
    expect(rows.map((row) => JSON.parse(row.value))).toEqual(['true', '0.7'])
  })

  it('keeps plain text for valueType=string', () => {
    const { rows } = buildEditorParseState('{"400":"500"}', 'string')
    expect(rows[0].value).toBe('500')
  })
})

describe('shouldOpenInJsonMode', () => {
  it('opens JSON mode for nested values', () => {
    expect(shouldOpenInJsonMode('{"operations":[{"path":"tools"}]}')).toBe(true)
    expect(shouldOpenInJsonMode('{"thinking":{"type":"enabled"}}')).toBe(true)
  })

  it('keeps visual mode for flat scalar values', () => {
    expect(shouldOpenInJsonMode('{"temperature":0.7,"max_tokens":2000}')).toBe(
      false
    )
    expect(shouldOpenInJsonMode('')).toBe(false)
  })

  it('opens JSON mode for valid non-object roots instead of flagging them invalid', () => {
    expect(shouldOpenInJsonMode('[1,2]')).toBe(true)
    expect(shouldOpenInJsonMode('"text"')).toBe(true)
    expect(shouldOpenInJsonMode('42')).toBe(true)
    expect(shouldOpenInJsonMode('null')).toBe(true)
    // 无法解析的文本仍交给可视化模式的 invalid 提示处理
    expect(shouldOpenInJsonMode('{"a":')).toBe(false)
  })
})

describe('JsonEditor', () => {
  it('renders existing flat config as rows instead of the empty state', () => {
    render(
      <JsonEditor
        value='{"temperature":0.7}'
        onChange={() => {}}
        emptyMessage='EMPTY'
      />
    )

    expect(screen.queryByText('EMPTY')).not.toBeInTheDocument()
    expect(screen.getByDisplayValue('temperature')).toBeInTheDocument()
    expect(screen.getByDisplayValue('0.7')).toBeInTheDocument()
  })

  it('opens nested config in JSON mode with the original text', () => {
    const value = '{"operations":[{"path":"tools","mode":"append"}]}'
    render(
      <JsonEditor value={value} onChange={() => {}} emptyMessage='EMPTY' />
    )

    expect(screen.queryByText('EMPTY')).not.toBeInTheDocument()
    expect(screen.getByDisplayValue(value)).toBeInTheDocument()
  })

  it('shows the empty state only when there is no value', () => {
    render(<JsonEditor value='' onChange={() => {}} emptyMessage='EMPTY' />)
    expect(screen.getByText('EMPTY')).toBeInTheDocument()
  })

  // 抽屉异步加载渠道配置时，value 从空变为已有配置，必须立即渲染出内容。
  it('renders config that arrives after mount', () => {
    const { rerender } = render(
      <JsonEditor value='' onChange={() => {}} emptyMessage='EMPTY' />
    )
    expect(screen.getByText('EMPTY')).toBeInTheDocument()

    rerender(
      <JsonEditor
        value='{"temperature":0.7}'
        onChange={() => {}}
        emptyMessage='EMPTY'
      />
    )
    expect(screen.queryByText('EMPTY')).not.toBeInTheDocument()
    expect(screen.getByDisplayValue('temperature')).toBeInTheDocument()
    expect(screen.getByDisplayValue('0.7')).toBeInTheDocument()
  })

  it('shows an error instead of the empty state for invalid JSON', () => {
    render(
      <JsonEditor value='{"a":' onChange={() => {}} emptyMessage='EMPTY' />
    )

    expect(screen.queryByText('EMPTY')).not.toBeInTheDocument()
    expect(
      screen.getByText('Invalid JSON. Switch to JSON mode to fix it.')
    ).toBeInTheDocument()
  })

  it('explains the visual limitation for non-object roots', () => {
    render(
      <JsonEditor value='[1,2]' onChange={() => {}} emptyMessage='EMPTY' />
    )

    expect(screen.getByDisplayValue('[1,2]')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Visual Mode/ }))

    expect(
      screen.getByText(
        'Top-level value is not an object. Switch to JSON mode to edit it.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByText('EMPTY')).not.toBeInTheDocument()
  })

  it('preserves string values when editing rows with valueType=any', () => {
    const onChange = vi.fn()
    render(
      <JsonEditor
        value='{"stop":"true"}'
        onChange={onChange}
        emptyMessage='EMPTY'
        valueType='any'
      />
    )

    fireEvent.change(screen.getByDisplayValue('"true"'), {
      target: { value: '"true!"' },
    })

    const calls = onChange.mock.calls
    const lastPayload = calls[calls.length - 1]?.[0]
    expect(typeof lastPayload).toBe('string')
    expect(JSON.parse(lastPayload as string)).toEqual({ stop: 'true!' })
  })
})
