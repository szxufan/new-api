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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import {
  ATTACHMENT_IMAGE_JPEG_QUALITY,
} from '../constants'

interface CameraCaptureDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCapture: (file: File) => void
}

/**
 * 拍照弹窗：打开摄像头实时预览，点击拍照后返回 File
 */
export function CameraCaptureDialog({
  open,
  onOpenChange,
  onCapture,
}: CameraCaptureDialogProps) {
  const { t } = useTranslation()
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [error, setError] = useState<string | null>(null)

  // 打开时申请摄像头
  useEffect(() => {
    if (!open) return
    let cancelled = false
    setError(null)

    async function start() {
      try {
        if (
          !navigator.mediaDevices ||
          typeof navigator.mediaDevices.getUserMedia !== 'function'
        ) {
          setError('CAMERA_NOT_SUPPORTED')
          return
        }
        const stream = await navigator.mediaDevices.getUserMedia({
          video: true,
          audio: false,
        })
        if (cancelled) {
          stream.getTracks().forEach((track) => track.stop())
          return
        }
        streamRef.current = stream
        if (videoRef.current) {
          videoRef.current.srcObject = stream
          try {
            await videoRef.current.play()
          } catch {
            // 忽略 play 中断
          }
        }
      } catch {
        if (!cancelled) {
          setError('CAMERA_ACCESS_DENIED')
        }
      }
    }

    start()
    return () => {
      cancelled = true
      if (streamRef.current) {
        streamRef.current.getTracks().forEach((track) => track.stop())
        streamRef.current = null
      }
    }
  }, [open])

  const handleCapture = () => {
    const video = videoRef.current
    if (!video) return
    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth || 1280
    canvas.height = video.videoHeight || 720
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      setError('CANVAS_CONTEXT_FAILED')
      return
    }
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
    const dataUrl = canvas.toDataURL(
      'image/jpeg',
      ATTACHMENT_IMAGE_JPEG_QUALITY
    )
    const file = dataUrlToFile(
      dataUrl,
      `photo-${Date.now()}.jpg`,
      'image/jpeg'
    )
    onCapture(file)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Take photo')}</DialogTitle>
          <DialogDescription>{t('Camera preview')}</DialogDescription>
        </DialogHeader>
        <div className='flex items-center justify-center bg-black rounded-md overflow-hidden aspect-video w-full'>
          {error ? (
            <p className='text-muted-foreground px-4 text-center text-sm'>
              {t('Camera access denied or unavailable')}
            </p>
          ) : (
            <video
              autoPlay
              className='h-full w-full object-cover'
              muted
              playsInline
              ref={videoRef}
            />
          )}
        </div>
        <DialogFooter>
          <Button
            onClick={() => onOpenChange(false)}
            type='button'
            variant='outline'
          >
            {t('Cancel')}
          </Button>
          <Button
            disabled={!!error}
            onClick={handleCapture}
            type='button'
          >
            {t('Capture')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// 本地辅助：dataURL -> File（避免与 attachment-utils 循环依赖）
function dataUrlToFile(
  dataUrl: string,
  filename: string,
  mediaType: string = 'image/jpeg'
): File {
  const arr = dataUrl.split(',')
  const mimeMatch = arr[0].match(/:(.*?);/)
  const mime = mimeMatch && mimeMatch[1] ? mimeMatch[1] : mediaType
  const bstr = atob(arr[1] || '')
  let n = bstr.length
  const u8arr = new Uint8Array(n)
  while (n--) {
    u8arr[n] = bstr.charCodeAt(n)
  }
  return new File([u8arr], filename, { type: mime })
}
