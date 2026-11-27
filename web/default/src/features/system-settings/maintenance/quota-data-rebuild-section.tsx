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
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  getQuotaDataRebuildPreview,
  rebuildQuotaData,
} from '@/features/dashboard/api'
import { SettingsSection } from '../components/settings-section'

type RebuildPreview = {
  log_consume_rows: number
  log_first_created_at: number
  log_last_created_at: number
  quota_data_rows: number
  quota_data_first_created_at: number
  quota_data_last_created_at: number
}

const formatTime = (ts: number, t: (key: string) => string) =>
  ts > 0 ? formatTimestamp(ts) : t('No data available')

/**
 * 从消费日志全量重建 quota_data 的运维入口。
 * 展示日志与现有 quota_data 的覆盖对比，管理员二次确认后执行。
 */
export function QuotaDataRebuildSection() {
  const { t } = useTranslation()
  const [preview, setPreview] = useState<RebuildPreview | null>(null)
  const [loadingPreview, setLoadingPreview] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [rebuilding, setRebuilding] = useState(false)

  const handleLoadPreview = async () => {
    setLoadingPreview(true)
    try {
      const res = await getQuotaDataRebuildPreview()
      if (!res.success)
        throw new Error(res.message ?? t('Failed to load preview'))
      setPreview(res.data)
    } catch {
      toast.error(t('Failed to load preview'))
    } finally {
      setLoadingPreview(false)
    }
  }

  const handleRebuild = async () => {
    setRebuilding(true)
    try {
      const res = await rebuildQuotaData()
      if (!res.success) throw new Error(res.message ?? t('Rebuild failed'))
      toast.success(
        t('Rebuilt {{count}} quota data rows', { count: res.data.rows })
      )
      setConfirmOpen(false)
      setPreview(null)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t('Rebuild failed'))
    } finally {
      setRebuilding(false)
    }
  }

  // quota_data 为空时没有历史可丢，不应误报警告
  const logsShorter =
    preview != null &&
    preview.quota_data_rows > 0 &&
    preview.log_consume_rows > 0 &&
    preview.log_first_created_at > preview.quota_data_first_created_at

  return (
    <SettingsSection
      title={t('Rebuild Dashboard Data')}
      description={t(
        'Rebuild quota_data from consumption logs. Existing data will be replaced.'
      )}
    >
      <div className='rounded-lg border p-4 text-sm'>
        <p className='text-muted-foreground'>
          {t(
            'Use this when historical data is missing channel information. The rebuild aggregates all consumption logs by user, model, channel and hour.'
          )}
        </p>
        <Button
          variant='outline'
          size='sm'
          className='mt-3'
          onClick={handleLoadPreview}
          disabled={loadingPreview}
        >
          {loadingPreview ? t('Loading...') : t('Load Preview')}
        </Button>

        {preview && (
          <div className='mt-4 grid gap-3 sm:grid-cols-2'>
            <div className='rounded-md border p-3'>
              <div className='font-medium'>{t('Consumption Logs')}</div>
              <div className='text-muted-foreground mt-1'>
                {t('Rows')}: {preview.log_consume_rows}
              </div>
              <div className='text-muted-foreground'>
                {t('Range')}: {formatTime(preview.log_first_created_at, t)} ~{' '}
                {formatTime(preview.log_last_created_at, t)}
              </div>
            </div>
            <div className='rounded-md border p-3'>
              <div className='font-medium'>{t('Dashboard Data')}</div>
              <div className='text-muted-foreground mt-1'>
                {t('Rows')}: {preview.quota_data_rows}
              </div>
              <div className='text-muted-foreground'>
                {t('Range')}:{' '}
                {formatTime(preview.quota_data_first_created_at, t)} ~{' '}
                {formatTime(preview.quota_data_last_created_at, t)}
              </div>
            </div>
          </div>
        )}

        {logsShorter && (
          <div className='mt-3 flex gap-2 rounded-md border border-amber-500/50 bg-amber-500/10 p-3 text-amber-700 dark:text-amber-300'>
            <AlertTriangle className='mt-0.5 size-4 shrink-0' />
            <span>
              {t(
                'Consumption logs cover a shorter period than existing dashboard data. Rebuilding will permanently lose older history. Consider backing up the database first.'
              )}
            </span>
          </div>
        )}

        <Button
          variant='destructive'
          size='sm'
          className='mt-4'
          onClick={() => setConfirmOpen(true)}
          disabled={!preview}
        >
          <RefreshCw className='mr-2 size-4' />
          {t('Rebuild Dashboard Data')}
        </Button>
      </div>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Confirm Data Rebuild')}</DialogTitle>
            <DialogDescription>
              {t(
                'This will delete and recreate all dashboard data from consumption logs. Task billing records are also counted as consumption, so totals may change. This action cannot be undone.'
              )}
            </DialogDescription>
          </DialogHeader>
          {preview && (
            <p className='text-sm'>
              {t('Rows to write')}: {preview.log_consume_rows}
            </p>
          )}
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setConfirmOpen(false)}
              disabled={rebuilding}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={handleRebuild}
              disabled={rebuilding}
            >
              {rebuilding ? t('Rebuilding...') : t('Confirm Rebuild')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
