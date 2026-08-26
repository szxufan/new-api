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
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Loader2Icon, SparklesIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { getUserModels, optimizePrompt } from '../api'
import {
  buildOptimizePayload,
  extractOptimizedPrompt,
  getOptimizeErrorMessage,
} from '../lib/prompt-optimizer'
import type { PromptOptimizeType } from '../types'

const TEXT_MODELS_QUERY_KEY = ['image-debug-text-models'] as const

interface PromptOptimizerProps {
  /** 待优化的媒体类型：图像 / 视频 */
  type: PromptOptimizeType
  /** 当前提示词内容 */
  prompt: string
  /** 使用分组（复用表单当前选择的分组） */
  group: string
  /** 生成任务提交中时禁用 */
  disabled?: boolean
  /** 优化成功后的回调，参数为优化后的提示词 */
  onOptimized: (newPrompt: string) => void
}

/**
 * 提示词一键 AI 优化：用户选择文本模型后，
 * 经 /pg/chat/completions 对当前提示词进行润色并直接替换。
 */
export function PromptOptimizer(props: PromptOptimizerProps) {
  const { t } = useTranslation()
  const [optimizeModel, setOptimizeModel] = useState('')
  const [isOptimizing, setIsOptimizing] = useState(false)

  const { data: models = [], isLoading: isModelLoading } = useQuery({
    queryKey: TEXT_MODELS_QUERY_KEY,
    queryFn: () => getUserModels(),
  })

  // 未选择时回退到第一个可用模型
  const selectedModel =
    optimizeModel || (models.length > 0 ? models[0].value : '')

  const handleOptimize = async () => {
    if (props.disabled || isOptimizing) return
    if (props.prompt.trim() === '' || selectedModel === '') return
    setIsOptimizing(true)
    try {
      const res = await optimizePrompt(
        buildOptimizePayload({
          prompt: props.prompt,
          type: props.type,
          model: selectedModel,
          group: props.group,
        })
      )
      if (res.error?.message) {
        throw new Error(res.error.message)
      }
      const optimized = extractOptimizedPrompt(res)
      if (!optimized) {
        toast.error(t('Failed to optimize prompt'))
        return
      }
      props.onOptimized(optimized)
      toast.success(t('Prompt optimized successfully'))
    } catch (err) {
      toast.error(getOptimizeErrorMessage(err, t('Failed to optimize prompt')))
    } finally {
      setIsOptimizing(false)
    }
  }

  const canOptimize =
    !props.disabled &&
    !isOptimizing &&
    !isModelLoading &&
    models.length > 0 &&
    props.prompt.trim() !== '' &&
    selectedModel !== ''

  return (
    <div className='flex flex-wrap items-center gap-2'>
      {models.length === 0 && !isModelLoading && (
        <span className='text-muted-foreground text-xs'>
          {t('No text models available')}
        </span>
      )}
      <NativeSelect
        size='sm'
        aria-label={t('Optimization model')}
        value={selectedModel}
        onChange={(e) => setOptimizeModel(e.target.value)}
        disabled={props.disabled || isOptimizing || isModelLoading}
        className={cn('max-w-44', models.length === 0 && 'hidden')}
      >
        {models.map((model) => (
          <NativeSelectOption key={model.value} value={model.value}>
            {model.label}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      <Button
        type='button'
        size='sm'
        variant='outline'
        onClick={handleOptimize}
        disabled={!canOptimize}
      >
        {isOptimizing ? (
          <Loader2Icon className='size-3.5 animate-spin' aria-hidden='true' />
        ) : (
          <SparklesIcon className='size-3.5' aria-hidden='true' />
        )}
        {isOptimizing ? t('Optimizing...') : t('AI Optimize')}
      </Button>
    </div>
  )
}
