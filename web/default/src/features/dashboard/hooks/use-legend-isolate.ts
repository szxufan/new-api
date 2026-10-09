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
import { useCallback, useRef } from 'react'
import type { IVChart } from '@visactor/vchart'
import {
  SERIES_LEGEND_ID,
  createInitialLegendState,
  resolveLegendClick,
  type LegendInteractionState,
  type LegendSelectionKey,
} from '../lib/legend-selection'

/**
 * VChart 实例上与本交互相关的少量方法（结构化子集，避免依赖完整 IVChart 类型）。
 */
interface LegendChartLike {
  getLegendSelectedDataById?: (id: string) => LegendSelectionKey[]
  getLegendDataById?: (id: string) => Array<Record<string, unknown>>
  setLegendSelectedDataById?: (
    id: string,
    selected: LegendSelectionKey[]
  ) => void
}

/** 读取图例的全部项：优先当前选中集（初始默认全选），回退到图例数据项。 */
function extractAllKeys(chart: LegendChartLike): LegendSelectionKey[] {
  const selected = chart.getLegendSelectedDataById?.(SERIES_LEGEND_ID)
  if (selected && selected.length > 0) {
    return [...selected]
  }
  const data = chart.getLegendDataById?.(SERIES_LEGEND_ID) ?? []
  return data
    .map(
      (datum) => (datum?.key ?? datum?.label) as LegendSelectionKey | undefined
    )
    .filter(
      (key): key is LegendSelectionKey => key !== undefined && key !== null
    )
}

/**
 * 图例「首次点击隔离、后续点击叠加」交互 hook。
 *
 * 用法：把 `handleReady` 传给 `<VChart onReady={...} />`，把 `handleLegendItemClick`
 * 传给 `<VChart onLegendItemClick={...} />`。图表 spec 的图例需由
 * `makeSeriesLegend()` 生成（`filter: false` + `selectMode: 'multiple'` + 固定 id）。
 *
 * 交互效果：
 * - 初始显示全部系列；
 * - 点击某系列 → 只隔离显示该系列；
 * - 继续点击其它系列 → 逐个叠加显示；点击已显示的系列 → 取消它；
 * - 全部取消 → 回到显示全部。
 *
 * 每当图表实例初始化（如切换 Tab、数据刷新导致重建）时，状态重置为「显示全部」。
 */
export function useSeriesLegendIsolate() {
  const chartRef = useRef<LegendChartLike | null>(null)
  const stateRef = useRef<LegendInteractionState>(createInitialLegendState([]))

  const handleReady = useCallback((instance: IVChart, isInitial: boolean) => {
    chartRef.current = instance as unknown as LegendChartLike
    // 仅在实例首次就绪（含重建）时重置为「显示全部」；
    // 非初始的 onReady（spec 增量更新）保留用户当前筛选，避免丢失选择。
    if (!isInitial) {
      return
    }
    stateRef.current = createInitialLegendState(
      extractAllKeys(chartRef.current)
    )
  }, [])

  const handleLegendItemClick = useCallback((event: { value?: unknown }) => {
    const chart = chartRef.current
    if (!chart?.setLegendSelectedDataById) {
      return
    }
    const current = Array.isArray(event?.value)
      ? (event.value as LegendSelectionKey[])
      : []
    const result = resolveLegendClick(stateRef.current, current)
    // 对称差不唯一（异常）时交回 VChart 默认行为，不接管。
    if (!result) {
      return
    }
    stateRef.current = {
      prevSelected: result.selected,
      allKeys: stateRef.current.allKeys,
      mode: result.mode,
    }
    chart.setLegendSelectedDataById(SERIES_LEGEND_ID, result.selected)
  }, [])

  return { handleReady, handleLegendItemClick }
}
