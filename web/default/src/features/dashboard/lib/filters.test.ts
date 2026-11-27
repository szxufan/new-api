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
import { beforeEach, describe, expect, it } from 'vitest'
import {
  DASHBOARD_CHART_PREFERENCES_STORAGE_KEY,
  DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY,
  DEFAULT_DASHBOARD_CHART_PREFERENCES,
} from '@/features/dashboard/constants'
import { getSavedChartPreferences, saveChartPreferences } from './filters'

describe('dashboard chart preferences', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('persists quota_proportion as default analytics chart', () => {
    saveChartPreferences({
      ...DEFAULT_DASHBOARD_CHART_PREFERENCES,
      modelAnalyticsChart: 'quota_proportion',
    })

    expect(getSavedChartPreferences().modelAnalyticsChart).toBe(
      'quota_proportion'
    )
  })

  it('keeps channel preferences isolated from model preferences', () => {
    saveChartPreferences(
      { ...DEFAULT_DASHBOARD_CHART_PREFERENCES, defaultTimeRangeDays: 7 },
      DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY
    )

    expect(
      getSavedChartPreferences(DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY)
        .defaultTimeRangeDays
    ).toBe(7)
    // 默认（模型看板）key 不受影响
    expect(getSavedChartPreferences().defaultTimeRangeDays).toBe(
      DEFAULT_DASHBOARD_CHART_PREFERENCES.defaultTimeRangeDays
    )
    expect(
      localStorage.getItem(DASHBOARD_CHART_PREFERENCES_STORAGE_KEY)
    ).toBeNull()
  })

  it('falls back to defaults on corrupted stored preferences', () => {
    localStorage.setItem(
      DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY,
      '{bad json'
    )

    expect(
      getSavedChartPreferences(DASHBOARD_CHANNEL_CHART_PREFERENCES_STORAGE_KEY)
        .modelAnalyticsChart
    ).toBe(DEFAULT_DASHBOARD_CHART_PREFERENCES.modelAnalyticsChart)
  })

  // 回归：旧版本可保存“天粒度 + 1天范围”的非法组合，看板只显示 1 天数据
  it('rejects a default range shorter than the granularity requires', () => {
    saveChartPreferences({
      ...DEFAULT_DASHBOARD_CHART_PREFERENCES,
      defaultTimeGranularity: 'day',
      defaultTimeRangeDays: 1,
    })
    expect(getSavedChartPreferences().defaultTimeRangeDays).toBe(7)

    saveChartPreferences({
      ...DEFAULT_DASHBOARD_CHART_PREFERENCES,
      defaultTimeGranularity: 'week',
      defaultTimeRangeDays: 1,
    })
    expect(getSavedChartPreferences().defaultTimeRangeDays).toBe(30)

    // hour 粒度下 1 天是合法的；用户显式选的更大范围也应保留
    saveChartPreferences({
      ...DEFAULT_DASHBOARD_CHART_PREFERENCES,
      defaultTimeGranularity: 'hour',
      defaultTimeRangeDays: 1,
    })
    expect(getSavedChartPreferences().defaultTimeRangeDays).toBe(1)

    saveChartPreferences({
      ...DEFAULT_DASHBOARD_CHART_PREFERENCES,
      defaultTimeGranularity: 'day',
      defaultTimeRangeDays: 14,
    })
    expect(getSavedChartPreferences().defaultTimeRangeDays).toBe(14)
  })
})
