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

/**
 * 图表图例「首次点击隔离、后续点击叠加」交互的纯逻辑实现。
 *
 * ## 背景
 * VChart（底层 vrender）的离散图例 `selectMode` 只有两种合法值：
 * - `'multiple'`：默认全部选中（显示），点击某项只切换该项显隐 —— 只能做「减法」，
 *   无法「点一个只看这一个」。
 * - `'single'`（或其他非法值）：点击某项后只保留该项、其余全部取消 —— 每次只能看一个，
 *   无法叠加多个做对比。
 *
 * 两者都不满足产品诉求：**默认显示全部 → 点击某个模型只隔离出这一个 → 继续点击其它模型可逐个叠加显示。**
 * 因此需要接管图例点击事件（配合 `legends.filter = false` 关闭 VChart 的默认过滤），
 * 由本文件的纯函数计算「点击后应显示的集合」，再调用
 * `chartInstance.setLegendSelectedDataById(SERIES_LEGEND_ID, selected)` 应用。
 *
 * ## 如何拿到「被点击项」
 * 接管后我们仍保持 `selectMode: 'multiple'`，这样 VChart 每次点击只会翻转一个图例项，
 * 于是「上一次选中集」与「本次 `e.value`（VChart 计算出的新选中集）」之间的**对称差**
 * 恰好只含被点击的那一项。若对称差不止一个元素（异常情况），直接不接管，交回 VChart 默认行为。
 *
 * ## 状态机
 * `mode` 取值：
 * - `'all'`：当前处于「显示全部」的初始态。
 * - `'filter'`：当前处于「用户主动筛选」态，选中集是用户挑出来的子集。
 *
 * 点击某项 `clicked` 的转移规则：
 * - `mode === 'all'`：隔离 —— 新选中集 = `[clicked]`，进入 `'filter'`。
 * - `mode === 'filter'`：叠加/取消 —— `clicked` 在选中集里则移除，否则加入；
 *   若移除后集合为空，则回到 `'all'`（重新显示全部）。
 */

/** 所有接管图例交互的图表共用的图例 id（在 spec 的 `legends.id` 上设置）。 */
export const SERIES_LEGEND_ID = 'dashboard-series-legend'

export type LegendSelectionKey = string | number

export type LegendInteractionMode = 'all' | 'filter'

/**
 * 图例交互的当前状态。
 * - `prevSelected`：上一次实际生效的选中集（与 VChart 图例内部状态保持同步）。
 * - `allKeys`：全部图例项（按 VChart 给出的原始顺序）。
 * - `mode`：见文件头「状态机」说明。
 */
export interface LegendInteractionState {
  prevSelected: LegendSelectionKey[]
  allKeys: LegendSelectionKey[]
  mode: LegendInteractionMode
}

/**
 * 一次点击的计算结果。
 * `selected` 为点击后应当显示的选中集；`mode` 为转移后的新模式。
 * 返回 `null` 表示无法识别被点击项（对称差不唯一），此时应保持原状、不接管。
 */
export interface LegendSelectionResult {
  selected: LegendSelectionKey[]
  mode: LegendInteractionMode
}

const toKey = (value: LegendSelectionKey): string => String(value)

/** 求两个选中集的对称差（用字符串归一化比较，保持元素原始类型）。 */
function symmetricDifference(
  a: LegendSelectionKey[],
  b: LegendSelectionKey[]
): LegendSelectionKey[] {
  const aKeys = new Set(a.map(toKey))
  const bKeys = new Set(b.map(toKey))
  const onlyInB = b.filter((item) => !aKeys.has(toKey(item)))
  const onlyInA = a.filter((item) => !bKeys.has(toKey(item)))
  return [...onlyInB, ...onlyInA]
}

/**
 * 构建一个「显示全部」的初始状态。
 * @param allKeys 全部图例项（通常来自 VChart 初始化时的选中集，默认全选）。
 */
export function createInitialLegendState(
  allKeys: LegendSelectionKey[]
): LegendInteractionState {
  const snapshot = [...allKeys]
  return { prevSelected: snapshot, allKeys: snapshot, mode: 'all' }
}

/**
 * 计算图例点击后应显示的选中集及新的交互模式。
 *
 * @param state 点击前的交互状态
 * @param clickedKey 被点击的图例项
 */
export function computeLegendSelection(
  state: LegendInteractionState,
  clickedKey: LegendSelectionKey
): LegendSelectionResult {
  const { prevSelected, allKeys, mode } = state
  const clicked = toKey(clickedKey)

  if (mode === 'all') {
    // 首次点击：隔离出被点击项（保持 allKeys 的原始顺序与类型）。
    return {
      selected: allKeys.filter((item) => toKey(item) === clicked),
      mode: 'filter',
    }
  }

  // filter 态：对被点击项做增 / 删。
  const selectedKeys = new Set(prevSelected.map(toKey))
  if (selectedKeys.has(clicked)) {
    selectedKeys.delete(clicked)
  } else {
    selectedKeys.add(clicked)
  }

  const filtered = allKeys.filter((item) => selectedKeys.has(toKey(item)))

  if (filtered.length === 0) {
    // 全部取消 → 回到显示全部。
    return { selected: [...allKeys], mode: 'all' }
  }

  return { selected: filtered, mode: 'filter' }
}

/**
 * 处理一次图例点击：先由「上一次选中集」与「VChart 本次给出的选中集」反推出被点击项，
 * 再交给 {@link computeLegendSelection} 得到最终结果。
 *
 * @param state 点击前的交互状态
 * @param currentSelected VChart `legendItemClick` 事件里 `e.value` 携带的本次选中集
 * @returns 计算结果；对称差不唯一时返回 `null`（不接管）。
 */
export function resolveLegendClick(
  state: LegendInteractionState,
  currentSelected: LegendSelectionKey[]
): LegendSelectionResult | null {
  const diff = symmetricDifference(state.prevSelected, currentSelected)
  if (diff.length !== 1) {
    return null
  }
  return computeLegendSelection(state, diff[0])
}
