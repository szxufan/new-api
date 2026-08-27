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
import { useState, type ReactNode } from 'react'
import { Check, Copy, Download, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { formatLogQuota, formatTimestampToDate } from '@/lib/format'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { StatusBadge } from '@/components/status-badge'
import { canDownloadTaskResult, downloadTaskResults } from '../../lib/download'
import { taskActionMapper, taskStatusMapper } from '../../lib/mappers'
import type { TaskLog } from '../../types'

interface TaskDetailsDialogProps {
  log: TaskLog
  isAdmin: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

function formatJsonValue(value: unknown): string {
  if (value == null) return ''
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function DetailRow(props: { label: string; children: ReactNode }) {
  return (
    <div className='grid grid-cols-[130px_1fr] items-start gap-2 text-sm'>
      <span className='text-muted-foreground'>{props.label}</span>
      <div className='min-w-0'>{props.children}</div>
    </div>
  )
}

function JsonBlock(props: {
  value: unknown
  copied: boolean
  onCopy: () => void
}) {
  const text = formatJsonValue(props.value)
  if (!text) {
    return <span className='text-muted-foreground/60 text-xs'>-</span>
  }
  return (
    <div className='bg-muted/50 relative rounded-md border p-3'>
      <Button
        variant='ghost'
        size='sm'
        className='absolute top-2 right-2 h-8 w-8 p-0'
        onClick={props.onCopy}
      >
        {props.copied ? (
          <Check className='size-4 text-green-600' />
        ) : (
          <Copy className='size-4' />
        )}
      </Button>
      <pre className='max-h-60 overflow-auto pr-10 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap'>
        {text}
      </pre>
    </div>
  )
}

export function TaskDetailsDialog(props: TaskDetailsDialogProps) {
  const { t } = useTranslation()
  const log = props.log
  const isAdmin = props.isAdmin
  const currentUserId = useAuthStore((s) => s.auth.user?.id)
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [downloading, setDownloading] = useState(false)

  const canDownload = canDownloadTaskResult(log, currentUserId)
  const pollRecord = log.poll_record

  const handleDownload = async () => {
    setDownloading(true)
    try {
      await downloadTaskResults(log)
      toast.success(t('Download started'))
    } catch {
      toast.error(t('Failed to download result'))
    } finally {
      setDownloading(false)
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Task Details')}</DialogTitle>
          <DialogDescription>
            {t('View the full details of this task')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[560px] pr-4'>
          <div className='space-y-5 py-4'>
            <div className='space-y-2'>
              <DetailRow label={t('Task ID')}>
                <span className='font-mono text-xs break-all'>
                  {log.task_id || '-'}
                </span>
              </DetailRow>
              <DetailRow label={t('Platform')}>
                <span>{t(log.platform) || '-'}</span>
              </DetailRow>
              <DetailRow label={t('Action')}>
                <span>{t(taskActionMapper.getLabel(log.action))}</span>
              </DetailRow>
              <DetailRow label={t('Status')}>
                <StatusBadge
                  label={t(
                    taskStatusMapper.getLabel(
                      log.status,
                      log.status || 'Submitting'
                    )
                  )}
                  variant={taskStatusMapper.getVariant(log.status)}
                  size='sm'
                  copyable={false}
                  showDot
                />
              </DetailRow>
              <DetailRow label={t('Progress')}>
                <span>{log.progress || '-'}</span>
              </DetailRow>
              {log.quota != null && (
                <DetailRow label={t('Quota')}>
                  <span>{formatLogQuota(log.quota)}</span>
                </DetailRow>
              )}
              <DetailRow label={t('Submit Time')}>
                <span className='font-mono text-xs tabular-nums'>
                  {formatTimestampToDate(log.submit_time, 'seconds')}
                </span>
              </DetailRow>
              {log.start_time ? (
                <DetailRow label={t('Start Time')}>
                  <span className='font-mono text-xs tabular-nums'>
                    {formatTimestampToDate(log.start_time, 'seconds')}
                  </span>
                </DetailRow>
              ) : null}
              {log.finish_time ? (
                <DetailRow label={t('Finish Time')}>
                  <span className='font-mono text-xs tabular-nums'>
                    {formatTimestampToDate(log.finish_time, 'seconds')}
                  </span>
                </DetailRow>
              ) : null}
              {isAdmin && log.channel_id > 0 && (
                <DetailRow label={t('Channel')}>
                  <span className='font-mono text-xs'>#{log.channel_id}</span>
                </DetailRow>
              )}
              {isAdmin && (
                <DetailRow label={t('User')}>
                  <span>
                    {log.username || '-'}
                    {log.user_id > 0 ? ` (${log.user_id})` : ''}
                  </span>
                </DetailRow>
              )}
            </div>

            {log.properties?.input ? (
              <div className='space-y-2'>
                <Label className='text-sm font-semibold'>{t('Input')}</Label>
                <div className='bg-muted/50 rounded-md border p-3'>
                  <p className='text-sm leading-relaxed break-all whitespace-pre-wrap'>
                    {log.properties.input}
                  </p>
                </div>
              </div>
            ) : null}

            {log.fail_reason ? (
              <div className='space-y-2'>
                <Label className='text-sm font-semibold'>
                  {t('Fail Reason')}
                </Label>
                <div className='bg-muted/50 rounded-md border border-red-200 p-3'>
                  <p className='text-sm leading-relaxed break-all whitespace-pre-wrap text-red-600 dark:text-red-400'>
                    {log.fail_reason}
                  </p>
                </div>
              </div>
            ) : null}

            {isAdmin && (
              <div className='space-y-2'>
                <Label className='text-sm font-semibold'>
                  {t('Last Upstream Poll')}
                </Label>
                {pollRecord && pollRecord.time ? (
                  <div className='space-y-2'>
                    <DetailRow label={t('Poll Time')}>
                      <span className='font-mono text-xs tabular-nums'>
                        {formatTimestampToDate(pollRecord.time, 'seconds')}
                      </span>
                    </DetailRow>
                    {(pollRecord.method || pollRecord.url) && (
                      <DetailRow label={t('Request URL')}>
                        <span className='font-mono text-xs break-all'>
                          {pollRecord.method ? `${pollRecord.method} ` : ''}
                          {pollRecord.url || ''}
                        </span>
                      </DetailRow>
                    )}
                    {pollRecord.status_code != null &&
                      pollRecord.status_code > 0 && (
                        <DetailRow label={t('Status Code')}>
                          <span className='font-mono text-xs'>
                            {pollRecord.status_code}
                          </span>
                        </DetailRow>
                      )}
                    <DetailRow label={t('Request')}>
                      <JsonBlock
                        value={pollRecord.request}
                        copied={
                          copiedText === formatJsonValue(pollRecord.request)
                        }
                        onCopy={() =>
                          copyToClipboard(formatJsonValue(pollRecord.request))
                        }
                      />
                    </DetailRow>
                    <DetailRow label={t('Response')}>
                      <JsonBlock
                        value={pollRecord.response}
                        copied={
                          copiedText === formatJsonValue(pollRecord.response)
                        }
                        onCopy={() =>
                          copyToClipboard(formatJsonValue(pollRecord.response))
                        }
                      />
                    </DetailRow>
                  </div>
                ) : (
                  <p className='text-muted-foreground text-sm'>
                    {t('No poll record yet')}
                  </p>
                )}
              </div>
            )}
          </div>
        </ScrollArea>

        {canDownload && (
          <DialogFooter>
            <Button onClick={handleDownload} disabled={downloading}>
              {downloading ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                <Download className='size-4' />
              )}
              {t('Download Result')}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
