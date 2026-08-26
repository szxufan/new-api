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
import { DownloadIcon, ExternalLinkIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import type { VideoResponse } from '../types'

interface VideoResultProps {
  result: VideoResponse | null
  error: string | null
  onDownload?: (url: string) => void
}

export function VideoResult({
  result,
  error,
  onDownload,
}: VideoResultProps) {
  const { t } = useTranslation()

  if (error) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Request failed')}</AlertTitle>
        <AlertDescription className='break-all whitespace-pre-wrap'>
          {error}
        </AlertDescription>
      </Alert>
    )
  }

  if (!result) {
    return (
      <div className='text-muted-foreground flex size-full min-h-64 items-center justify-center'>
        <p className='text-sm'>
          {t('No result yet. Configure the form and submit to generate a video.')}
        </p>
      </div>
    )
  }

  const url = result.metadata?.url

  if (result.status === 'failed') {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Video generation failed')}</AlertTitle>
        <AlertDescription className='break-all whitespace-pre-wrap'>
          {result.error?.message || result.error?.code || ''}
        </AlertDescription>
      </Alert>
    )
  }

  if (result.status !== 'completed' || !url) {
    const statusLabel =
      result.status === 'queued' ? t('Queued...') : t('Generating video...')
    return (
      <div className='flex size-full min-h-64 flex-col items-center justify-center gap-3 text-muted-foreground'>
        <Spinner className='size-8' />
        <p className='text-sm'>{statusLabel}</p>
        {result.progress != null && result.progress > 0 && (
          <p className='text-xs'>{result.progress}%</p>
        )}
      </div>
    )
  }

  return (
    <div className='space-y-3'>
      <video
        src={url}
        controls
        preload='metadata'
        className='bg-muted block max-h-[480px] w-full rounded-lg border'
      >
        {t('Your browser does not support the video tag.')}
      </video>
      <div className='flex items-center gap-2'>
        {onDownload && (
          <Button type='button' variant='outline' size='sm' onClick={() => onDownload(url)}>
            <DownloadIcon className='size-4' aria-hidden='true' />
            {t('Download')}
          </Button>
        )}
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => window.open(url, '_blank', 'noreferrer')}
        >
          <ExternalLinkIcon className='size-4' aria-hidden='true' />
          {t('Open in new tab')}
        </Button>
      </div>
    </div>
  )
}