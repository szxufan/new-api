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
import { describe, expect, it } from 'vitest'

import {
  getDefaultTTSFormat,
  getDefaultTTSVoice,
  getASRAcceptForModel,
  getASRFormatOptions,
  getASREndpointOptions,
  getTTSFormatOptions,
  getTTSVoiceOptions,
  isDeepSeekASRModel,
  isDeepSeekTTSModel,
  isValidTTSVoice,
  resolveASRFormatForModel,
  resolveASREndpointForModel,
  resolveTTSFormatForModel,
  resolveTTSVoiceForModel,
} from './constants'

describe('isDeepSeekTTSModel', () => {
  it('matches deepseek tts virtual models', () => {
    expect(isDeepSeekTTSModel('deepseek-tts')).toBe(true)
    expect(isDeepSeekTTSModel('DeepSeek-TTS')).toBe(true)
    expect(isDeepSeekTTSModel(' deepseek-tts ')).toBe(true)
    expect(isDeepSeekTTSModel('deepseek-v4-tts')).toBe(true)
  })

  it('does not match other models', () => {
    expect(isDeepSeekTTSModel('deepseek-chat')).toBe(false)
    expect(isDeepSeekTTSModel('deepseek-asr')).toBe(false)
    expect(isDeepSeekTTSModel('tts-1')).toBe(false)
    expect(isDeepSeekTTSModel('gpt-4o-mini-tts')).toBe(false)
    expect(isDeepSeekTTSModel('')).toBe(false)
  })
})

describe('isDeepSeekASRModel', () => {
  it('matches deepseek asr virtual models', () => {
    expect(isDeepSeekASRModel('deepseek-asr')).toBe(true)
    expect(isDeepSeekASRModel('DeepSeek-ASR')).toBe(true)
    expect(isDeepSeekASRModel('deepseek-v4-asr')).toBe(true)
  })

  it('does not match other models', () => {
    expect(isDeepSeekASRModel('deepseek-chat')).toBe(false)
    expect(isDeepSeekASRModel('deepseek-tts')).toBe(false)
    expect(isDeepSeekASRModel('whisper-1')).toBe(false)
    expect(isDeepSeekASRModel('')).toBe(false)
  })
})

describe('TTS voice options', () => {
  it('returns official voices for deepseek tts', () => {
    expect(getTTSVoiceOptions('deepseek-tts')).toEqual(['stella', 'tide'])
    expect(getDefaultTTSVoice('deepseek-tts')).toBe('stella')
  })

  it('returns openai voices for other models', () => {
    expect(getTTSVoiceOptions('tts-1')).toContain('alloy')
    expect(getDefaultTTSVoice('tts-1')).toBe('alloy')
  })

  it('validates voice per model', () => {
    expect(isValidTTSVoice('deepseek-tts', 'stella')).toBe(true)
    expect(isValidTTSVoice('deepseek-tts', 'STELLA')).toBe(true)
    expect(isValidTTSVoice('deepseek-tts', 'alloy')).toBe(false)
    // 非 DeepSeek 模型允许任意自定义音色
    expect(isValidTTSVoice('tts-1', 'my-custom-voice')).toBe(true)
    expect(isValidTTSVoice('tts-1', '')).toBe(false)
  })

  it('resolves invalid voice to model default', () => {
    expect(resolveTTSVoiceForModel('deepseek-tts', 'alloy')).toBe('stella')
    expect(resolveTTSVoiceForModel('deepseek-tts', 'tide')).toBe('tide')
    expect(resolveTTSVoiceForModel('tts-1', 'my-voice')).toBe('my-voice')
  })
})

describe('TTS format options', () => {
  it('restricts deepseek tts to wav/pcm', () => {
    const values = getTTSFormatOptions('deepseek-tts').map((f) => f.value)
    expect(values).toEqual(['wav', 'pcm'])
    expect(getDefaultTTSFormat('deepseek-tts')).toBe('wav')
  })

  it('keeps full format list for other models', () => {
    const values = getTTSFormatOptions('tts-1').map((f) => f.value)
    expect(values).toContain('mp3')
    expect(getDefaultTTSFormat('tts-1')).toBe('mp3')
  })

  it('resolves invalid format to model default', () => {
    expect(resolveTTSFormatForModel('deepseek-tts', 'mp3')).toBe('wav')
    expect(resolveTTSFormatForModel('deepseek-tts', 'pcm')).toBe('pcm')
    expect(resolveTTSFormatForModel('tts-1', 'mp3')).toBe('mp3')
  })
})

describe('ASR options per model', () => {
  it('restricts deepseek asr endpoint to transcriptions', () => {
    const values = getASREndpointOptions('deepseek-asr').map((e) => e.value)
    expect(values).toEqual(['transcriptions'])
    expect(resolveASREndpointForModel('deepseek-asr', 'translations')).toBe(
      'transcriptions'
    )
    expect(resolveASREndpointForModel('whisper-1', 'translations')).toBe(
      'translations'
    )
  })

  it('restricts deepseek asr format to json', () => {
    const values = getASRFormatOptions('deepseek-asr').map((f) => f.value)
    expect(values).toEqual(['json'])
    expect(resolveASRFormatForModel('deepseek-asr', 'verbose_json')).toBe('json')
    expect(resolveASRFormatForModel('whisper-1', 'verbose_json')).toBe(
      'verbose_json'
    )
  })

  it('narrows file accept for deepseek asr', () => {
    expect(getASRAcceptForModel('deepseek-asr')).toBe('.wav,.ogg,.opus')
    expect(getASRAcceptForModel('whisper-1')).toContain('.mp3')
  })
})
