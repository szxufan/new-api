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
import { DownloadIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { isPlayableTtsFormat } from '../constants'
import type { TTSAudioResult } from '../types'

interface TTSResultProps {
  result: TTSAudioResult | null
  isLoading: boolean
  error: string | null
  onDownload: (result: TTSAudioResult) => void
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(2)} MB`
  return `${(bytes / 1024).toFixed(1)} KB`
}

export function TTSResult(props: TTSResultProps) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <div className='text-muted-foreground flex size-full min-h-64 flex-col items-center justify-center gap-3'>
        <Spinner className='size-8' />
        <p className='text-sm'>{t('Synthesizing...')}</p>
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

  if (!props.result) {
    return (
      <div className='text-muted-foreground flex size-full min-h-64 items-center justify-center'>
        <p className='text-sm'>
          {t(
            'No result yet. Configure the form and submit to synthesize speech.'
          )}
        </p>
      </div>
    )
  }

  const playable = isPlayableTtsFormat(props.result.format)

  return (
    <div className='space-y-4'>
      {playable ? (
        <audio
          controls
          src={props.result.url}
          className='w-full'
          preload='metadata'
        >
          {t('Your browser does not support the audio element.')}
        </audio>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t('PCM raw stream cannot be previewed, download the file instead.')}
        </p>
      )}
      <div className='border-border flex items-center justify-between gap-2 rounded-lg border px-3 py-2'>
        <span className='text-muted-foreground text-xs'>
          {props.result.format.toUpperCase()} · {formatSize(props.result.size)}
        </span>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => props.onDownload(props.result as TTSAudioResult)}
          className='shrink-0'
        >
          <DownloadIcon className='size-4' aria-hidden='true' />
          {t('Download Audio')}
        </Button>
      </div>
    </div>
  )
}
