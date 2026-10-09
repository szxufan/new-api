import { describe, it, expect, vi } from 'vitest'
import type { QuotaDataItem } from '@/features/dashboard/types'
import {
  processChartData,
  processUserChartData,
  processUserModelChartData,
} from './charts'
import { SERIES_LEGEND_ID } from './legend-selection'

vi.mock('@visactor/vchart/esm/theme/color-scheme/builtin/default', () => ({
  dataScheme: [
    { maxDomainLength: 3, scheme: ['#5B8FF9', '#5AD8A6', '#F6BD16'] },
    {
      maxDomainLength: 10,
      scheme: [
        '#5B8FF9',
        '#5AD8A6',
        '#F6BD16',
        '#E8684A',
        '#6DC8EC',
        '#9270CA',
        '#FF9D4D',
        '#269A99',
        '#FF99C3',
        '#5D7092',
      ],
    },
  ],
}))

vi.mock('@/lib/currency', () => ({
  getCurrencyDisplay: () => ({
    config: { quotaPerUnit: 1000 },
    meta: { kind: 'currency', symbol: '$', exchangeRate: 1 },
  }),
}))

const mockT = ((key: string) => key) as never

describe('processChartData - spec_quota_pie', () => {
  it('should use quota (not count) for spec_quota_pie values', () => {
    const data: QuotaDataItem[] = [
      { created_at: 1700000000, model_name: 'gpt-4', quota: 500000, count: 10 },
      { created_at: 1700000000, model_name: 'claude', quota: 300000, count: 5 },
    ]

    const result = processChartData(data, 'day', mockT, 'default')
    const values = result.spec_quota_pie.data[0].values

    expect(values).toHaveLength(2)
    expect(values[0]).toEqual({ type: 'gpt-4', value: 500000 })
    expect(values[1]).toEqual({ type: 'claude', value: 300000 })
  })

  it('should sort spec_quota_pie values descending by quota', () => {
    const data: QuotaDataItem[] = [
      { created_at: 1700000000, model_name: 'low', quota: 100, count: 50 },
      { created_at: 1700000000, model_name: 'high', quota: 900, count: 1 },
      { created_at: 1700000000, model_name: 'mid', quota: 500, count: 20 },
    ]

    const result = processChartData(data, 'day', mockT, 'default')
    const values = result.spec_quota_pie.data[0].values

    expect(values[0].type).toBe('high')
    expect(values[0].value).toBe(900)
    expect(values[1].type).toBe('mid')
    expect(values[2].type).toBe('low')
  })

  it('should return valid empty spec for spec_quota_pie when data is empty', () => {
    const result = processChartData([], 'day', mockT, 'default')

    expect(result.spec_quota_pie).toBeDefined()
    expect(result.spec_quota_pie.type).toBe('pie')
    expect(result.spec_quota_pie.data[0].values).toEqual([])
    expect(result.spec_quota_pie.valueField).toBe('value')
    expect(result.spec_quota_pie.categoryField).toBe('type')
  })

  it('should return valid empty spec for spec_quota_pie when data is null', () => {
    const result = processChartData(
      null as unknown as QuotaDataItem[],
      'day',
      mockT,
      'default'
    )

    expect(result.spec_quota_pie).toBeDefined()
    expect(result.spec_quota_pie.data[0].values).toEqual([])
  })

  it('should filter out zero-quota models from spec_quota_pie', () => {
    const data: QuotaDataItem[] = [
      { created_at: 1700000000, model_name: 'gpt-4', quota: 500000, count: 10 },
      { created_at: 1700000000, model_name: 'zero', quota: 0, count: 5 },
    ]

    const result = processChartData(data, 'day', mockT, 'default')
    const values = result.spec_quota_pie.data[0].values

    expect(values).toHaveLength(1)
    expect(values[0]).toEqual({ type: 'gpt-4', value: 500000 })
  })
})

// VChart / vrender 的离散图例只识别 'multiple'（多选）与 'single'（单选隔离）。
// 'multi' 是非法值，会被静默当成 'single'，这正是历史 bug 的成因，这里显式防回归。
const VALID_LEGEND_SELECT_MODES = ['multiple', 'single']

