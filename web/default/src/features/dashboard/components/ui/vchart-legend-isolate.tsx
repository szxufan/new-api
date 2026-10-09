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
import { VChart } from '@visactor/react-vchart'
import { VCHART_OPTION } from '@/lib/vchart'
import { useSeriesLegendIsolate } from '@/features/dashboard/hooks/use-legend-isolate'

interface VChartLegendIsolateProps {
  /** 图表 spec（由 processChartData / processUserChartData 等生成） */
  spec: Record<string, unknown>
  /** React key，变化时重建图表实例（如切换 Tab / 主题 / 数据刷新） */
  chartKey: string
  /** 当前解析后的主题（'dark' | 'light'） */
  resolvedTheme: string
}

/**
 * 带「图例首次隔离 + 叠加多选」交互的 VChart 封装。
 *
 * 内部为每个实例持有独立的图例交互状态，可在同一页面渲染多个（如 /dashboard/users）。
 * 要求传入 spec 的图例使用 `makeSeriesLegend()`（`filter:false` + `selectMode:'multiple'`）生成。
 */
export function VChartLegendIsolate(props: VChartLegendIsolateProps) {
  const { handleReady, handleLegendItemClick } = useSeriesLegendIsolate()

  return (
    <VChart
      key={props.chartKey}
      spec={{
        ...props.spec,
        theme: props.resolvedTheme === 'dark' ? 'dark' : 'light',
        background: 'transparent',
      }}
      option={VCHART_OPTION}
      onReady={handleReady}
      onLegendItemClick={handleLegendItemClick}
    />
  )
}
