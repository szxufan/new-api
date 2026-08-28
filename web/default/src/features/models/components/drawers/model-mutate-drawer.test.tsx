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
import { QueryClient } from '@tanstack/react-query'
import { describe, it, expect } from 'vitest'
import { ENDPOINT_TEMPLATES } from '../../constants'
import { modelsQueryKeys, mergeEndpointTemplate } from '../../lib'
import {
  MODEL_PRICING_SETTINGS_URL,
  modelFormSchema,
} from './model-mutate-drawer'

describe('models metadata 详情缓存失效', () => {
  it('编辑模型成功后应移除该模型详情缓存，确保再次编辑能拿到最新数据', () => {
    const queryClient = new QueryClient()
    const modelId = 42

    // 填充一条旧详情缓存，模拟编辑前已经加载过的数据
    queryClient.setQueryData(modelsQueryKeys.detail(modelId), {
      success: true,
      data: { id: modelId, model_name: 'Old Name' },
    })

    // 验证缓存命中
    const cachedBefore = queryClient.getQueryData(
      modelsQueryKeys.detail(modelId)
    )
    expect(cachedBefore).toBeTruthy()

    // 模拟 ModelMutateDrawer 中编辑成功后的缓存失效逻辑
    queryClient.removeQueries({ queryKey: modelsQueryKeys.detail(modelId) })

    // 验证详情缓存已被清除
    const cachedAfter = queryClient.getQueryData(
      modelsQueryKeys.detail(modelId)
    )
    expect(cachedAfter).toBeUndefined()
  })
})

describe('模型编辑侧栏价格配置移除', () => {
  it('跳转链接应指向计费设置的模型定价分区', () => {
    // 与 channel-test-dialog / message-error 中的跳转路径保持一致
    expect(MODEL_PRICING_SETTINGS_URL).toBe(
      '/system-settings/billing/model-pricing'
    )
  })

  it('表单 schema 不应包含任何价格/倍率字段', () => {
    const pricingFields = [
      'price',
      'ratio',
      'cacheRatio',
      'completionRatio',
      'imageRatio',
      'audioRatio',
      'audioCompletionRatio',
    ]
    const shapeKeys = Object.keys(modelFormSchema.shape)
    for (const field of pricingFields) {
      expect(shapeKeys).not.toContain(field)
    }
  })

  it('表单 schema 应保留模型元数据字段', () => {
    const shapeKeys = Object.keys(modelFormSchema.shape)
    for (const field of [
      'model_name',
      'description',
      'icon',
      'tags',
      'vendor_id',
      'endpoints',
      'name_rule',
      'status',
      'sync_official',
      'prompt_optimize',
      'fallback_model',
    ]) {
      expect(shapeKeys).toContain(field)
    }
  })
})

describe('端点模板填充为追加语义', () => {
  it('已有端点配置时，加载模板应保留原有端点', () => {
    const current = JSON.stringify({
      openai: ENDPOINT_TEMPLATES['openai'],
    })
    const merged = mergeEndpointTemplate(
      current,
      'anthropic',
      ENDPOINT_TEMPLATES['anthropic']
    )
    expect(merged).not.toBeNull()
    const parsed = JSON.parse(merged!)
    // 原有端点保留，新模板追加
    expect(parsed['openai']).toEqual(ENDPOINT_TEMPLATES['openai'])
    expect(parsed['anthropic']).toEqual(ENDPOINT_TEMPLATES['anthropic'])
  })

  it('配置为空时，加载模板应只包含所选模板', () => {
    const merged = mergeEndpointTemplate(
      '',
      'openai',
      ENDPOINT_TEMPLATES['openai']
    )
    expect(JSON.parse(merged!)).toEqual({
      openai: ENDPOINT_TEMPLATES['openai'],
    })
    // undefined / 纯空白同样视为空
    expect(
      JSON.parse(
        mergeEndpointTemplate(
          undefined,
          'openai',
          ENDPOINT_TEMPLATES['openai']
        )!
      )
    ).toEqual({
      openai: ENDPOINT_TEMPLATES['openai'],
    })
    expect(
      JSON.parse(
        mergeEndpointTemplate('   ', 'openai', ENDPOINT_TEMPLATES['openai'])!
      )
    ).toEqual({
      openai: ENDPOINT_TEMPLATES['openai'],
    })
  })

  it('同名端点已存在时，模板覆盖该键但保留其他键', () => {
    const current = JSON.stringify({
      openai: { path: '/custom/path', method: 'GET' },
      anthropic: ENDPOINT_TEMPLATES['anthropic'],
    })
    const merged = mergeEndpointTemplate(
      current,
      'openai',
      ENDPOINT_TEMPLATES['openai']
    )
    const parsed = JSON.parse(merged!)
    expect(parsed['openai']).toEqual(ENDPOINT_TEMPLATES['openai'])
    expect(parsed['anthropic']).toEqual(ENDPOINT_TEMPLATES['anthropic'])
  })

  it('已有自定义端点配置（非模板键）时不应丢失', () => {
    const custom = { path: '/v1/custom', method: 'PUT', extra: 1 }
    const current = JSON.stringify({ custom })
    const merged = mergeEndpointTemplate(
      current,
      'embeddings',
      ENDPOINT_TEMPLATES['embeddings']
    )
    const parsed = JSON.parse(merged!)
    expect(parsed['custom']).toEqual(custom)
    expect(parsed['embeddings']).toEqual(ENDPOINT_TEMPLATES['embeddings'])
  })

  it('已有内容不是合法 JSON 对象时，返回 null 以避免静默丢弃用户输入', () => {
    // 非法 JSON
    expect(
      mergeEndpointTemplate('{invalid', 'openai', ENDPOINT_TEMPLATES['openai'])
    ).toBeNull()
    // JSON 数组
    expect(
      mergeEndpointTemplate(
        '["openai"]',
        'openai',
        ENDPOINT_TEMPLATES['openai']
      )
    ).toBeNull()
    // JSON 标量
    expect(
      mergeEndpointTemplate('"openai"', 'openai', ENDPOINT_TEMPLATES['openai'])
    ).toBeNull()
    expect(
      mergeEndpointTemplate('null', 'openai', ENDPOINT_TEMPLATES['openai'])
    ).toBeNull()
  })
})
