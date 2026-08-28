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
import { useRef, useState } from 'react'
import {
  ClapperboardIcon,
  FilmIcon,
  SendIcon,
  SquareIcon,
  XIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import { ModelSelector, GroupSelector } from '@/components/model-group-selector'
import {
  PROMPT_MAX_LENGTH,
  VIDEO_DURATION_MAX,
  VIDEO_DURATION_MIN,
  VIDEO_IMAGE_MAX,
  VIDEO_RATIOS,
  VIDEO_RESOLUTIONS,
} from '../constants'
import { fileToDataUrl } from '../lib/file-utils'
import type { GroupOption, ModelOption, VideoDebugFormState } from '../types'
import { PromptOptimizer } from './prompt-optimizer'

interface VideoDebugFormProps {
  state: VideoDebugFormState
  models: ModelOption[]
  groups: GroupOption[]
  isModelLoading: boolean
  isSubmitting: boolean
  onStateChange: (patch: Partial<VideoDebugFormState>) => void
  onSubmit: () => void
  onStop?: () => void
}

export function VideoDebugForm({
  state,
  models,
  groups,
  isModelLoading,
  isSubmitting,
  onStateChange,
  onSubmit,
  onStop,
}: VideoDebugFormProps) {
  const { t } = useTranslation()
  const [fileError, setFileError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const canSubmit = state.prompt.trim().length > 0 && state.model !== ''

  const handleFilesSelected = async (fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return
    setFileError(null)
    const remaining = VIDEO_IMAGE_MAX - state.images.length
    if (remaining <= 0) {
      setFileError(
        t('You can upload up to {{count}} images.', { count: VIDEO_IMAGE_MAX })
      )
      return
    }
    const files = Array.from(fileList).slice(0, remaining)
    try {
      const dataUrls = await Promise.all(
        files.map((file) => fileToDataUrl(file))
      )
      onStateChange({ images: [...state.images, ...dataUrls] })
    } catch {
      setFileError(t('Failed to read image file'))
    }
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handleRemoveImage = (index: number) => {
    onStateChange({
      images: state.images.filter((_, i) => i !== index),
    })
  }

  return (
    <div className='space-y-4'>
      {/* 模型与分组 */}
      <div className='flex flex-wrap items-center gap-2'>
        <ModelSelector
          selectedModel={state.model}
          models={models}
          onModelChange={(value) => onStateChange({ model: value })}
          disabled={isModelLoading || isSubmitting}
        />
        <GroupSelector
          selectedGroup={state.group}
          groups={groups}
          onGroupChange={(value) => onStateChange({ group: value })}
          disabled={isSubmitting}
        />
      </div>

      {/* 图片上传（可选）：1 张 = 首帧生视频，多张 = 参考生视频 */}
      <div className='space-y-2'>
        <Label>{t('Input Images (optional)')}</Label>
        <div className='text-muted-foreground text-xs'>
          {t(
            'One image for image-to-video, multiple images for reference-based generation.'
          )}
        </div>
        <div className='border-border flex min-h-24 flex-wrap items-center gap-2 rounded-lg border border-dashed p-3'>
          {state.images.length === 0 && (
            <button
              type='button'
              onClick={() => fileInputRef.current?.click()}
              className='text-muted-foreground hover:text-foreground flex size-full min-h-20 flex-col items-center justify-center gap-1 rounded-md text-xs transition-colors'
            >
              <FilmIcon className='size-5' aria-hidden='true' />
              {t('Click to upload images (JPEG/PNG/WebP)')}
            </button>
          )}
          {state.images.map((dataUrl, index) => (
            <div key={index} className='relative'>
              <img
                src={dataUrl}
                alt={`preview-${index + 1}`}
                className='bg-muted size-20 rounded-md object-cover'
              />
              <button
                type='button'
                onClick={() => handleRemoveImage(index)}
                className='bg-background hover:bg-destructive hover:text-destructive-foreground absolute -top-2 -right-2 rounded-full border p-0.5'
                aria-label={t('Remove image')}
              >
                <XIcon className='size-3' aria-hidden='true' />
              </button>
            </div>
          ))}
          {state.images.length > 0 && state.images.length < VIDEO_IMAGE_MAX && (
            <button
              type='button'
              onClick={() => fileInputRef.current?.click()}
              className='text-muted-foreground hover:text-foreground flex size-20 flex-col items-center justify-center gap-1 rounded-md border border-dashed text-[10px] transition-colors'
            >
              <ClapperboardIcon className='size-4' aria-hidden='true' />
              {t('Add')}
            </button>
          )}
          <input
            ref={fileInputRef}
            type='file'
            accept='image/jpeg,image/png,image/webp'
            multiple
            className='hidden'
            onChange={(e) => handleFilesSelected(e.target.files)}
          />
        </div>
        {fileError && <p className='text-destructive text-xs'>{fileError}</p>}
      </div>

      {/* Prompt */}
      <div className='space-y-2'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <Label htmlFor='video-debug-prompt'>{t('Prompt')}</Label>
          <PromptOptimizer
            type='video'
            prompt={state.prompt}
            group={state.group}
            duration={state.duration}
            images={state.images}
            disabled={isSubmitting}
            onOptimized={(prompt) => onStateChange({ prompt })}
          />
        </div>
        <Textarea
          id='video-debug-prompt'
          value={state.prompt}
          onChange={(e) => onStateChange({ prompt: e.target.value })}
          placeholder={t('Describe the video you want to generate...')}
          rows={4}
          maxLength={PROMPT_MAX_LENGTH}
          disabled={isSubmitting}
          className='max-h-80'
        />
      </div>

      {/* 参数 */}
      <div className='grid grid-cols-2 gap-3'>
        <div className='space-y-2'>
          <Label>{t('Aspect Ratio')}</Label>
          <NativeSelect
            className='w-full'
            value={state.ratio}
            onChange={(e) => onStateChange({ ratio: e.target.value })}
            disabled={isSubmitting}
          >
            {VIDEO_RATIOS.map((ratio) => (
              <NativeSelectOption
                key={ratio.value || 'adaptive'}
                value={ratio.value}
              >
                {ratio.label === 'Adaptive' ? t('Adaptive') : ratio.value}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-2'>
          <Label>{t('Resolution')}</Label>
          <NativeSelect
            className='w-full'
            value={state.resolution}
            onChange={(e) => onStateChange({ resolution: e.target.value })}
            disabled={isSubmitting}
          >
            {VIDEO_RESOLUTIONS.map((resolution) => (
              <NativeSelectOption
                key={resolution.value}
                value={resolution.value}
              >
                {resolution.value}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-2'>
          <Label>{t('Duration (seconds)')}</Label>
          <Input
            type='number'
            min={VIDEO_DURATION_MIN}
            max={VIDEO_DURATION_MAX}
            step={1}
            value={state.duration}
            onChange={(e) =>
              onStateChange({
                duration:
                  e.target.value.trim() === ''
                    ? 0
                    : Number(e.target.value) || 0,
              })
            }
            disabled={isSubmitting}
          />
          <p className='text-muted-foreground text-xs'>
            {t('2-30 seconds, or -1 for smart duration.')}
          </p>
        </div>
      </div>

      {/* 提交 */}
      <div className='flex items-center gap-2'>
        {isSubmitting ? (
          <Button type='button' variant='outline' onClick={onStop}>
            <SquareIcon className='size-4' aria-hidden='true' />
            {t('Stop')}
          </Button>
        ) : (
          <Button type='button' onClick={onSubmit} disabled={!canSubmit}>
            <SendIcon className='size-4' aria-hidden='true' />
            {t('Generate Video')}
          </Button>
        )}
      </div>
    </div>
  )
}
