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
import {
  PaperclipIcon,
  FileIcon,
  ImageIcon,
  ScreenShareIcon,
  CameraIcon,
  GlobeIcon,
  SendIcon,
  SquareIcon,
  BarChartIcon,
  BoxIcon,
  NotepadTextIcon,
  CodeSquareIcon,
  GraduationCapIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { t as tStatic } from 'i18next'
import { toast } from 'sonner'
import { nanoid } from 'nanoid'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  PromptInput,
  PromptInputAttachment,
  PromptInputAttachments,
  PromptInputButton,
  PromptInputFooter,
  PromptInputTextarea,
  PromptInputTools,
  usePromptInputAttachments,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { Suggestion, Suggestions } from '@/components/ai-elements/suggestion'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { CameraCaptureDialog } from './camera-capture-dialog'
import {
  captureScreenshot,
  compressImageDataUrl,
  isImageMediaType,
  isTextLikeFile,
} from '../lib/attachment-utils'
import { TEXT_FILE_MAX_BYTES } from '../constants'
import type { FileUIPart } from 'ai'
import type { ModelOption, GroupOption, MessageAttachment } from '../types'

interface PlaygroundInputProps {
  onSubmit: (
    text: string,
    attachments: MessageAttachment[]
  ) => void | Promise<void>
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  models: ModelOption[]
  modelValue: string
  onModelChange: (value: string) => void
  isModelLoading?: boolean
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (value: string) => void
}

const suggestions = [
  { icon: BarChartIcon, text: 'Analyze data', color: '#76d0eb' },
  { icon: BoxIcon, text: 'Surprise me', color: '#76d0eb' },
  { icon: NotepadTextIcon, text: 'Summarize text', color: '#ea8444' },
  { icon: CodeSquareIcon, text: 'Code', color: '#6c71ff' },
  { icon: GraduationCapIcon, text: 'Get advice', color: '#76d0eb' },
  { icon: null, text: 'More' },
]

export function PlaygroundInput({
  onSubmit,
  onStop,
  disabled,
  isGenerating,
  models,
  modelValue,
  onModelChange,
  isModelLoading = false,
  groups,
  groupValue,
  onGroupChange,
}: PlaygroundInputProps) {
  const [text, setText] = useState('')

  const isModelSelectDisabled =
    disabled || isModelLoading || models.length === 0
  const isGroupSelectDisabled = disabled || groups.length === 0

  const handleSubmit = (message: PromptInputMessage) => {
    const hasText = !!message.text?.trim()
    const hasFiles = !!message.files?.length
    if ((!hasText && !hasFiles) || disabled) return

    // 返回 Promise：resolve 后 PromptInput 自动清空附件与文本；reject 时保留
    return processSubmit(message.text || '', message.files || [])
  }

  // 处理附件：图片压缩为 dataURL；文本类文件内容内联进消息文本；其余拒绝
  const processSubmit = async (
    rawText: string,
    files: FileUIPart[]
  ): Promise<void> => {
    const attachments: MessageAttachment[] = []
    const textParts: string[] = [rawText.trim()]

    for (const file of files) {
      const filename = file.filename || 'attachment'
      const realMediaType = file.mediaType || ''

      // 获取 dataURL（blob: 或 data:）
      let dataUrl: string | undefined
      if (file.url) {
        if (file.url.startsWith('data:')) {
          dataUrl = file.url
        } else if (file.url.startsWith('blob:')) {
          dataUrl = await blobToDataUrl(file.url)
        }
      }

      if (isImageMediaType(realMediaType) && dataUrl) {
        try {
          const compressed = await compressImageDataUrl(dataUrl)
          attachments.push({
            id: nanoid(),
            kind: 'image',
            filename,
            mediaType: realMediaType,
            url: compressed,
          })
        } catch {
          toast.error(tStatic('Failed to process attachment'), {
            description: filename,
          })
          throw new Error(`compress failed: ${filename}`)
        }
        continue
      }

      if (isTextLikeFile(realMediaType, filename)) {
        try {
          const blob = await fetchBlob(file.url)
          if (!blob) {
            toast.error(tStatic('Failed to process attachment'), {
              description: filename,
            })
            throw new Error(`fetch failed: ${filename}`)
          }
          let content = await blob.text()
          if (content.length > TEXT_FILE_MAX_BYTES) {
            content = content.slice(0, TEXT_FILE_MAX_BYTES)
            toast.info(tStatic('File is too large and was truncated'), {
              description: filename,
            })
          }
          const ext = filename.split('.').pop() || ''
          textParts.push(
            `\n\`\`\`${ext}\n# ${filename}\n${content}\n\`\`\`\n`
          )
          attachments.push({
            id: nanoid(),
            kind: 'file',
            filename,
            mediaType: realMediaType,
          })
        } catch {
          toast.error(tStatic('Failed to process attachment'), {
            description: filename,
          })
          throw new Error(`read failed: ${filename}`)
        }
        continue
      }

      toast.error(tStatic('Unsupported file type'), {
        description: filename,
      })
      throw new Error(`unsupported: ${filename}`)
    }

    const finalText = textParts.join('').trim()
    await onSubmit(finalText, attachments)
  }

  const handleSuggestionClick = (suggestion: string) => {
    onSubmit(suggestion, [])
  }

  return (
    <div className='grid shrink-0 gap-4 px-1 md:pb-4'>
      <PromptInput
        groupClassName='rounded-xl'
        multiple
        onSubmit={handleSubmit}
        onError={({ message }) => toast.error(message)}
      >
        <PlaygroundInputInner
          disabled={disabled}
          groups={groups}
          groupValue={groupValue}
          isGenerating={isGenerating}
          isGroupSelectDisabled={isGroupSelectDisabled}
          isModelSelectDisabled={isModelSelectDisabled}
          models={models}
          modelValue={modelValue}
          onGroupChange={onGroupChange}
          onModelChange={onModelChange}
          onStop={onStop}
          setText={setText}
          text={text}
        />
      </PromptInput>

      <Suggestions>
        {suggestions.map(({ icon: Icon, text: sText, color }) => (
          <Suggestion
            className={`text-xs font-normal sm:text-sm ${
              sText === 'More' ? 'hidden sm:flex' : ''
            }`}
            key={sText}
            onClick={() => handleSuggestionClick(sText)}
            suggestion={sText}
          >
            {Icon && <Icon size={16} style={{ color }} />}
            {sText}
          </Suggestion>
        ))}
      </Suggestions>
    </div>
  )
}

/**
 * 内部组件：在 PromptInput 上下文中，可安全使用 usePromptInputAttachments
 */
function PlaygroundInputInner({
  text,
  setText,
  disabled,
  isGenerating,
  onStop,
  models,
  modelValue,
  onModelChange,
  isModelSelectDisabled,
  groups,
  groupValue,
  onGroupChange,
  isGroupSelectDisabled,
}: {
  text: string
  setText: (v: string) => void
  disabled?: boolean
  isGenerating?: boolean
  onStop?: () => void
  models: ModelOption[]
  modelValue: string
  onModelChange: (v: string) => void
  isModelSelectDisabled: boolean
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (v: string) => void
  isGroupSelectDisabled: boolean
}) {
  const { t } = useTranslation()
  const attachments = usePromptInputAttachments()
  const [cameraOpen, setCameraOpen] = useState(false)

  const handleUploadFile = () => {
    if (attachments.fileInputRef.current) {
      attachments.fileInputRef.current.accept = ''
    }
    attachments.openFileDialog()
  }

  const handleUploadPhoto = () => {
    if (attachments.fileInputRef.current) {
      attachments.fileInputRef.current.accept = 'image/*'
    }
    attachments.openFileDialog()
  }

  const handleScreenshot = async () => {
    if (disabled) return
    try {
      const file = await captureScreenshot()
      attachments.add([file])
    } catch (error) {
      const msg = (error as Error)?.message || ''
      if (msg === 'SCREEN_CAPTURE_NOT_SUPPORTED') {
        toast.error(t('Screen capture is not supported in this browser'))
      } else {
        toast.info(t('Screen capture was cancelled'))
      }
    }
  }

  const handleCameraCapture = (file: File) => {
    attachments.add([file])
  }

  const hasAttachments = attachments.files.length > 0

  return (
    <>
      <PromptInputAttachments>
        {(attachment) => (
          <PromptInputAttachment data={attachment} />
        )}
      </PromptInputAttachments>

      <PromptInputTextarea
        autoComplete='off'
        autoCorrect='off'
        autoCapitalize='off'
        spellCheck={false}
        className='px-5 md:text-base'
        disabled={disabled}
        onChange={(event) => setText(event.target.value)}
        placeholder={t('Ask anything')}
        value={text}
      />

      <PromptInputFooter className='p-2.5'>
        <PromptInputTools>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <PromptInputButton
                  className='border font-medium'
                  disabled={disabled}
                  variant='outline'
                />
              }
            >
              <PaperclipIcon size={16} />
              <span className='hidden sm:inline'>{t('Attach')}</span>
              <span className='sr-only sm:hidden'>{t('Attach')}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align='start'>
              <DropdownMenuItem onClick={handleUploadFile}>
                <FileIcon className='mr-2' size={16} />
                {t('Upload file')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleUploadPhoto}>
                <ImageIcon className='mr-2' size={16} />
                {t('Upload photo')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleScreenshot}>
                <ScreenShareIcon className='mr-2' size={16} />
                {t('Take screenshot')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => {
                  if (disabled) return
                  setCameraOpen(true)
                }}
              >
                <CameraIcon className='mr-2' size={16} />
                {t('Take photo')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <PromptInputButton
            className='border font-medium'
            disabled={disabled}
            onClick={() => toast.info(t('Search feature in development'))}
            variant='outline'
          >
            <GlobeIcon size={16} />
            <span className='hidden sm:inline'>{t('Search')}</span>
            <span className='sr-only sm:hidden'>{t('Search')}</span>
          </PromptInputButton>
        </PromptInputTools>

        <div className='flex items-center gap-1.5 md:gap-2'>
          <ModelGroupSelector
            selectedModel={modelValue}
            models={models}
            onModelChange={onModelChange}
            selectedGroup={groupValue}
            groups={groups}
            onGroupChange={onGroupChange}
            disabled={isModelSelectDisabled || isGroupSelectDisabled}
          />

          {isGenerating && onStop ? (
            <PromptInputButton
              className='text-foreground font-medium'
              onClick={onStop}
              variant='secondary'
            >
              <SquareIcon className='fill-current' size={16} />
              <span className='hidden sm:inline'>{t('Stop')}</span>
              <span className='sr-only sm:hidden'>{t('Stop')}</span>
            </PromptInputButton>
          ) : (
            <PromptInputButton
              className='text-foreground font-medium'
              disabled={disabled || (!text.trim() && !hasAttachments)}
              type='submit'
              variant='secondary'
            >
              <SendIcon size={16} />
              <span className='hidden sm:inline'>{t('Send')}</span>
              <span className='sr-only sm:hidden'>{t('Send')}</span>
            </PromptInputButton>
          )}
        </div>
      </PromptInputFooter>

      <CameraCaptureDialog
        onCapture={handleCameraCapture}
        onOpenChange={setCameraOpen}
        open={cameraOpen}
      />
    </>
  )
}

// --- 辅助函数：blob URL -> dataURL ---
function blobToDataUrl(blobUrl: string): Promise<string> {
  return new Promise((resolve, reject) => {
    fetch(blobUrl)
      .then((r) => r.blob())
      .then((blob) => {
        const reader = new FileReader()
        reader.onloadend = () => resolve(reader.result as string)
        reader.onerror = reject
        reader.readAsDataURL(blob)
      })
      .catch(reject)
  })
}

function fetchBlob(url?: string): Promise<Blob | null> {
  if (!url) return Promise.resolve(null)
  return fetch(url).then((r) => r.blob())
}
