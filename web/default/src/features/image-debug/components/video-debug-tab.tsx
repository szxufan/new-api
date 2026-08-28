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
import { useCallback, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ScrollArea } from '@/components/ui/scroll-area'
import { createVideo, fetchVideo, getUserGroups, getUserModels } from '../api'
import {
  DEFAULT_GROUP,
  DEFAULT_VIDEO_DURATION,
  DEFAULT_VIDEO_RATIO,
  DEFAULT_VIDEO_RESOLUTION,
  VIDEO_POLL_INTERVAL_MS,
} from '../constants'
import { resolveSelection } from '../lib/selection'
import { buildVideoPayload } from '../lib/video-payload'
import type { VideoDebugFormState, VideoResponse } from '../types'
import { VideoDebugForm } from './video-debug-form'
import { VideoResult } from './video-result'

function createDefaultState(): VideoDebugFormState {
  return {
    model: '',
    group: DEFAULT_GROUP,
    prompt: '',
    images: [],
    ratio: DEFAULT_VIDEO_RATIO,
    resolution: DEFAULT_VIDEO_RESOLUTION,
    duration: DEFAULT_VIDEO_DURATION,
  }
}

/** 视频调试端点：openai-video（异步视频任务） */
const VIDEO_ENDPOINTS = ['openai-video']

export function VideoDebugTab() {
  const { t } = useTranslation()
  const [state, setState] = useState<VideoDebugFormState>(createDefaultState)
  const [result, setResult] = useState<VideoResponse | null>(null)
  const [taskId, setTaskId] = useState<string | null>(null)
  const [isPolling, setIsPolling] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { data: models = [], isLoading: isLoadingModels } = useQuery({
    queryKey: ['image-debug-models', VIDEO_ENDPOINTS],
    queryFn: () => getUserModels(VIDEO_ENDPOINTS),
  })

  const { data: groups = [] } = useQuery({
    queryKey: ['image-debug-groups', VIDEO_ENDPOINTS],
    queryFn: () => getUserGroups(VIDEO_ENDPOINTS),
  })

  const handleStateChange = useCallback(
    (patch: Partial<VideoDebugFormState>) => {
      setState((prev) => ({ ...prev, ...patch }))
    },
    []
  )

  const handleStop = useCallback(() => {
    setIsPolling(false)
    setTaskId(null)
  }, [])

  // 任务提交
  const handleSubmit = useCallback(async () => {
    setError(null)
    setIsPolling(true)
    setResult(null)
    try {
      const res = await createVideo(buildVideoPayload(state))
      if (res.error?.message) {
        setError(res.error.message)
        setResult(null)
        setIsPolling(false)
        return
      }
      const id = res.id || res.task_id
      if (!id) {
        setError(t('Invalid response: missing task id'))
        setIsPolling(false)
        return
      }
      setResult(res)
      setTaskId(id)
    } catch (err) {
      // 后端以 {"error": {"message": ...}} 返回错误；skipErrorHandler 下需自行提取
      const axiosError = err as {
        response?: { data?: { error?: { message?: string } } }
      }
      setError(
        axiosError.response?.data?.error?.message ||
          (err instanceof Error ? err.message : t('Request error occurred'))
      )
      setResult(null)
      setIsPolling(false)
    }
  }, [state, t])

  // 任务轮询：提交成功拿到 task id 后定时查询，直到完成或失败
  useEffect(() => {
    if (!taskId || !isPolling) return
    let cancelled = false
    const timer = setInterval(async () => {
      try {
        const res = await fetchVideo(taskId)
        if (cancelled) return
        setResult(res)
        if (res.status === 'completed' || res.status === 'failed') {
          clearInterval(timer)
          if (res.status === 'failed') {
            setError(res.error?.message || t('Video generation failed'))
          }
          setIsPolling(false)
          setTaskId(null)
        }
      } catch (err) {
        if (cancelled) return
        clearInterval(timer)
        setError(
          err instanceof Error ? err.message : t('Request error occurred')
        )
        setIsPolling(false)
        setTaskId(null)
      }
    }, VIDEO_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [taskId, isPolling, t])

  const handleDownload = (url: string) => {
    const link = document.createElement('a')
    link.href = url
    link.download = `video-debug-${Date.now()}.mp4`
    link.click()
  }

  // 过滤结果变化后，当前选中值可能失效：渲染期自动修正到第一个有效选项
  // （React "adjusting state during render" 官方模式，避免在 effect 内 setState；
  // 必须位于所有 Hooks 之后，early return 只在中途渲染触发）
  const resolvedModel = resolveSelection(state.model, models, state.model)
  const resolvedGroup = resolveSelection(state.group, groups, state.group)
  if (resolvedModel !== state.model) {
    setState((prev) => ({ ...prev, model: resolvedModel }))
    return null
  }
  if (resolvedGroup !== state.group) {
    setState((prev) => ({ ...prev, group: resolvedGroup }))
    return null
  }

  return (
    <div className='grid flex-1 grid-cols-1 gap-4 lg:grid-cols-2'>
      <div className='min-h-0'>
        {/* 与图片 Tab 一致：表单超出视口高度时内部滚动（Main 布局 overflow-hidden） */}
        <ScrollArea className='max-h-[calc(100vh-220px)] pr-3'>
          <VideoDebugForm
            state={state}
            models={models}
            groups={groups}
            isModelLoading={isLoadingModels}
            isSubmitting={isPolling}
            onStateChange={handleStateChange}
            onSubmit={handleSubmit}
            onStop={handleStop}
          />
        </ScrollArea>
      </div>
      <div className='min-h-0'>
        <VideoResult
          result={result}
          error={error}
          onDownload={handleDownload}
        />
      </div>
    </div>
  )
}
