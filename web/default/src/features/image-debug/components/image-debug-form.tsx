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
import { ImagePlusIcon, SendIcon, SquareIcon, XIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ModelSelector, GroupSelector } from '@/components/model-group-selector'
import {
  IMAGE_QUALITIES,
  IMAGE_SIZES,
  IMAGE_STYLES,
  N_MAX,
  N_MIN,
  RESPONSE_FORMATS,
} from '../constants'
import type {
  GroupOption,
  ImageDebugFormState,
  ImageMode,
  ModelOption,
} from '../types'
import { PromptOptimizer } from './prompt-optimizer'

interface ImageDebugFormProps {
  state: ImageDebugFormState
  models: ModelOption[]
  groups: GroupOption[]
  isModelLoading: boolean
  isSubmitting: boolean
  onStateChange: (patch: Partial<ImageDebugFormState>) => void
  /** 提交；图生图时 files 为选中的图片文件列表 */
  onSubmit: (files: File[]) => void
  onStop?: () => void
  onModeChange: (mode: ImageMode) => void
}

export function ImageDebugForm({
  state,
  models,
  groups,
  isModelLoading,
  isSubmitting,
  onStateChange,
  onSubmit,
  onStop,
  onModeChange,
}: ImageDebugFormProps) {
  const { t } = useTranslation()
  const [previewUrls, setPreviewUrls] = useState<string[]>([])
  const [files, setFiles] = useState<File[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)

  const isEdits = state.mode === 'edits'
  const canSubmit =
    state.prompt.trim().length > 0 && (isEdits ? files.length > 0 : true)

  const handleFilesSelected = (fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return
    const selected = Array.from(fileList)
    const combined = [...files, ...selected].slice(0, 4)
    setFiles(combined)
    setPreviewUrls(combined.map((f) => URL.createObjectURL(f)))
  }

  const handleRemovePreview = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index))
    setPreviewUrls((prev) => prev.filter((_, i) => i !== index))
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  return (
    <div className='space-y-4'>
      {/* 模式切换 */}
      <Tabs
        value={state.mode}
        onValueChange={(value) => onModeChange(value as ImageMode)}
      >
        <TabsList>
          <TabsTrigger value='generations'>{t('Text to Image')}</TabsTrigger>
          <TabsTrigger value='edits'>{t('Image to Image')}</TabsTrigger>
        </TabsList>
      </Tabs>

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

      {/* 图生图：图片上传 */}
      {isEdits && (
        <div className='space-y-2'>
          <Label>{t('Input Images')}</Label>
          <div className='border-border flex min-h-24 flex-wrap items-center gap-2 rounded-lg border border-dashed p-3'>
            {previewUrls.length === 0 && (
              <button
                type='button'
                onClick={() => fileInputRef.current?.click()}
                className='text-muted-foreground hover:text-foreground flex size-full min-h-20 flex-col items-center justify-center gap-1 rounded-md text-xs transition-colors'
              >
                <ImagePlusIcon className='size-5' aria-hidden='true' />
                {t('Click to upload images (JPEG/PNG/WebP)')}
              </button>
            )}
            {previewUrls.map((url, index) => (
              <div key={index} className='relative'>
                <img
                  src={url}
                  alt={`preview-${index + 1}`}
                  className='bg-muted size-20 rounded-md object-cover'
                />
                <button
                  type='button'
                  onClick={() => handleRemovePreview(index)}
                  className='bg-background hover:bg-destructive hover:text-destructive-foreground absolute -top-2 -right-2 rounded-full border p-0.5'
                  aria-label={t('Remove image')}
                >
                  <XIcon className='size-3' aria-hidden='true' />
                </button>
              </div>
            ))}
            {previewUrls.length > 0 && (
              <button
                type='button'
                onClick={() => fileInputRef.current?.click()}
                className='text-muted-foreground hover:text-foreground flex size-20 flex-col items-center justify-center gap-1 rounded-md border border-dashed text-[10px] transition-colors'
              >
                <ImagePlusIcon className='size-4' aria-hidden='true' />
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
        </div>
      )}

      {/* Prompt */}
      <div className='space-y-2'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <Label htmlFor='image-debug-prompt'>{t('Prompt')}</Label>
          <PromptOptimizer
            type='image'
            prompt={state.prompt}
            group={state.group}
            disabled={isSubmitting}
            onOptimized={(prompt) => onStateChange({ prompt })}
          />
        </div>
        <Textarea
          id='image-debug-prompt'
          value={state.prompt}
          onChange={(e) => onStateChange({ prompt: e.target.value })}
          placeholder={t('Describe the image you want to generate...')}
          rows={4}
          maxLength={4000}
          disabled={isSubmitting}
        />
      </div>

      {/* 参数 */}
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
        <div className='space-y-2'>
          <Label>{t('Size')}</Label>
          <NativeSelect
            className='w-full'
            value={state.size}
            onChange={(e) => onStateChange({ size: e.target.value })}
            disabled={isSubmitting}
          >
            {IMAGE_SIZES.map((size) => (
              <NativeSelectOption key={size.value} value={size.value}>
                {t(size.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-2'>
          <Label>{t('Quantity')}</Label>
          <Input
            type='number'
            min={N_MIN}
            max={N_MAX}
            value={state.n}
            onChange={(e) => onStateChange({ n: Number(e.target.value) || 1 })}
            disabled={isSubmitting}
          />
        </div>

        <div className='space-y-2'>
          <Label>{t('Quality')}</Label>
          <NativeSelect
            className='w-full'
            value={state.quality}
            onChange={(e) => onStateChange({ quality: e.target.value })}
            disabled={isSubmitting}
          >
            {IMAGE_QUALITIES.map((q) => (
              <NativeSelectOption key={q.value} value={q.value}>
                {t(q.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-2'>
          <Label>{t('Style')}</Label>
          <NativeSelect
            className='w-full'
            value={state.style}
            onChange={(e) => onStateChange({ style: e.target.value })}
            disabled={isSubmitting}
          >
            {IMAGE_STYLES.map((s) => (
              <NativeSelectOption key={s.value} value={s.value}>
                {t(s.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-2'>
          <Label>{t('Response Format')}</Label>
          <NativeSelect
            className='w-full'
            value={state.responseFormat}
            onChange={(e) => onStateChange({ responseFormat: e.target.value })}
            disabled={isSubmitting}
          >
            {RESPONSE_FORMATS.map((f) => (
              <NativeSelectOption key={f.value} value={f.value}>
                {t(f.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='flex items-end pb-1'>
          <label className='flex items-center gap-2 text-sm'>
            <Checkbox
              checked={state.watermark}
              onCheckedChange={(checked) =>
                onStateChange({ watermark: checked === true })
              }
              disabled={isSubmitting}
            />
            {t('Watermark')}
          </label>
        </div>
      </div>

      {/* 高级参数（透传 extra.parameters，仅文生图） */}
      {!isEdits && (
        <div className='space-y-2'>
          <Label htmlFor='image-debug-extra'>
            {t('Advanced Parameters (JSON, optional)')}
          </Label>
          <Textarea
            id='image-debug-extra'
            value={state.extraParameters}
            onChange={(e) => onStateChange({ extraParameters: e.target.value })}
            placeholder='{"size": "1024*1024", "prompt_extend": true, "watermark": false}'
            rows={3}
            className='font-mono text-xs'
            disabled={isSubmitting}
          />
        </div>
      )}

      {/* 提交 */}
      <div className='flex items-center gap-2'>
        {isSubmitting ? (
          <Button type='button' variant='outline' onClick={onStop}>
            <SquareIcon className='size-4' aria-hidden='true' />
            {t('Stop')}
          </Button>
        ) : (
          <Button
            type='button'
            onClick={() => onSubmit(files)}
            disabled={!canSubmit}
          >
            <SendIcon className='size-4' aria-hidden='true' />
            {isEdits ? t('Edit Image') : t('Generate Image')}
          </Button>
        )}
      </div>
    </div>
  )
}
