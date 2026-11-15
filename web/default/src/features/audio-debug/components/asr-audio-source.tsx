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
import {
  FileAudioIcon,
  MicIcon,
  SquareIcon,
  UploadIcon,
  XIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ASR_ACCEPT, ASR_FILE_MAX_SIZE } from '../constants'
import { useAudioRecorder } from '../hooks/use-audio-recorder'
import type { AsrSource } from '../types'

interface AsrAudioSourceProps {
  file: File | null
  onFileChange: (file: File | null) => void
  disabled?: boolean
}

function formatSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(2)} MB`
  return `${(bytes / 1024).toFixed(1)} KB`
}

/** ASR 音频来源：上传文件 / 麦克风录音 */
export function AsrAudioSource(props: AsrAudioSourceProps) {
  const { t } = useTranslation()
  const [source, setSource] = useState<AsrSource>('upload')
  const [fileError, setFileError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const recorder = useAudioRecorder()

  // 录音产物同步给表单状态（record 模式下 recorder.file 即待识别音频）
  useEffect(() => {
    if (source === 'record') {
      props.onFileChange(recorder.file)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, recorder.file])

  const handleSourceChange = (value: AsrSource) => {
    // 切换来源时清空已选/已录音频，避免提交到非预期的文件
    setSource(value)
    setFileError(null)
    recorder.reset()
    props.onFileChange(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handleFilesSelected = (fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return
    const file = fileList[0]
    if (file.size > ASR_FILE_MAX_SIZE) {
      setFileError(
        t('File exceeds the maximum size of {{size}} MB', {
          size: ASR_FILE_MAX_SIZE / 1024 / 1024,
        })
      )
      return
    }
    setFileError(null)
    props.onFileChange(file)
  }

  const handleRemoveFile = () => {
    setFileError(null)
    props.onFileChange(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const recordedFile = source === 'record' ? recorder.file : null
  const activeFile = source === 'record' ? recordedFile : props.file

  const handleRecordStop = () => {
    recorder.stop()
  }

  return (
    <div className='space-y-2'>
      <Tabs
        value={source}
        onValueChange={(value) => handleSourceChange(value as AsrSource)}
      >
        <TabsList>
          <TabsTrigger value='upload'>{t('Upload File')}</TabsTrigger>
          <TabsTrigger value='record'>
            {t('Record from microphone')}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      {source === 'upload' ? (
        <div
          className='border-border flex min-h-20 flex-col items-center justify-center gap-1 rounded-lg border border-dashed p-3 text-xs transition-colors'
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault()
            handleFilesSelected(e.dataTransfer.files)
          }}
        >
          {props.file ? (
            <div className='flex w-full items-center justify-between gap-2'>
              <span className='flex min-w-0 items-center gap-2'>
                <FileAudioIcon
                  className='text-muted-foreground size-4 shrink-0'
                  aria-hidden='true'
                />
                <span className='truncate' title={props.file.name}>
                  {props.file.name} · {formatSize(props.file.size)}
                </span>
              </span>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                onClick={handleRemoveFile}
                disabled={props.disabled}
                aria-label={t('Remove file')}
              >
                <XIcon className='size-4' aria-hidden='true' />
              </Button>
            </div>
          ) : (
            <button
              type='button'
              onClick={() => fileInputRef.current?.click()}
              disabled={props.disabled}
              className='text-muted-foreground hover:text-foreground flex flex-col items-center justify-center gap-1 rounded-md p-2 transition-colors disabled:opacity-50'
            >
              <UploadIcon className='size-5' aria-hidden='true' />
              {t('Click or drop an audio file to upload')}
            </button>
          )}
          <input
            ref={fileInputRef}
            type='file'
            accept={ASR_ACCEPT}
            className='hidden'
            onChange={(e) => handleFilesSelected(e.target.files)}
          />
        </div>
      ) : (
        <div className='border-border flex min-h-20 flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-3'>
          {recorder.isRecording ? (
            <>
              <span className='text-destructive flex items-center gap-2 text-sm'>
                <span className='bg-destructive size-2 animate-pulse rounded-full' />
                {t('Recording... {{seconds}}s', { seconds: recorder.duration })}
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={handleRecordStop}
              >
                <SquareIcon className='size-4' aria-hidden='true' />
                {t('Stop Recording')}
              </Button>
            </>
          ) : activeFile ? (
            <div className='flex w-full items-center justify-between gap-2'>
              <span className='flex min-w-0 items-center gap-2'>
                <FileAudioIcon
                  className='text-muted-foreground size-4 shrink-0'
                  aria-hidden='true'
                />
                <span className='truncate'>
                  {activeFile.name} · {formatSize(activeFile.size)}
                </span>
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => recorder.start()}
                disabled={props.disabled}
              >
                <MicIcon className='size-4' aria-hidden='true' />
                {t('Record again')}
              </Button>
            </div>
          ) : (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => recorder.start()}
              disabled={props.disabled}
            >
              <MicIcon className='size-4' aria-hidden='true' />
              {t('Start Recording')}
            </Button>
          )}
          {recorder.errorKey && (
            <p className='text-destructive text-xs'>{t(recorder.errorKey)}</p>
          )}
        </div>
      )}

      {fileError && <p className='text-destructive text-xs'>{fileError}</p>}
    </div>
  )
}
