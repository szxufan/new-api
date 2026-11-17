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
import { Textarea } from '@/components/ui/textarea'
import { ModelSelector, GroupSelector } from '@/components/model-group-selector'
import {
  TTS_INPUT_MAX_LENGTH,
  TTS_SPEED_MAX,
  TTS_SPEED_MIN,
  TTS_SPEED_STEP,
  getDefaultTTSVoice,
  getTTSFormatOptions,
  getTTSVoiceOptions,
  isDeepSeekTTSModel,
} from '../constants'
import type { GroupOption, ModelOption, TTSFormState } from '../types'

interface TTSDebugFormProps {
  state: TTSFormState
  models: ModelOption[]
  groups: GroupOption[]
  isModelLoading: boolean
  isSubmitting: boolean
  onStateChange: (patch: Partial<TTSFormState>) => void
  onSubmit: () => void
}

export function TTSDebugForm(props: TTSDebugFormProps) {
  const { t } = useTranslation()

  const isDeepSeekTTS = isDeepSeekTTSModel(props.state.model)
  const voiceOptions = getTTSVoiceOptions(props.state.model)
  const formatOptions = getTTSFormatOptions(props.state.model)

  const canSubmit =
    props.state.model.trim().length > 0 &&
    props.state.input.trim().length > 0 &&
    props.state.voice.trim().length > 0

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

      {/* 合成文本 */}
      <div className='space-y-2'>
        <Label htmlFor='tts-debug-input'>{t('Input Text')}</Label>
        <Textarea
          id='tts-debug-input'
          value={props.state.input}
          onChange={(e) => props.onStateChange({ input: e.target.value })}
          placeholder={t('Enter the text to synthesize...')}
          rows={4}
          maxLength={TTS_INPUT_MAX_LENGTH}
          disabled={props.isSubmitting}
          className='max-h-80'
        />
      </div>

      {/* 参数 */}
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
        <div className='space-y-2'>
          <Label htmlFor='tts-debug-voice'>{t('Voice')}</Label>
          {isDeepSeekTTS ? (
            // DeepSeek 仅支持官方音色，用固定下拉避免选到无效音色
            <NativeSelect
              id='tts-debug-voice'
              className='w-full'
              value={props.state.voice}
              onChange={(e) => props.onStateChange({ voice: e.target.value })}
              disabled={props.isSubmitting}
            >
              {voiceOptions.map((voice) => (
                <NativeSelectOption key={voice} value={voice}>
                  {voice}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          ) : (
            <>
              <Input
                id='tts-debug-voice'
                list='tts-debug-voices'
                value={props.state.voice}
                onChange={(e) => props.onStateChange({ voice: e.target.value })}
                placeholder={getDefaultTTSVoice(props.state.model)}
                disabled={props.isSubmitting}
              />
              <datalist id='tts-debug-voices'>
                {voiceOptions.map((voice) => (
                  <option key={voice} value={voice} />
                ))}
              </datalist>
            </>
          )}
        </div>

        <div className='space-y-2'>
          <Label htmlFor='tts-debug-speed'>{t('Speed')}</Label>
          <Input
            id='tts-debug-speed'
            type='number'
            min={TTS_SPEED_MIN}
            max={TTS_SPEED_MAX}
            step={TTS_SPEED_STEP}
            value={props.state.speed}
            onChange={(e) => props.onStateChange({ speed: e.target.value })}
            placeholder='1'
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
            {formatOptions.map((format) => (
              <NativeSelectOption key={format.value} value={format.value}>
                {t(format.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      </div>

      {/* 指令（可选，gpt-4o-mini-tts 等支持语气/风格控制） */}
      <div className='space-y-2'>
        <Label htmlFor='tts-debug-instructions'>
          {t('Instructions (optional)')}
        </Label>
        <Textarea
          id='tts-debug-instructions'
          value={props.state.instructions}
          onChange={(e) =>
            props.onStateChange({ instructions: e.target.value })
          }
          placeholder={t('Voice tone, emotion and delivery style...')}
          rows={2}
          maxLength={TTS_INPUT_MAX_LENGTH}
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
          {props.isSubmitting ? t('Synthesizing...') : t('Synthesize Speech')}
        </Button>
      </div>
    </div>
  )
}
