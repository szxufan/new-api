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
import { useState, useCallback, useMemo, lazy, Suspense } from 'react'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { FadeIn } from '@/components/page-transition'
import { ModelsChartPreferences } from './components/models/models-chart-preferences'
import { ModelsFilter } from './components/models/models-filter-dialog'
import { OverviewDashboard } from './components/overview/overview-dashboard'
import {
  DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY,
  DEFAULT_TIME_GRANULARITY,
} from './constants'
import {
  buildDefaultDashboardFilters,
  getSavedChartPreferences,
  saveChartPreferences,
} from './lib'
import {
  type DashboardSectionId,
  DASHBOARD_DEFAULT_SECTION,
  DASHBOARD_SECTION_IDS,
} from './section-registry'
import {
  type DashboardChartPreferences,
  type DashboardFilters,
  type QuotaDataItem,
} from './types'

const route = getRouteApi('/_authenticated/dashboard/$section')

const LazyLogStatCards = lazy(() =>
  import('./components/models/log-stat-cards').then((m) => ({
    default: m.LogStatCards,
  }))
)

const LazyModelCharts = lazy(() =>
  import('./components/models/model-charts').then((m) => ({
    default: m.ModelCharts,
  }))
)

const LazyConsumptionDistributionChart = lazy(() =>
  import('./components/models/consumption-distribution-chart').then((m) => ({
    default: m.ConsumptionDistributionChart,
  }))
)

const LazyPerformanceOverview = lazy(() =>
  import('./components/models/performance-overview').then((m) => ({
    default: m.PerformanceOverview,
  }))
)

const LazyUserCharts = lazy(() =>
  import('./components/users/user-charts').then((m) => ({
    default: m.UserCharts,
  }))
)

const LazyChannelStatCards = lazy(() =>
  import('./components/channels/channel-analytics').then((m) => ({
    default: m.ChannelStatCards,
  }))
)

const LazyChannelConsumptionDistributionChart = lazy(() =>
  import('./components/channels/channel-analytics').then((m) => ({
    default: m.ChannelConsumptionDistributionChart,
  }))
)

const LazyChannelCharts = lazy(() =>
  import('./components/channels/channel-analytics').then((m) => ({
    default: m.ChannelCharts,
  }))
)

function LogStatCardsFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-5'>
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className='px-4 py-3.5 sm:px-5 sm:py-4'>
            <Skeleton className='h-3.5 w-16' />
            <Skeleton className='mt-2 h-7 w-20' />
            <Skeleton className='mt-1.5 h-3.5 w-28' />
          </div>
        ))}
      </div>
    </div>
  )
}

function ModelChartsFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex items-center justify-between border-b px-4 py-3 sm:px-5'>
        <Skeleton className='h-5 w-32' />
        <Skeleton className='h-8 w-72' />
      </div>
      <div className='h-96 p-2'>
        <Skeleton className='h-full w-full' />
      </div>
    </div>
  )
}

function PerformanceOverviewFallback() {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3 sm:px-5'>
        <div className='flex items-center gap-2'>
          <Skeleton className='h-4 w-24' />
        </div>
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className='flex items-center gap-1.5'>
            <Skeleton className='h-3 w-14' />
            <Skeleton className='h-4 w-16' />
          </div>
        ))}
        <div className='ml-auto flex items-center gap-2'>
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className='h-5 w-28 rounded-full' />
          ))}
        </div>
      </div>
    </div>
  )
}

const SECTION_META: Record<
  DashboardSectionId,
  { titleKey: string; descriptionKey: string }
> = {
  overview: {
    titleKey: 'Overview',
    descriptionKey: 'View dashboard overview and statistics',
  },
  models: {
    titleKey: 'Model Call Analytics',
    descriptionKey: 'View model call count analytics and charts',
  },
  users: {
    titleKey: 'User Analytics',
    descriptionKey: 'View user consumption statistics and charts',
  },
  channels: {
    titleKey: 'Channel Call Analytics',
    descriptionKey: 'View channel call count analytics and charts',
  },
}

