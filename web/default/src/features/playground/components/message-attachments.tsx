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
import { FileIcon, ImageIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type { MessageAttachment } from '../types'

interface MessageAttachmentsProps {
  attachments: MessageAttachment[]
  className?: string
}

/**
 * 在用户消息气泡内渲染附件：
 * - 图片（有 url）：缩略图
 * - 其余（含已剥离 url 的历史图片）：文件名占位 chip
 */
export function MessageAttachments({
  attachments,
  className,
}: MessageAttachmentsProps) {
  const { t } = useTranslation()

  return (
    <div
      className={cn('flex flex-wrap gap-2', className)}
    >
      {attachments.map((att) => {
        if (att.kind === 'image' && att.url) {
          return (
            <img
              alt={att.filename || t('Image')}
              className='max-h-40 rounded-md border object-cover'
              key={att.id}
              src={att.url}
            />
          )
        }

        const isImage = att.kind === 'image'
        const label =
          att.filename || (isImage ? t('Image') : t('Attachment'))

        return (
          <div
            className='flex items-center gap-1.5 rounded-md border bg-muted px-2 py-1 text-xs text-muted-foreground'
            key={att.id}
          >
            {isImage ? (
              <ImageIcon className='size-3' />
            ) : (
              <FileIcon className='size-3' />
            )}
            <span className='truncate'>{label}</span>
          </div>
        )
      })}
    </div>
  )
}