describe('processChartData - model legend isolate-then-multi', () => {
  // /dashboard/models 折线/柱状图的图例被接管：filter:false + selectMode:'multiple' + 固定 id，
  // 组件层据此实现「首次点击隔离、后续点击叠加」。
  const legendSeriesSpecKeys = [
    'spec_line',
    'spec_area',
    'spec_model_line',
    'spec_rank_bar',
  ] as const

  const sampleData: QuotaDataItem[] = [
    { created_at: 1700000000, model_name: 'gpt-4', quota: 500000, count: 10 },
    { created_at: 1700000000, model_name: 'claude', quota: 300000, count: 5 },
  ]

  const expectedLegend = expect.objectContaining({
    id: SERIES_LEGEND_ID,
    visible: true,
    filter: false,
    selectMode: 'multiple',
  })

  it('should configure taken-over legends for all model series charts', () => {
    const result = processChartData(sampleData, 'day', mockT, 'default')

    legendSeriesSpecKeys.forEach((key) => {
      expect(result[key].legends).toEqual(expectedLegend)
    })
  })

  it('should keep taken-over legend config when data is empty', () => {
    const result = processChartData([], 'day', mockT, 'default')

    legendSeriesSpecKeys.forEach((key) => {
      expect(result[key].legends).toEqual(expectedLegend)
    })
  })

  it('should never use the invalid selectMode value "multi"', () => {
    const result = processChartData(sampleData, 'day', mockT, 'default')

    legendSeriesSpecKeys.forEach((key) => {
      const { selectMode } = result[key].legends
      if (selectMode !== undefined) {
        expect(VALID_LEGEND_SELECT_MODES).toContain(selectMode)
      }
    })
  })

  it('should leave pie chart legends on the VChart default (multiple) mode', () => {
    const result = processChartData(sampleData, 'day', mockT, 'default')

    expect(result.spec_pie.legends.selectMode).toBeUndefined()
    expect(result.spec_quota_pie.legends.selectMode).toBeUndefined()
  })
})

describe('processUserChartData - user chart legend multi-select', () => {
  // /dashboard/users 页面：用户消费趋势图（系列 = 用户）与用户模型排行图
  // （系列 = 模型）都希望通过图例叠加显示多个系列，而不是「点一个取消上一个」。
  const userData: QuotaDataItem[] = [
    {
      created_at: 1700000000,
      username: 'alice',
      model_name: 'gpt-4',
      quota: 500000,
      count: 10,
    },
    {
      created_at: 1700000000,
      username: 'bob',
      model_name: 'gpt-4',
      quota: 300000,
      count: 5,
    },
  ]

  it('should configure taken-over legend for spec_user_trend', () => {
    const result = processUserChartData(userData, 'day', mockT, 10, 'default')

    expect(result.spec_user_trend.legends).toEqual(
      expect.objectContaining({
        id: SERIES_LEGEND_ID,
        visible: true,
        filter: false,
        selectMode: 'multiple',
      })
    )
  })

  it('should keep taken-over legend config for spec_user_trend when data is empty', () => {
    const result = processUserChartData([], 'day', mockT, 10, 'default')

    expect(result.spec_user_trend.legends).toEqual(
      expect.objectContaining({
        id: SERIES_LEGEND_ID,
        visible: true,
        filter: false,
        selectMode: 'multiple',
      })
    )
  })

  it('should not use the invalid "multi" value for spec_user_trend', () => {
    const result = processUserChartData(userData, 'day', mockT, 10, 'default')

    expect(VALID_LEGEND_SELECT_MODES).toContain(
      result.spec_user_trend.legends.selectMode
    )
  })
})

