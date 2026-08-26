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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { editImage, generateImage, getUserGroups, getUserModels } from './api'
import { ImageDebugForm } from './components/image-debug-form'
import { ImageResult } from './components/image-result'
import { VideoDebugTab } from './components/video-debug-tab'
import { getImageSrc } from './lib/image-utils'
import {
  DEFAULT_GROUP,
  DEFAULT_MODE,
  DEFAULT_N,
  DEFAULT_QUALITY,
  DEFAULT_RESPONSE_FORMAT,
  DEFAULT_SIZE,
  DEFAULT_STYLE,
} from './constants'
import { buildEditFormData, buildImagePayload } from './lib/payload'
import { getModeEndpoints, resolveSelection } from './lib/selection'
import type {
  ImageData,
  ImageDebugFormState,
  ImageMode,
} from './types'

function createDefaultState(): ImageDebugFormState {
  return {
    mode: DEFAULT_MODE,
    model: '',
    group: DEFAULT_GROUP,
    prompt: '',
    size: DEFAULT_SIZE,
    n: DEFAULT_N,
    quality: DEFAULT_QUALITY,
    style: DEFAULT_STYLE,
    responseFormat: DEFAULT_RESPONSE_FORMAT,
    watermark: false,
    extraParameters: '',
  }
}

export function ImageDebugPage() {
  const { t } = useTranslation()
  const [pageTab, setPageTab] = useState<'image' | 'video'>('image')
  const [state, setState] = useState<ImageDebugFormState>(createDefaultState)
  const [images, setImages] = useState<ImageData[]>([])
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // 按当前模式过滤：文生图 → image-generation；图生图 → image-edit
  const endpoints = getModeEndpoints(state.mode)

  const { data: models = [], isLoading: isLoadingModels } = useQuery({
    queryKey: ['image-debug-models', endpoints],
    queryFn: () => getUserModels(endpoints),
  })

  const { data: groups = [] } = useQuery({
    queryKey: ['image-debug-groups', endpoints],
    queryFn: () => getUserGroups(endpoints),
  })

  const handleStateChange = useCallback((patch: Partial<ImageDebugFormState>) => {
    setState((prev) => ({ ...prev, ...patch }))
  }, [])

  // 过滤结果变化后，当前选中值可能失效：自动修正到第一个有效选项
  useEffect(() => {
    const resolved = resolveSelection(state.model, models, state.model)
    if (resolved !== state.model) {
      setState((prev) => ({ ...prev, model: resolved }))
    }
  }, [models, state.model])

  useEffect(() => {
    const resolved = resolveSelection(state.group, groups, state.group)
    if (resolved !== state.group) {
      setState((prev) => ({ ...prev, group: resolved }))
    }
  }, [groups, state.group])

  const handleModeChange = (mode: ImageMode) => {
    setState((prev) => ({ ...prev, mode }))
    setImages([])
    setError(null)
  }

  const handleSubmit = useCallback(
    async (files: File[]) => {
      setError(null)
      setIsSubmitting(true)
      try {
        const result =
          state.mode === 'edits'
            ? await editImage(buildEditFormData(state, files)!)
            : await generateImage(buildImagePayload(state))
        if (result.error?.message) {
          setError(result.error.message)
          setImages([])
        } else {
          setImages(result.data ?? [])
        }
      } catch (err) {
        // 后端以 {"error": {"message": ...}} 返回错误；skipErrorHandler 下需自行提取
        const axiosError = err as {
          response?: { data?: { error?: { message?: string } } }
        }
        setError(
          axiosError.response?.data?.error?.message ||
            (err instanceof Error ? err.message : t('Request error occurred'))
        )
        setImages([])
      } finally {
        setIsSubmitting(false)
      }
    },
    [state, t]
  )

  const handleStop = useCallback(() => {
    setIsSubmitting(false)
  }, [])

  const handleDownload = (image: ImageData, index: number) => {
    const src = getImageSrc(image)
    if (!src) return
    const link = document.createElement('a')
    link.href = src
    link.download = `image-debug-${index + 1}.png`
    link.click()
  }

  return (
    <div className='mx-auto flex size-full max-w-6xl flex-col gap-4 p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h1 className='text-xl font-semibold'>{t('Image Debug')}</h1>
        <Tabs
          value={pageTab}
          onValueChange={(value) => setPageTab(value as 'image' | 'video')}
        >
          <TabsList>
            <TabsTrigger value='image'>{t('Image')}</TabsTrigger>
            <TabsTrigger value='video'>{t('Video')}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {pageTab === 'video' ? (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('Test video generation endpoints (async task, poll for result).')}
          </p>
          <VideoDebugTab />
        </>
      ) : (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('Test text-to-image and image-to-image endpoints.')}
          </p>
          <div className='grid flex-1 grid-cols-1 gap-4 lg:grid-cols-2'>
            <Card className='min-h-0'>
              <CardHeader>
                <CardTitle>{t('Request')}</CardTitle>
              </CardHeader>
              <CardContent>
                <ScrollArea className='max-h-[calc(100vh-220px)] pr-3'>
                  <ImageDebugForm
                    state={state}
                    models={models}
                    groups={groups}
                    isModelLoading={isLoadingModels}
                    isSubmitting={isSubmitting}
                    onStateChange={handleStateChange}
                    onModeChange={handleModeChange}
                    onSubmit={handleSubmit}
                    onStop={handleStop}
                  />
                </ScrollArea>
              </CardContent>
            </Card>

            <Card className='min-h-0'>
              <CardHeader>
                <CardTitle>{t('Result')}</CardTitle>
              </CardHeader>
              <CardContent>
                <ScrollArea className='max-h-[calc(100vh-220px)] pr-3'>
                  <ImageResult
                    images={images}
                    isLoading={isSubmitting}
                    error={error}
                    onDownload={(image, index) => handleDownload(image, index)}
                  />
                </ScrollArea>
              </CardContent>
            </Card>
          </div>
        </>
      )}
    </div>
  )
}