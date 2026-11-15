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
import { CheckIcon, CopyIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import type { ASRResponse } from '../types'

interface ASRResultProps {
  /** json/verbose_json 为解析后的对象；text/srt/vtt 为原始文本 */
  result: ASRResponse | string | null
  isLoading: boolean
  error: string | null
}

/** 展示识别出的文本，附复制按钮 */
function ResultText(props: { text: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(props.text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      /* ignore */
    }
  }

  return (
    <div className='space-y-2'>
      <p className='bg-muted border-border rounded-lg border p-3 text-sm leading-relaxed whitespace-pre-wrap'>
        {props.text}
      </p>
      <Button type='button' variant='outline' size='sm' onClick={handleCopy}>
        {copied ? (
          <CheckIcon className='size-4' aria-hidden='true' />
        ) : (
          <CopyIcon className='size-4' aria-hidden='true' />
        )}
        {copied ? t('Copied') : t('Copy')}
      </Button>
    </div>
  )
}

export function ASRResult(props: ASRResultProps) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <div className='text-muted-foreground flex size-full min-h-64 flex-col items-center justify-center gap-3'>
        <Spinner className='size-8' />
        <p className='text-sm'>{t('Transcribing...')}</p>
      </div>
    )
  }

  if (props.error) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Request failed')}</AlertTitle>
        <AlertDescription className='break-all whitespace-pre-wrap'>
          {props.error}
        </AlertDescription>
      </Alert>
    )
  }

  if (props.result === null) {
    return (
      <div className='text-muted-foreground flex size-full min-h-64 items-center justify-center'>
        <p className='text-sm'>
          {t('No result yet. Upload or record audio and submit to transcribe.')}
        </p>
      </div>
    )
  }

  // 纯文本格式（text/srt/vtt）：直接展示原文
  if (typeof props.result === 'string') {
    return <ResultText text={props.result} />
  }

  const response = props.result
  const text = response.text ?? ''

  return (
    <div className='space-y-4'>
      {text && <ResultText text={text} />}

      {(response.language || response.duration) && (
        <p className='text-muted-foreground text-xs'>
          {[
            response.language && `Language: ${response.language}`,
            response.duration && `Duration: ${response.duration.toFixed(1)}s`,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      )}

      {/* 原始响应：json/verbose_json（含 segments）时 pretty-print 展示 */}
      <div className='space-y-2'>
        <p className='text-muted-foreground text-xs font-medium'>
          {t('Raw Response')}
        </p>
        <pre className='bg-muted border-border max-h-72 overflow-auto rounded-lg border p-3 font-mono text-xs break-all whitespace-pre-wrap'>
          {JSON.stringify(response, null, 2)}
        </pre>
      </div>
    </div>
  )
}
