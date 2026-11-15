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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  getUserGroups,
  getUserModels,
  synthesizeSpeech,
  transcribeAudio,
} from './api'
import { ASRDebugForm } from './components/asr-debug-form'
import { ASRResult } from './components/asr-result'
import { TTSDebugForm } from './components/tts-debug-form'
import { TTSResult } from './components/tts-result'
import {
  ASR_MODEL_ENDPOINTS,
  DEFAULT_ASR_FORMAT,
  DEFAULT_GROUP,
  DEFAULT_TTS_FORMAT,
  DEFAULT_TTS_SPEED,
  DEFAULT_TTS_VOICE,
  TTS_MODEL_ENDPOINTS,
  getAsrRequestPath,
} from './constants'
import { isAudioRequestCanceled, resolveAudioErrorMessage } from './lib/errors'
import {
  buildASRFormData,
  buildTTSPayload,
  resolveSelection,
} from './lib/payload'
import type {
  ASRFormState,
  ASRResponse,
  TTSFormState,
  TTSAudioResult,
} from './types'

type PageTab = 'tts' | 'asr'

function createDefaultTTSState(): TTSFormState {
  return {
    model: '',
    group: DEFAULT_GROUP,
    input: '',
    voice: DEFAULT_TTS_VOICE,
    speed: DEFAULT_TTS_SPEED,
    responseFormat: DEFAULT_TTS_FORMAT,
    instructions: '',
  }
}

function createDefaultASRState(): ASRFormState {
  return {
    model: '',
    group: DEFAULT_GROUP,
    endpoint: 'transcriptions',
    language: '',
    prompt: '',
    responseFormat: DEFAULT_ASR_FORMAT,
  }
}

