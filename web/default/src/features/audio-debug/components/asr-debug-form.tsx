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
import { AudioLinesIcon, SendIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ModelSelector, GroupSelector } from '@/components/model-group-selector'
import { ASR_ENDPOINTS, ASR_FORMATS } from '../constants'
import type { ASRFormState, GroupOption, ModelOption } from '../types'
import { AsrAudioSource } from './asr-audio-source'

interface ASRDebugFormProps {
  state: ASRFormState
  models: ModelOption[]
  groups: GroupOption[]
  isModelLoading: boolean
  isSubmitting: boolean
  /** 当前选中的音频文件（上传或录音产物） */
  file: File | null
  onStateChange: (patch: Partial<ASRFormState>) => void
  onFileChange: (file: File | null) => void
  onSubmit: () => void
}

export function ASRDebugForm(props: ASRDebugFormProps) {
  const { t } = useTranslation()

  const canSubmit = props.state.model.trim().length > 0 && props.file !== null

  return (
    <div className='space-y-4'>
      {/* 模型与分组 */}
      <div className='flex flex-wrap items-center gap-2'>
        <ModelSelector
          selectedModel={props.state.model}
          models={props.models}
          onModelChange={(value) => props.onStateChange({ model: value })}
          disabled={props.isModelLoading || props.isSubmitting}
        />
        <GroupSelector
          selectedGroup={props.state.group}
          groups={props.groups}
          onGroupChange={(value) => props.onStateChange({ group: value })}
          disabled={props.isSubmitting}
        />
      </div>

      {/* 音频来源（上传 / 录音） */}
      <div className='space-y-2'>
        <Label>{t('Audio File')}</Label>
        <AsrAudioSource
          file={props.file}
          onFileChange={props.onFileChange}
          disabled={props.isSubmitting}
        />
      </div>

      {/* 参数 */}
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
        <div className='space-y-2'>
          <Label>{t('Endpoint')}</Label>
          <NativeSelect
            className='w-full'
            value={props.state.endpoint}
            onChange={(e) =>
              props.onStateChange({
                endpoint: e.target.value as ASRFormState['endpoint'],
              })
            }
            disabled={props.isSubmitting}
          >
            {ASR_ENDPOINTS.map((endpoint) => (
              <NativeSelectOption key={endpoint.value} value={endpoint.value}>
                {t(endpoint.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='asr-debug-language'>{t('Language (optional)')}</Label>
          <Input
            id='asr-debug-language'
            value={props.state.language}
            onChange={(e) => props.onStateChange({ language: e.target.value })}
            placeholder='zh / en'
            disabled={props.isSubmitting}
          />
        </div>

        <div className='space-y-2'>
          <Label>{t('Response Format')}</Label>
          <NativeSelect
            className='w-full'
            value={props.state.responseFormat}
            onChange={(e) =>
              props.onStateChange({ responseFormat: e.target.value })
            }
            disabled={props.isSubmitting}
          >
            {ASR_FORMATS.map((format) => (
              <NativeSelectOption key={format.value} value={format.value}>
                {t(format.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      </div>

      {/* 引导提示（可选） */}
      <div className='space-y-2'>
        <Label htmlFor='asr-debug-prompt'>{t('Prompt (optional)')}</Label>
        <Input
          id='asr-debug-prompt'
          value={props.state.prompt}
          onChange={(e) => props.onStateChange({ prompt: e.target.value })}
          placeholder={t('Words or style to guide recognition...')}
          maxLength={400}
          disabled={props.isSubmitting}
        />
      </div>

      {/* 提交 */}
      <div className='flex items-center gap-2'>
        <Button
          type='button'
          onClick={props.onSubmit}
          disabled={!canSubmit || props.isSubmitting}
        >
          {props.isSubmitting ? (
            <AudioLinesIcon
              className='size-4 animate-pulse'
              aria-hidden='true'
            />
          ) : (
            <SendIcon className='size-4' aria-hidden='true' />
          )}
          {props.isSubmitting ? t('Transcribing...') : t('Transcribe Audio')}
        </Button>
      </div>
    </div>
  )
}
