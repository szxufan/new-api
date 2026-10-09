import { describe, it, expect } from 'vitest'
import {
  createInitialLegendState,
  computeLegendSelection,
  resolveLegendClick,
  type LegendInteractionState,
} from './legend-selection'

// vrender 在 selectMode:'multiple' 下点击某项只会翻转该项，
// 因此「上次选中集」与「本次 currentSelected」的对称差恰好是被点击项。
// 下列测试用 toggle 结果模拟 VChart 传入的 currentSelected。

const ALL = ['gpt-4', 'claude', 'gemini']

function toggleSelected(prevSelected: (string | number)[], clicked: string) {
  const set = new Set(prevSelected.map(String))
  if (set.has(clicked)) set.delete(clicked)
  else set.add(clicked)
  return ALL.filter((k) => set.has(k))
}

function advance(
  state: LegendInteractionState,
  clicked: string
): LegendInteractionState {
  const currentSelected = toggleSelected(state.prevSelected, clicked)
  const result = resolveLegendClick(state, currentSelected)
  if (!result) return state
  return {
    prevSelected: result.selected,
    allKeys: state.allKeys,
    mode: result.mode,
  }
}

describe('legend-selection - first click isolates', () => {
  it('first click from "all" isolates the clicked series', () => {
    const state = createInitialLegendState(ALL)
    const result = resolveLegendClick(state, toggleSelected(ALL, 'claude'))

    expect(result).not.toBeNull()
    expect(result?.selected).toEqual(['claude'])
    expect(result?.mode).toBe('filter')
  })
})

describe('legend-selection - subsequent clicks toggle additively', () => {
  it('isolates then accumulates multiple series', () => {
    let state = createInitialLegendState(ALL)

    state = advance(state, 'gpt-4')
    expect(state.prevSelected).toEqual(['gpt-4'])
    expect(state.mode).toBe('filter')

    state = advance(state, 'gemini')
    expect(state.prevSelected).toEqual(['gpt-4', 'gemini'])
    expect(state.mode).toBe('filter')

    state = advance(state, 'claude')
    expect(state.prevSelected).toEqual(['gpt-4', 'claude', 'gemini'])
    expect(state.mode).toBe('filter')
  })

  it('clicking a visible series hides it again (toggle off)', () => {
    let state = createInitialLegendState(ALL)
    state = advance(state, 'gpt-4') // ['gpt-4']
    state = advance(state, 'gemini') // ['gpt-4','gemini']

    state = advance(state, 'gpt-4') // remove gpt-4 -> ['gemini']
    expect(state.prevSelected).toEqual(['gemini'])
    expect(state.mode).toBe('filter')
  })

  it('returns to "all" when the last visible series is toggled off', () => {
    let state = createInitialLegendState(ALL)
    state = advance(state, 'claude') // ['claude']
    state = advance(state, 'claude') // remove -> back to all
    expect(state.mode).toBe('all')
    expect(state.prevSelected).toEqual(ALL)
  })
})

describe('legend-selection - keeps original key order and type', () => {
  it('isolated selection preserves allKeys order', () => {
    const ordered = ['b', 'a', 'c']
    const state = createInitialLegendState(ordered)
    // currentSelected 顺序即使不同，结果也按 allKeys 顺序输出
    const result = computeLegendSelection(
      { ...state, mode: 'filter', prevSelected: ['a'] },
      'b'
    )
    expect(result.selected).toEqual(['b', 'a'])
  })

  it('supports numeric keys', () => {
    const numeric = [1, 2, 3]
    const state = createInitialLegendState(numeric)
    const result = computeLegendSelection(state, 2)
    expect(result.selected).toEqual([2])
    expect(result.mode).toBe('filter')
  })
})

describe('legend-selection - guards against ambiguous clicks', () => {
  it('returns null when symmetric difference is not exactly one item', () => {
    const state = createInitialLegendState(ALL)
    // 与 prev 差两个元素，无法判断被点击项
    const result = resolveLegendClick(state, ['gpt-4'])
    expect(result).toBeNull()
  })

  it('returns null when nothing changed', () => {
    const state = createInitialLegendState(ALL)
    const result = resolveLegendClick(state, [...ALL])
    expect(result).toBeNull()
  })
})
