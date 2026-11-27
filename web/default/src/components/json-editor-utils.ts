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
export type EditorValueType = 'string' | 'number' | 'any'

export type EditorRow = {
  id: string
  key: string
  value: string
}

export type EditorParseError = 'none' | 'syntax' | 'unsupported-root'

export type EditorParseState = {
  rows: EditorRow[]
  invalidJson: boolean
  unsupportedRoot: boolean
}

export const getEditorParseError = (
  state: EditorParseState
): EditorParseError => {
  if (state.invalidJson) return 'syntax'
  if (state.unsupportedRoot) return 'unsupported-root'
  return 'none'
}

// 行文本必须能被 convertRowsToJson 还原成同类型的值。
// valueType=any 时统一使用 JSON 文本，否则字符串 "true" / "0.7" 在行内编辑后
// 会被 JSON.parse 成布尔 / 数字，导致静默改掉发往上游的参数类型。
const formatRowValue = (val: unknown, valueType: EditorValueType): string => {
  if (valueType === 'string') {
    return typeof val === 'object' ? JSON.stringify(val) : String(val)
  }
  if (valueType === 'number') {
    return String(val)
  }
  try {
    return JSON.stringify(val) ?? String(val)
  } catch {
    return String(val)
  }
}

// 把 JSON 文本解析为可视化行。顶层必须是对象，否则可视化模式无法表达。
export function buildEditorParseState(
  json: string,
  valueType: EditorValueType = 'string'
): EditorParseState {
  const trimmed = json.trim()
  if (!trimmed) {
    return { rows: [], invalidJson: false, unsupportedRoot: false }
  }
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      // 合法 JSON，但顶层不是对象：交给 JSON 模式，不要误报为语法错误。
      return { rows: [], invalidJson: false, unsupportedRoot: true }
    }
    const rows = Object.entries(parsed as Record<string, unknown>).map(
      ([key, val], index) => ({
        id: `${Date.now()}-${index}`,
        key,
        value: formatRowValue(val, valueType),
      })
    )
    return { rows, invalidJson: false, unsupportedRoot: false }
  } catch {
    return { rows: [], invalidJson: true, unsupportedRoot: false }
  }
}

// 值里含对象/数组时（如参数覆盖的 operations 规则），单行 key-value 编辑器
// 无法可靠编辑，直接用 JSON 模式打开，避免用户改成字符串后规则静默失效。
// 顶层不是对象（数组/标量）时可视化模式同样无法表达，也直接进 JSON 模式，
// 否则合法 JSON 会被误显示为「Invalid JSON」。
export function shouldOpenInJsonMode(json: string): boolean {
  const trimmed = json.trim()
  if (!trimmed) return false
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return true
    }
    return Object.values(parsed as Record<string, unknown>).some(
      (val) => val !== null && typeof val === 'object'
    )
  } catch {
    return false
  }
}