export function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = route.useParams()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const activeSection = (params.section ??
    DASHBOARD_DEFAULT_SECTION) as DashboardSectionId

  const [modelData, setModelData] = useState<QuotaDataItem[]>([])
  const [dataLoading, setDataLoading] = useState(false)
  const [chartPreferences, setChartPreferences] =
    useState<DashboardChartPreferences>(() => getSavedChartPreferences())
  const [modelFilters, setModelFilters] = useState<DashboardFilters>(() =>
    buildDefaultDashboardFilters(getSavedChartPreferences())
  )

  // 渠道看板独立的数据/筛选/偏好状态，localStorage key 与模型看板隔离
  const [channelData, setChannelData] = useState<QuotaDataItem[]>([])
  const [channelDataLoading, setChannelDataLoading] = useState(false)
  const [channelChartPreferences, setChannelChartPreferences] =
    useState<DashboardChartPreferences>(() =>
      getSavedChartPreferences(DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY)
    )
  const [channelFilters, setChannelFilters] = useState<DashboardFilters>(() =>
    buildDefaultDashboardFilters(
      getSavedChartPreferences(DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY)
    )
  )

  const handleChannelFilterChange = useCallback((filters: DashboardFilters) => {
    setChannelFilters(filters)
  }, [])

  const handleChannelResetFilters = useCallback(() => {
    setChannelFilters(buildDefaultDashboardFilters(channelChartPreferences))
  }, [channelChartPreferences])

  const handleChannelDataUpdate = useCallback(
    (data: QuotaDataItem[], loading: boolean) => {
      setChannelData(data)
      setChannelDataLoading(loading)
    },
    []
  )

  const handleChannelChartPreferencesChange = useCallback(
    (preferences: DashboardChartPreferences) => {
      setChannelChartPreferences(preferences)
      setChannelFilters(buildDefaultDashboardFilters(preferences))
      saveChartPreferences(
        preferences,
        DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY
      )
    },
    []
  )

  const handleFilterChange = useCallback((filters: DashboardFilters) => {
    setModelFilters(filters)
  }, [])

  const handleResetFilters = useCallback(() => {
    setModelFilters(buildDefaultDashboardFilters(chartPreferences))
  }, [chartPreferences])

  const handleDataUpdate = useCallback(
    (data: QuotaDataItem[], loading: boolean) => {
      setModelData(data)
      setDataLoading(loading)
    },
    []
  )

  const handleChartPreferencesChange = useCallback(
    (preferences: DashboardChartPreferences) => {
      setChartPreferences(preferences)
      setModelFilters(buildDefaultDashboardFilters(preferences))
      saveChartPreferences(preferences)
    },
    []
  )

  const meta = SECTION_META[activeSection] ?? SECTION_META.overview
  const isAdmin = Boolean(userRole && userRole >= ROLE.ADMIN)
  const visibleSections = useMemo(
    () =>
      DASHBOARD_SECTION_IDS.filter(
        (section) =>
          section !== 'overview' &&
          ((section !== 'users' && section !== 'channels') || isAdmin)
      ),
    [isAdmin]
  )
  const handleSectionChange = useCallback(
    (section: string) => {
      void navigate({
        to: '/dashboard/$section',
        params: { section: section as DashboardSectionId },
      })
    },
    [navigate]
  )
  const showSectionTabs =
    activeSection !== 'overview' && visibleSections.length > 1
  const modelActions =
    activeSection === 'models' ? (
      <>
        <ModelsChartPreferences
          preferences={chartPreferences}
          onPreferencesChange={handleChartPreferencesChange}
        />
        <ModelsFilter
          preferences={chartPreferences}
          onFilterChange={handleFilterChange}
          onReset={handleResetFilters}
        />
      </>
    ) : null

  const channelActions =
    activeSection === 'channels' ? (
      <>
        <ModelsChartPreferences
          preferences={channelChartPreferences}
          onPreferencesChange={handleChannelChartPreferencesChange}
          descriptionKey='Choose the default charts, range, and time granularity for channel analytics.'
          analyticsChartLabelKey='Default channel call chart'
        />
        <ModelsFilter
          preferences={channelChartPreferences}
          onFilterChange={handleChannelFilterChange}
          onReset={handleChannelResetFilters}
          titleKey='Filter Dashboard Channels'
          showUsernameFilter={false}
        />
      </>
    ) : null
  const sectionActions = modelActions ?? channelActions

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t(meta.titleKey)}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t(meta.descriptionKey)}
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='space-y-3 sm:space-y-4'>
          {activeSection !== 'overview' && (
            <div className='flex flex-wrap items-center justify-between gap-1.5 sm:gap-2'>
              {showSectionTabs ? (
                <Tabs value={activeSection} onValueChange={handleSectionChange}>
                  <TabsList className='group-data-horizontal/tabs:h-auto max-w-full flex-wrap justify-start'>
                    {visibleSections.map((section) => (
                      <TabsTrigger key={section} value={section}>
                        {t(SECTION_META[section].titleKey)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                </Tabs>
              ) : (
                <div />
              )}
              {sectionActions != null && (
                <div className='flex shrink-0 flex-wrap items-center gap-1.5 sm:gap-2'>
                  {sectionActions}
                </div>
              )}
            </div>
          )}
          {activeSection === 'overview' && <OverviewDashboard />}
          {activeSection === 'models' && (
            <>
              <FadeIn>
                <Suspense fallback={<LogStatCardsFallback />}>
                  <LazyLogStatCards
                    filters={modelFilters}
                    onDataUpdate={handleDataUpdate}
                  />
                </Suspense>
              </FadeIn>
              {isAdmin && (
                <FadeIn delay={0.05}>
                  <Suspense fallback={<PerformanceOverviewFallback />}>
                    <LazyPerformanceOverview />
                  </Suspense>
                </FadeIn>
              )}
              <FadeIn delay={0.1}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyConsumptionDistributionChart
                    data={modelData}
                    loading={dataLoading}
                    defaultChartType={
                      chartPreferences.consumptionDistributionChart
                    }
                    timeGranularity={
                      modelFilters.time_granularity || DEFAULT_TIME_GRANULARITY
                    }
                  />
                </Suspense>
              </FadeIn>
              <FadeIn delay={0.15}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyModelCharts
                    data={modelData}
                    loading={dataLoading}
                    defaultChartTab={chartPreferences.modelAnalyticsChart}
                    timeGranularity={
                      modelFilters.time_granularity || DEFAULT_TIME_GRANULARITY
                    }
                  />
                </Suspense>
              </FadeIn>
            </>
          )}
          {activeSection === 'users' && (
            <FadeIn>
              <Suspense fallback={<ModelChartsFallback />}>
                <LazyUserCharts />
              </Suspense>
            </FadeIn>
          )}
          {activeSection === 'channels' && (
            <>
              <FadeIn>
                <Suspense fallback={<LogStatCardsFallback />}>
                  <LazyChannelStatCards
                    filters={channelFilters}
                    onDataUpdate={handleChannelDataUpdate}
                  />
                </Suspense>
              </FadeIn>
              <FadeIn delay={0.05}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyChannelConsumptionDistributionChart
                    data={channelData}
                    loading={channelDataLoading}
                    defaultChartType={
                      channelChartPreferences.consumptionDistributionChart
                    }
                    timeGranularity={
                      channelFilters.time_granularity ||
                      DEFAULT_TIME_GRANULARITY
                    }
                  />
                </Suspense>
              </FadeIn>
              <FadeIn delay={0.1}>
                <Suspense fallback={<ModelChartsFallback />}>
                  <LazyChannelCharts
                    data={channelData}
                    loading={channelDataLoading}
                    defaultChartTab={
                      channelChartPreferences.modelAnalyticsChart
                    }
                    timeGranularity={
                      channelFilters.time_granularity ||
                      DEFAULT_TIME_GRANULARITY
                    }
                  />
                </Suspense>
              </FadeIn>
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
