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
import type { TimeGranularity } from '@/lib/time'
import { getChannelQuotaDates } from '@/features/dashboard/api'
import { getChannelSeriesName } from '@/features/dashboard/lib'
import type {
  ConsumptionDistributionChartType,
  DashboardFilters,
  ModelAnalyticsChartTab,
  QuotaDataItem,
} from '@/features/dashboard/types'
import { ConsumptionDistributionChart } from '../models/consumption-distribution-chart'
import { LogStatCards } from '../models/log-stat-cards'
import { ModelCharts } from '../models/model-charts'

/**
 * 渠道调用分析看板：薄包装复用模型看板组件，仅替换数据源与系列维度。
 * 数据来自 /api/data/channels（quota_data 按渠道 + 小时桶聚合），仅管理员可访问。
 */

interface ChannelStatCardsProps {
  filters?: DashboardFilters
  onDataUpdate?: (data: QuotaDataItem[], loading: boolean) => void
}

export function ChannelStatCards(props: ChannelStatCardsProps) {
  return (
    <LogStatCards
      filters={props.filters}
      onDataUpdate={props.onDataUpdate}
      fetchData={getChannelQuotaDates}
    />
  )
}

interface ChannelConsumptionDistributionChartProps {
  data: QuotaDataItem[]
  loading?: boolean
  timeGranularity?: TimeGranularity
  defaultChartType?: ConsumptionDistributionChartType
}

export function ChannelConsumptionDistributionChart(
  props: ChannelConsumptionDistributionChartProps
) {
  return (
    <ConsumptionDistributionChart
      data={props.data}
      loading={props.loading}
      timeGranularity={props.timeGranularity}
      defaultChartType={props.defaultChartType}
      getSeriesName={getChannelSeriesName}
    />
  )
}

interface ChannelChartsProps {
  data: QuotaDataItem[]
  loading?: boolean
  timeGranularity?: TimeGranularity
  defaultChartTab?: ModelAnalyticsChartTab
}

export function ChannelCharts(props: ChannelChartsProps) {
  return (
    <ModelCharts
      data={props.data}
      loading={props.loading}
      timeGranularity={props.timeGranularity}
      defaultChartTab={props.defaultChartTab}
      titleKey='Channel Call Analytics'
      getSeriesName={getChannelSeriesName}
    />
  )
}
