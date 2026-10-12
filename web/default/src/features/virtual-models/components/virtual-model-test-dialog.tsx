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
import { useCallback, useEffect, useState } from 'react'
import { type TFunction } from 'i18next'
import { FlaskConical, Loader2, RotateCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { StatusBadge } from '@/components/status-badge'
import { testVirtualModel } from '../api'
import { VIRTUAL_MODEL_MODE_CONFIG } from '../constants'
import {
  type VirtualModelTestResult,
  type VirtualModelTestTargetResult,
} from '../types'
import { useVirtualModels } from './virtual-models-provider'

function formatDuration(ms: number, t: TFunction) {
  if (!ms || ms <= 0) return '-'
  if (ms < 1000) return t('{{value}}ms', { value: ms })
  return t('{{value}}s', { value: (ms / 1000).toFixed(2) })
}

function ResultStatus({ result }: { result: VirtualModelTestTargetResult }) {
  const { t } = useTranslation()
  return result.success ? (
    <StatusBadge variant='success'>{t('Success')}</StatusBadge>
  ) : (
    <StatusBadge variant='danger'>{t('Failed')}</StatusBadge>
  )
}

function TargetResultCard({
  result,
  fastest = false,
}: {
  result: VirtualModelTestTargetResult
  fastest?: boolean
}) {
  const { t } = useTranslation()
  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-center justify-between gap-2'>
        <div className='flex min-w-0 items-center gap-2'>
          <span className='truncate font-medium'>{result.model}</span>
          {fastest && (
            <StatusBadge variant='info'>{t('Fastest')}</StatusBadge>
          )}
        </div>
        <ResultStatus result={result} />
      </div>
      <div className='mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground'>
        {result.channel_id > 0 && (
          <span>
            {t('Channel')} #{result.channel_id}
          </span>
        )}
        {result.first_token_ms > 0 && (
          <span>
            {t('First Token')}: {formatDuration(result.first_token_ms, t)}
          </span>
        )}
        <span>
          {t('Total Time')}: {formatDuration(result.elapsed_ms, t)}
        </span>
      </div>
      {result.error && (
        <p className='mt-2 text-xs whitespace-pre-wrap break-words text-destructive'>
          {result.error}
        </p>
      )}
      {result.answer && (
        <p className='mt-2 text-sm whitespace-pre-wrap break-words'>
          {result.answer}
        </p>
      )}
    </div>
  )
}

export function VirtualModelTestDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow } = useVirtualModels()
  const isOpen = open === 'test'
  const currentId = currentRow?.id

  const [testing, setTesting] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')
  const [result, setResult] = useState<VirtualModelTestResult | null>(null)

  const runTest = useCallback(
    async (vmId: number) => {
      setTesting(true)
      setErrorMsg('')
      setResult(null)
      try {
        const res = await testVirtualModel(vmId)
        if (res.success && res.data) {
          setResult(res.data)
        } else {
          setErrorMsg(res.message || t('Test failed'))
        }
      } catch (e) {
        setErrorMsg(e instanceof Error ? e.message : t('Test failed'))
      } finally {
        setTesting(false)
      }
    },
    [t]
  )

  useEffect(() => {
    if (isOpen && currentId) {
      runTest(currentId)
    } else if (!isOpen) {
      setResult(null)
      setErrorMsg('')
    }
  }, [isOpen, currentId, runTest])

  const mode = result?.mode || currentRow?.mode
  const modeConfig = mode ? VIRTUAL_MODEL_MODE_CONFIG[mode] : null

  // fastest = smallest first-token time among successful sub-models
  const fastestMs = (() => {
    if (!result) return null
    const times = result.targets
      .filter((r) => r.success && r.first_token_ms > 0)
      .map((r) => r.first_token_ms)
    return times.length > 0 ? Math.min(...times) : null
  })()

  return (
    <Sheet open={isOpen} onOpenChange={(v) => !v && setOpen(null)}>
      <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
        <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
          <SheetTitle className='flex items-center gap-2'>
            <FlaskConical className='size-4' />
            {t('Test Virtual Model')}
            {currentRow && (
              <span className='text-muted-foreground'>{currentRow.name}</span>
            )}
            {modeConfig && (
              <StatusBadge variant={mode === 'speed' ? 'info' : 'warning'}>
                {t(modeConfig.labelKey)}
              </StatusBadge>
            )}
          </SheetTitle>
          <SheetDescription>
            {modeConfig ? t(modeConfig.descriptionKey) : ''}
          </SheetDescription>
        </SheetHeader>

        <div className='flex-1 space-y-3 overflow-y-auto px-4 py-4 sm:px-6'>
          {testing && (
            <div className='flex items-center justify-center gap-2 py-12 text-muted-foreground'>
              <Loader2 className='size-4 animate-spin' />
              {t('Testing...')}
            </div>
          )}

          {!testing && errorMsg && (
            <div className='rounded-lg border border-destructive/50 p-3 text-sm text-destructive'>
              {errorMsg}
            </div>
          )}

          {!testing && result && (
            <>
              <div className='rounded-lg bg-muted p-3 text-sm'>
                <div className='text-xs font-medium text-muted-foreground'>
                  {t('Question')}
                </div>
                <p className='mt-1 whitespace-pre-wrap break-words'>
                  {result.question}
                </p>
              </div>

              {result.mode === 'speed' && (
                <div className='space-y-2'>
                  {result.targets.map((r) => (
                    <TargetResultCard
                      key={r.index}
                      result={r}
                      fastest={
                        r.success &&
                        fastestMs !== null &&
                        r.first_token_ms === fastestMs
                      }
                    />
                  ))}
                </div>
              )}

              {result.mode === 'quality' && (
                <div className='space-y-2'>
                  {result.targets.map((r) => (
                    <TargetResultCard key={r.index} result={r} />
                  ))}
                  {result.aggregator ? (
                    <div className='rounded-lg border border-primary p-3'>
                      <div className='flex items-center justify-between gap-2'>
                        <span className='truncate font-medium'>
                          {t('Aggregated Output')}
                          <span className='ml-2 text-muted-foreground'>
                            {result.aggregator.model}
                          </span>
                        </span>
                        <ResultStatus result={result.aggregator} />
                      </div>
                      <div className='mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground'>
                        {result.aggregator.channel_id > 0 && (
                          <span>
                            {t('Channel')} #{result.aggregator.channel_id}
                          </span>
                        )}
                        <span>
                          {t('Total Time')}:{' '}
                          {formatDuration(result.aggregator.elapsed_ms, t)}
                        </span>
                      </div>
                      {result.aggregator.error && (
                        <p className='mt-2 text-xs whitespace-pre-wrap break-words text-destructive'>
                          {result.aggregator.error}
                        </p>
                      )}
                      {result.aggregator.answer && (
                        <p className='mt-2 text-sm whitespace-pre-wrap break-words'>
                          {result.aggregator.answer}
                        </p>
                      )}
                    </div>
                  ) : (
                    <div className='rounded-lg border border-dashed p-3 text-sm text-muted-foreground'>
                      {t(
                        'No successful sub-model answers, aggregation skipped'
                      )}
                    </div>
                  )}
                </div>
              )}

              <div className='pt-1 text-xs text-muted-foreground'>
                {t('Total Time')}: {formatDuration(result.total_ms, t)}
              </div>
            </>
          )}
        </div>

        <div className='border-t px-4 py-3 sm:px-6'>
          <Button
            variant='outline'
            size='sm'
            disabled={testing || !currentId}
            onClick={() => currentId && runTest(currentId)}
          >
            {testing ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <RotateCw className='size-4' />
            )}
            {t('Retry')}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