export function AudioDebugPage() {
  const { t } = useTranslation()
  const [pageTab, setPageTab] = useState<PageTab>('tts')

  const [ttsState, setTTSState] = useState<TTSFormState>(createDefaultTTSState)
  const [asrState, setASRState] = useState<ASRFormState>(createDefaultASRState)
  const [asrFile, setAsrFile] = useState<File | null>(null)

  const [ttsResult, setTTSResult] = useState<TTSAudioResult | null>(null)
  const [asrResult, setAsrResult] = useState<ASRResponse | string | null>(null)
  const [ttsSubmitting, setTTSSubmitting] = useState(false)
  const [asrSubmitting, setASRSubmitting] = useState(false)
  const [ttsError, setTTSError] = useState<string | null>(null)
  const [asrError, setASRError] = useState<string | null>(null)

  // 保存当前音频 object URL，替换/切换 Tab/卸载时释放，避免内存泄漏
  const audioUrlRef = useRef<string | null>(null)
  // 保存当前在飞请求，切换 Tab 或卸载时取消
  const abortRef = useRef<AbortController | null>(null)

  const releaseAudioUrl = useCallback(() => {
    if (audioUrlRef.current) {
      URL.revokeObjectURL(audioUrlRef.current)
      audioUrlRef.current = null
    }
  }, [])

  const cancelInFlight = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    setTTSSubmitting(false)
    setASRSubmitting(false)
  }, [])

  useEffect(() => {
    return () => {
      abortRef.current?.abort()
      abortRef.current = null
      releaseAudioUrl()
    }
  }, [releaseAudioUrl])

  // 按当前 Tab 过滤模型与分组：TTS → tts 端点；ASR → asr 端点
  const endpoints =
    pageTab === 'tts' ? [...TTS_MODEL_ENDPOINTS] : [...ASR_MODEL_ENDPOINTS]

  const { data: models = [], isLoading: isLoadingModels } = useQuery({
    queryKey: ['audio-debug-models', endpoints],
    queryFn: () => getUserModels(endpoints),
  })

  const { data: groups = [] } = useQuery({
    queryKey: ['audio-debug-groups', endpoints],
    queryFn: () => getUserGroups(endpoints),
  })

  // 过滤结果变化后，当前选中值可能失效：在渲染期派生有效值（回退到第一个有效选项）
  const activeModel = pageTab === 'tts' ? ttsState.model : asrState.model
  const activeGroup = pageTab === 'tts' ? ttsState.group : asrState.group
  const resolvedModel = resolveSelection(activeModel, models, activeModel)
  const resolvedGroup = resolveSelection(activeGroup, groups, activeGroup)
  const ttsFormState: TTSFormState = {
    ...ttsState,
    model: resolvedModel,
    group: resolvedGroup,
  }
  const asrFormState: ASRFormState = {
    ...asrState,
    model: resolvedModel,
    group: resolvedGroup,
  }

  const handleTTSStateChange = useCallback((patch: Partial<TTSFormState>) => {
    setTTSState((prev) => ({ ...prev, ...patch }))
  }, [])

  const handleASRStateChange = useCallback((patch: Partial<ASRFormState>) => {
    setASRState((prev) => ({ ...prev, ...patch }))
  }, [])

  const handleTTSSubmit = useCallback(async () => {
    cancelInFlight()
    const controller = new AbortController()
    abortRef.current = controller
    setTTSError(null)
    setTTSSubmitting(true)
    try {
      const result = await synthesizeSpeech(
        buildTTSPayload({
          ...ttsState,
          model: resolveSelection(ttsState.model, models, ttsState.model),
          group: resolveSelection(ttsState.group, groups, ttsState.group),
        }),
        controller.signal
      )
      if (controller.signal.aborted) {
        // 请求已被取消：释放刚创建的 object URL，避免泄漏
        URL.revokeObjectURL(result.url)
        return
      }
      // 释放上一条音频的 object URL
      releaseAudioUrl()
      audioUrlRef.current = result.url
      setTTSResult(result)
    } catch (err) {
      if (isAudioRequestCanceled(err)) return
      setTTSResult(null)
      setTTSError(resolveAudioErrorMessage(err, t))
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null
        setTTSSubmitting(false)
      }
    }
  }, [ttsState, models, groups, t, cancelInFlight, releaseAudioUrl])

  const handleASRSubmit = useCallback(async () => {
    const formData = buildASRFormData(
      {
        ...asrState,
        model: resolveSelection(asrState.model, models, asrState.model),
        group: resolveSelection(asrState.group, groups, asrState.group),
      },
      asrFile
    )
    if (!formData) return
    cancelInFlight()
    const controller = new AbortController()
    abortRef.current = controller
    setASRError(null)
    setASRSubmitting(true)
    try {
      const result = await transcribeAudio(
        getAsrRequestPath(asrState.endpoint),
        formData,
        controller.signal
      )
      if (controller.signal.aborted) return
      setAsrResult(result)
    } catch (err) {
      if (isAudioRequestCanceled(err)) return
      setAsrResult(null)
      setASRError(resolveAudioErrorMessage(err, t))
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null
        setASRSubmitting(false)
      }
    }
  }, [asrState, asrFile, models, groups, t, cancelInFlight])

  const handleDownload = useCallback((result: TTSAudioResult) => {
    const link = document.createElement('a')
    link.href = result.url
    link.download = `audio-debug.${result.format}`
    link.click()
  }, [])

  const handleTabChange = (value: string) => {
    const nextTab = value as PageTab
    if (nextTab === pageTab) return
    // 切换 Tab 时取消在飞请求并清空另一侧的音频与状态，避免串台与内存泄漏
    cancelInFlight()
    releaseAudioUrl()
    setPageTab(nextTab)
    setTTSResult(null)
    setAsrResult(null)
    setTTSError(null)
    setASRError(null)
  }

  return (
    <div className='mx-auto flex size-full max-w-6xl flex-col gap-4 p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h1 className='text-xl font-semibold'>{t('Audio Debug')}</h1>
        <Tabs value={pageTab} onValueChange={handleTabChange}>
          <TabsList>
            <TabsTrigger value='tts'>{t('Text to Speech')}</TabsTrigger>
            <TabsTrigger value='asr'>{t('Speech to Text')}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <p className='text-muted-foreground text-sm'>
        {t('Test text-to-speech and speech recognition endpoints.')}
      </p>

      <div className='grid flex-1 grid-cols-1 gap-4 lg:grid-cols-2'>
        <Card className='min-h-0'>
          <CardHeader>
            <CardTitle>{t('Request')}</CardTitle>
          </CardHeader>
          <CardContent>
            <ScrollArea className='max-h-[calc(100vh-220px)] pr-3'>
              {pageTab === 'tts' ? (
                <TTSDebugForm
                  state={ttsFormState}
                  models={models}
                  groups={groups}
                  isModelLoading={isLoadingModels}
                  isSubmitting={ttsSubmitting}
                  onStateChange={handleTTSStateChange}
                  onSubmit={handleTTSSubmit}
                />
              ) : (
                <ASRDebugForm
                  state={asrFormState}
                  models={models}
                  groups={groups}
                  isModelLoading={isLoadingModels}
                  isSubmitting={asrSubmitting}
                  file={asrFile}
                  onStateChange={handleASRStateChange}
                  onFileChange={setAsrFile}
                  onSubmit={handleASRSubmit}
                />
              )}
            </ScrollArea>
          </CardContent>
        </Card>

        <Card className='min-h-0'>
          <CardHeader>
            <CardTitle>{t('Result')}</CardTitle>
          </CardHeader>
          <CardContent>
            <ScrollArea className='max-h-[calc(100vh-220px)] pr-3'>
              {pageTab === 'tts' ? (
                <TTSResult
                  result={ttsResult}
                  isLoading={ttsSubmitting}
                  error={ttsError}
                  onDownload={handleDownload}
                />
              ) : (
                <ASRResult
                  result={asrResult}
                  isLoading={asrSubmitting}
                  error={asrError}
                />
              )}
            </ScrollArea>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
