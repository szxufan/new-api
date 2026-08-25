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
import { getImageSrc } from '../lib/image-utils'
import type { ImageData } from '../types'

interface ImageResultProps {
  images: ImageData[]
  isLoading: boolean
  error: string | null
  onDownload?: (image: ImageData, index: number) => void
}

export function ImageResult({
  images,
  isLoading,
  error,
  onDownload,
}: ImageResultProps) {
  const { t } = useTranslation()

  if (isLoading) {
    return (
      <div className='flex size-full min-h-64 flex-col items-center justify-center gap-3 text-muted-foreground'>
        <Spinner className='size-8' />
        <p className='text-sm'>{t('Generating...')}</p>
      </div>
    )
  }

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

  if (images.length === 0) {
    return (
      <div className='text-muted-foreground flex size-full min-h-64 items-center justify-center'>
        <p className='text-sm'>{t('No result yet. Configure the form and submit to generate images.')}</p>
      </div>
    )
  }

  return (
    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
      {images.map((image, index) => {
        const src = getImageSrc(image)
        if (!src) return null
        return (
          <figure
            key={index}
            className='border-border overflow-hidden rounded-lg border'
          >
            <a href={src} target='_blank' rel='noreferrer'>
              <img
                src={src}
                alt={image.revised_prompt || `result-${index + 1}`}
                className='bg-muted block size-full max-h-96 w-full object-contain'
                loading='lazy'
              />
            </a>
            {onDownload && (
              <figcaption className='border-border flex items-center justify-between gap-2 border-t px-3 py-2'>
                {image.revised_prompt ? (
                  <span className='text-muted-foreground truncate text-xs'>
                    {image.revised_prompt}
                  </span>
                ) : (
                  <span className='text-muted-foreground text-xs'>
                    {t('Result {{index}}', { index: index + 1 })}
                  </span>
                )}
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => onDownload(image, index)}
                  className='shrink-0'
                >
                  <DownloadIcon className='size-4' aria-hidden='true' />
                  <span className='sr-only'>{t('Download')}</span>
                </Button>
              </figcaption>
            )}
          </figure>
        )
      })}
    </div>
  )
}