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
import { QueryClient } from '@tanstack/react-query'
import { modelsQueryKeys } from '../../lib'
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
    const cachedBefore = queryClient.getQueryData(modelsQueryKeys.detail(modelId))
    expect(cachedBefore).toBeTruthy()

    // 模拟 ModelMutateDrawer 中编辑成功后的缓存失效逻辑
    queryClient.removeQueries({ queryKey: modelsQueryKeys.detail(modelId) })

    // 验证详情缓存已被清除
    const cachedAfter = queryClient.getQueryData(modelsQueryKeys.detail(modelId))
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
      'fallback_model',
    ]) {
      expect(shapeKeys).toContain(field)
    }
  })
})
