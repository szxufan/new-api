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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  BarChart3,
  Clock,
  Gauge,
  HeartPulse,
  Timer,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getActiveRequests,
  getPerfMetricsSummary,
} from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatRequestCount,
  formatThroughput,
  formatUptimePct,
} from '@/features/performance-metrics/lib/format'
import type { PerfModelSummary } from '@/features/performance-metrics/types'

const PERFORMANCE_WINDOW_HOURS = 24
const DEFAULT_TOP_MODEL_LIMIT = 5

type PerformanceHealthPanelProps = {
  topModelLimit?: number
}

type WeightedMetric = 'avg_latency_ms' | 'avg_tps' | 'success_rate'

function simpleAverage(
  rows: PerfModelSummary[],
  metric: WeightedMetric,
  isValid: (value: number) => boolean
): number {
  let total = 0
  let count = 0
  for (const row of rows) {
    const value = Number(row[metric])
    if (!isValid(value)) continue
    total += value
    count++
  }
  return count > 0 ? total / count : NaN
}

function rateTextClass(rate: number): string {
  if (!Number.isFinite(rate)) return 'text-muted-foreground'
  if (rate >= 99.9) return 'text-success'
  if (rate >= 99) return 'text-warning'
  return 'text-destructive'
}

function rateDotClass(rate: number): string {
  if (!Number.isFinite(rate)) return 'bg-muted-foreground'
  if (rate >= 99.9) return 'bg-success'
  if (rate >= 99) return 'bg-warning'
  return 'bg-destructive'
}

export function PerformanceHealthPanel(props: PerformanceHealthPanelProps) {
  const { t } = useTranslation()
  const limit = props.topModelLimit ?? DEFAULT_TOP_MODEL_LIMIT
  const metricsQuery = useQuery({
    queryKey: ['perf-metrics-summary', PERFORMANCE_WINDOW_HOURS],
    queryFn: () => getPerfMetricsSummary(PERFORMANCE_WINDOW_HOURS),
    staleTime: 60 * 1000,
    retry: false,
  })

  const activeQuery = useQuery({
    queryKey: ['perf-metrics-active'],
    queryFn: () => getActiveRequests(),
    refetchInterval: 5000,
    staleTime: 0,
    retry: false,
  })

  const models = useMemo(
    () => metricsQuery.data?.data.models ?? [],
    [metricsQuery.data]
  )

  const summary = useMemo(() => {
    return {
      avgLatencyMs: Math.round(
        simpleAverage(
          models,
          'avg_latency_ms',
          (v) => Number.isFinite(v) && v > 0
        )
      ),
      avgTps: simpleAverage(
        models,
        'avg_tps',
        (v) => Number.isFinite(v) && v > 0
      ),
      successRate: simpleAverage(models, 'success_rate', Number.isFinite),
    }
  }, [models])

  const topModels = useMemo(() => models.slice(0, limit), [models, limit])
  const loading = metricsQuery.isLoading
  const activeLoading = activeQuery.isLoading
  const activeStats = activeQuery.data?.data
  const hasData = models.length > 0

  return (
    <section className='bg-card h-full overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center gap-2 border-b px-4 py-3 sm:px-5'>
        <HeartPulse
          className='text-muted-foreground/60 size-4 shrink-0'
          aria-hidden='true'
        />
        <h3 className='text-sm font-semibold'>{t('Performance health')}</h3>
        <span className='text-muted-foreground ml-auto text-xs'>
          {t('Performance metrics for the last 24 hours')}
        </span>
      </div>

      <div className='space-y-3 p-4 sm:p-5'>
        <div className='grid grid-cols-3 gap-2'>
          <MetricCell
            icon={HeartPulse}
            label={t('Success rate')}
            value={formatUptimePct(summary.successRate)}
            loading={loading}
            valueClassName={rateTextClass(summary.successRate)}
          />
          <MetricCell
            icon={Timer}
            label={t('Average latency')}
            value={formatLatency(summary.avgLatencyMs)}
            loading={loading}
          />
          <MetricCell
            icon={Gauge}
            label={t('Throughput')}
            value={formatThroughput(summary.avgTps)}
            loading={loading}
          />
        </div>

        <div className='grid grid-cols-3 gap-2'>
          <MetricCell
            icon={Activity}
            label={t('Active requests')}
            value={
              activeStats
                ? formatRequestCount(activeStats.active_requests)
                : '—'
            }
            loading={activeLoading}
          />
          <MetricCell
            icon={Clock}
            label={t('Requests (10m)')}
            value={
              activeStats ? formatRequestCount(activeStats.requests_10m) : '—'
            }
            loading={activeLoading}
          />
          <MetricCell
            icon={BarChart3}
            label={t('Requests (1h)')}
            value={
              activeStats ? formatRequestCount(activeStats.requests_1h) : '—'
            }
            loading={activeLoading}
          />
        </div>

        {loading ? (
          <div className='space-y-1'>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className='h-5 w-full rounded' />
            ))}
          </div>
        ) : (
          hasData && (
            <div>
              <span className='text-muted-foreground mb-1 block text-[11px] font-medium'>
                {t('Top models by traffic')}
              </span>
              <div className='grid grid-cols-1 gap-x-4 sm:grid-cols-2'>
                {topModels.map((model) => (
                  <div
                    key={model.model_name}
                    className='flex items-center justify-between gap-2 rounded px-1.5 py-1'
                  >
                    <span className='min-w-0 flex-1 truncate font-mono text-[11px]'>
                      {model.model_name}
                    </span>
                    <span className='inline-flex shrink-0 items-center gap-1'>
                      <span
                        className={cn(
                          'size-1.5 rounded-full',
                          rateDotClass(model.success_rate)
                        )}
                        aria-hidden='true'
                      />
                      <span
                        className={cn(
                          'font-mono text-[11px] font-semibold tabular-nums',
                          rateTextClass(model.success_rate)
                        )}
                      >
                        {formatUptimePct(model.success_rate)}
                      </span>
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )
        )}
      </div>
    </section>
  )
}

function MetricCell(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  loading: boolean
  valueClassName?: string
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/40 rounded-xl px-3 py-2.5'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-[11px] font-medium'>
        <Icon className='size-3 shrink-0' aria-hidden='true' />
        <span className='truncate'>{props.label}</span>
      </div>
      {props.loading ? (
        <Skeleton className='mt-1.5 h-5 w-16' />
      ) : (
        <div
          className={cn(
            'mt-1.5 font-mono text-sm font-semibold tabular-nums',
            props.valueClassName
          )}
        >
          {props.value}
        </div>
      )}
    </div>
  )
}
