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
/**
 * 参数覆盖 Value 输入框的文本 <-> 值转换规则。
 *
 * 规则：文本去掉首尾空白后，若能被 JSON.parse 解析成功就按 JSON 值使用，
 * 否则原样作为字符串发送。解析失败时不会报错，因此需要 describeValueText
 * 告诉用户当前文本最终会以什么类型发往上游。
 */

export type LooseValueKind = 'empty' | 'json' | 'text' | 'invalid-json'

export type JsonValueType =
  'array' | 'object' | 'number' | 'boolean' | 'null' | 'string'

export type LooseValueDescription = {
  kind: LooseValueKind
  jsonType?: JsonValueType
}

export const parseLooseValue = (valueText: string): unknown => {
  const raw = String(valueText ?? '').trim()
  if (raw === '') return ''
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

const getJsonValueType = (value: unknown): JsonValueType => {
  if (Array.isArray(value)) return 'array'
  if (value === null) return 'null'
  switch (typeof value) {
    case 'object':
      return 'object'
    case 'number':
      return 'number'
    case 'boolean':
      return 'boolean'
    default:
      return 'string'
  }
}

export const describeValueText = (valueText: string): LooseValueDescription => {
  const raw = String(valueText ?? '').trim()
  if (raw === '') return { kind: 'empty' }
  try {
    return { kind: 'json', jsonType: getJsonValueType(JSON.parse(raw)) }
  } catch {
    // 以 { 或 [ 开头通常意味着用户想写 JSON，语法错误要显式提醒，
    // 否则会被静默当作纯文本发给上游。
    if (raw.startsWith('{') || raw.startsWith('[')) {
      return { kind: 'invalid-json' }
    }
    return { kind: 'text' }
  }
}

/**
 * 反向转换为编辑器文本，保证 parseLooseValue(toValueText(v)) 与 v 等价。
 * 字符串若本身能被 JSON 解析（如 "0.7"、"true"、"[1]"），必须加引号输出，
 * 否则重新打开编辑器再保存会被悄悄改写成数字/布尔/数组。
 */
export const toValueText = (value: unknown): string => {
  if (value === undefined) return ''
  if (typeof value === 'string') {
    const reparsed = parseLooseValue(value)
    return reparsed === value ? value : JSON.stringify(value)
  }
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

export const isStructuredValueMode = (mode: string): boolean =>
  mode === 'set' || mode === 'append' || mode === 'prepend'
