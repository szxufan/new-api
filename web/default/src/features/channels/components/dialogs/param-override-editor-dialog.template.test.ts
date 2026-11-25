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
  DEEPSEEK_WEB_TOOLS_TEMPLATE,
  KIMI_K31_FIXED_PARAMS_TEMPLATE,
} from './param-override-editor-dialog'

// 模板必须同时满足四点才能直接可用：
// 1) append + as=array：tools 缺失时自动建数组，而不是让请求 400；
// 2) 每条只注入一个工具，避免客户端已有其中一个时重复声明；
// 3) 用 invert 条件去重，客户端已声明时跳过；
// 4) invert + pass_missing_key：缺失时只有 pass_missing_key 才通过，
//    否则未携带 tools 的请求反而不会被注入。
describe('DeepSeek web tools template', () => {
  const operations = DEEPSEEK_WEB_TOOLS_TEMPLATE.operations

  it('declares web search and read url in order', () => {
    expect(
      operations.map(
        (operation) => (operation.value as { type: string }[])[0].type
      )
    ).toEqual(['deepseek_web_search', 'deepseek_read_url'])
  })

  it('appends into tools with array creation for missing field', () => {
    for (const operation of operations) {
      expect(operation.mode).toBe('append')
      expect(operation.path).toBe('tools')
      expect(operation.as).toBe('array')
    }
  })

  it('injects when tools is missing and skips when already declared', () => {
    for (const operation of operations) {
      expect(operation.conditions).toHaveLength(1)
      const condition = operation.conditions[0]
      expect(condition.path).toBe('tools')
      expect(condition.mode).toBe('contains')
      expect(condition.invert).toBe(true)
      expect(condition.pass_missing_key).toBe(true)
      expect(condition.value).toBe(
        (operation.value as { type: string }[])[0].type
      )
    }
  })
})

// Kimi K3.1 系列采样参数由服务端固定，显式传入非默认值会 400。模板必须：
// 1) 覆盖全部五个固定参数；2) 用 delete 而非 set（官方建议不显式传入，
// 字段缺失时 delete 也是空操作）；3) 条件限定 k3.1 模型，避免误伤
// 同渠道温度可调的模型（如 kimi-k2.6 非思考模式固定 0.6）。
describe('Kimi K3.1 fixed params template', () => {
  const operations = KIMI_K31_FIXED_PARAMS_TEMPLATE.operations

  it('deletes all five server-fixed sampling params', () => {
    expect(operations.map((operation) => operation.path)).toEqual([
      'temperature',
      'top_p',
      'n',
      'presence_penalty',
      'frequency_penalty',
    ])
    for (const operation of operations) {
      expect(operation.mode).toBe('delete')
    }
  })

  it('only applies to k3.1 models', () => {
    for (const operation of operations) {
      expect(operation.conditions).toEqual([
        { path: 'model', mode: 'contains', value: 'k3.1' },
      ])
    }
  })
})