describe('processUserModelChartData', () => {
  const normalData: QuotaDataItem[] = [
    {
      username: 'alice',
      model_name: 'gpt-4',
      quota: 500000,
      count: 10,
      created_at: 0,
    },
    {
      username: 'alice',
      model_name: 'claude',
      quota: 300000,
      count: 5,
      created_at: 0,
    },
    {
      username: 'bob',
      model_name: 'gpt-4',
      quota: 200000,
      count: 8,
      created_at: 0,
    },
    {
      username: 'bob',
      model_name: 'claude',
      quota: 100000,
      count: 3,
      created_at: 0,
    },
    {
      username: 'charlie',
      model_name: 'gpt-4',
      quota: 50000,
      count: 2,
      created_at: 0,
    },
  ]

  it('should return spec with xField Quota for quota rank chart', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')

    expect(result.spec_user_model_quota_rank).toBeDefined()
    expect(result.spec_user_model_quota_rank.type).toBe('bar')
    expect(result.spec_user_model_quota_rank.xField).toBe('Quota')
  })

  it('should return spec with xField Count for count rank chart', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')

    expect(result.spec_user_model_count_rank).toBeDefined()
    expect(result.spec_user_model_count_rank.type).toBe('bar')
    expect(result.spec_user_model_count_rank.xField).toBe('Count')
  })

  it('should sort data by user total quota descending', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')
    const quotaData = result.spec_user_model_quota_rank.data[0].values

    const userOrder = quotaData.map((d: { User: string }) => d.User)
    const firstAlice = userOrder.indexOf('alice')
    const firstBob = userOrder.indexOf('bob')
    const firstCharlie = userOrder.indexOf('charlie')

    expect(firstAlice).toBeLessThan(firstBob)
    expect(firstBob).toBeLessThan(firstCharlie)
  })

  it('should respect Top N user limit', () => {
    const result = processUserModelChartData(normalData, mockT, 2, 'default')
    const quotaData = result.spec_user_model_quota_rank.data[0].values
    const users = new Set(quotaData.map((d: { User: string }) => d.User))

    expect(users.size).toBe(2)
    expect(users.has('alice')).toBe(true)
    expect(users.has('bob')).toBe(true)
    expect(users.has('charlie')).toBe(false)
  })

  it('should also limit count rank to Top N users', () => {
    const result = processUserModelChartData(normalData, mockT, 2, 'default')
    const countData = result.spec_user_model_count_rank.data[0].values
    const users = new Set(countData.map((d: { User: string }) => d.User))

    expect(users.size).toBe(2)
    expect(users.has('alice')).toBe(true)
    expect(users.has('bob')).toBe(true)
  })

  it('should return valid empty specs when data is empty', () => {
    const result = processUserModelChartData([], mockT, 10, 'default')

    expect(result.spec_user_model_quota_rank).toBeDefined()
    expect(result.spec_user_model_quota_rank.type).toBe('bar')
    expect(result.spec_user_model_quota_rank.data[0].values).toEqual([])
    expect(result.spec_user_model_quota_rank.xField).toBe('Quota')
    expect(result.spec_user_model_quota_rank.yField).toBe('User')
    expect(result.spec_user_model_quota_rank.direction).toBe('horizontal')

    expect(result.spec_user_model_count_rank).toBeDefined()
    expect(result.spec_user_model_count_rank.type).toBe('bar')
    expect(result.spec_user_model_count_rank.data[0].values).toEqual([])
    expect(result.spec_user_model_count_rank.xField).toBe('Count')
    expect(result.spec_user_model_count_rank.yField).toBe('User')
    expect(result.spec_user_model_count_rank.direction).toBe('horizontal')
  })

  it('should return valid empty specs when data is null', () => {
    const result = processUserModelChartData(
      null as unknown as QuotaDataItem[],
      mockT,
      10,
      'default'
    )

    expect(result.spec_user_model_quota_rank).toBeDefined()
    expect(result.spec_user_model_count_rank).toBeDefined()
    expect(result.spec_user_model_quota_rank.data[0].values).toEqual([])
    expect(result.spec_user_model_count_rank.data[0].values).toEqual([])
  })

  it('should have correct quota values in quota rank chart', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')
    const quotaData = result.spec_user_model_quota_rank.data[0].values

    const aliceGpt4 = quotaData.find(
      (d: { User: string; Model: string }) =>
        d.User === 'alice' && d.Model === 'gpt-4'
    )
    expect(aliceGpt4).toBeDefined()
    expect(aliceGpt4.Quota).toBe(500000)
  })

  it('should have correct count values in count rank chart', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')
    const countData = result.spec_user_model_count_rank.data[0].values

    const aliceGpt4 = countData.find(
      (d: { User: string; Model: string }) =>
        d.User === 'alice' && d.Model === 'gpt-4'
    )
    expect(aliceGpt4).toBeDefined()
    expect(aliceGpt4.Count).toBe(10)
  })

  it('should set horizontal direction for rank charts', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')

    expect(result.spec_user_model_quota_rank.direction).toBe('horizontal')
    expect(result.spec_user_model_count_rank.direction).toBe('horizontal')
  })

  it('should handle single user single model data', () => {
    const singleData: QuotaDataItem[] = [
      {
        username: 'solo',
        model_name: 'gpt-4',
        quota: 1000,
        count: 1,
        created_at: 0,
      },
    ]

    const result = processUserModelChartData(singleData, mockT, 10, 'default')

    const quotaData = result.spec_user_model_quota_rank.data[0].values
    expect(quotaData).toHaveLength(1)
    expect(quotaData[0]).toEqual({
      User: 'solo',
      Model: 'gpt-4',
      Quota: 1000,
    })

    const countData = result.spec_user_model_count_rank.data[0].values
    expect(countData).toHaveLength(1)
    expect(countData[0]).toEqual({
      User: 'solo',
      Model: 'gpt-4',
      Count: 1,
    })
  })

  it('should pad zero for missing (user, model) combinations to enable stacking', () => {
    const data: QuotaDataItem[] = [
      {
        username: 'alice',
        model_name: 'gpt-4',
        quota: 500000,
        count: 10,
        created_at: 0,
      },
      {
        username: 'alice',
        model_name: 'claude',
        quota: 300000,
        count: 5,
        created_at: 0,
      },
      {
        username: 'bob',
        model_name: 'gpt-4',
        quota: 200000,
        count: 8,
        created_at: 0,
      },
    ]

    const result = processUserModelChartData(data, mockT, 10, 'default')
    const quotaData = result.spec_user_model_quota_rank.data[0].values

    expect(quotaData).toHaveLength(4)

    const aliceGpt4 = quotaData.find(
      (d: { User: string; Model: string }) =>
        d.User === 'alice' && d.Model === 'gpt-4'
    )
    const aliceClaude = quotaData.find(
      (d: { User: string; Model: string }) =>
        d.User === 'alice' && d.Model === 'claude'
    )
    const bobGpt4 = quotaData.find(
      (d: { User: string; Model: string }) =>
        d.User === 'bob' && d.Model === 'gpt-4'
    )
    const bobClaude = quotaData.find(
      (d: { User: string; Model: string }) =>
        d.User === 'bob' && d.Model === 'claude'
    )

    expect(aliceGpt4).toBeDefined()
    expect(aliceGpt4.Quota).toBe(500000)
    expect(aliceClaude).toBeDefined()
    expect(aliceClaude.Quota).toBe(300000)
    expect(bobGpt4).toBeDefined()
    expect(bobGpt4.Quota).toBe(200000)
    expect(bobClaude).toBeDefined()
    expect(bobClaude.Quota).toBe(0)
  })

  it('should use color.specified (Record<string, string>) for model color mapping', () => {
    const result = processUserModelChartData(normalData, mockT, 10, 'default')

    const quotaColor = result.spec_user_model_quota_rank.color
    expect(quotaColor).toBeDefined()
    expect(quotaColor).toHaveProperty('specified')
    expect(typeof quotaColor.specified).toBe('object')

    const countColor = result.spec_user_model_count_rank.color
    expect(countColor).toBeDefined()
    expect(countColor).toHaveProperty('specified')
    expect(typeof countColor.specified).toBe('object')
  })
})
