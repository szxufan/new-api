import { describe, it, expect } from 'vitest'
import {
  CHANNEL_PRICE_FACTOR_KEYS,
  CHANNEL_PRICE_FACTOR_LABELS,
  getChannelPriceFactorEntries,
} from './format'

describe('getChannelPriceFactorEntries', () => {
  it('returns empty array for empty input', () => {
    expect(getChannelPriceFactorEntries(undefined)).toEqual([])
    expect(getChannelPriceFactorEntries(null)).toEqual([])
    expect(getChannelPriceFactorEntries({})).toEqual([])
  })

  it('returns entries in fixed display order', () => {
    const entries = getChannelPriceFactorEntries({
      audio_output: 4,
      input: 0.5,
      total: 0.9,
      time_window: 0.8,
    })

    expect(entries).toEqual([
      { key: 'total', labelKey: 'Total Factor', value: 0.9 },
      { key: 'time_window', labelKey: 'Time Window Factor', value: 0.8 },
      { key: 'input', labelKey: 'Input Factor', value: 0.5 },
      { key: 'audio_output', labelKey: 'Audio Output Factor', value: 4 },
    ])
  })

  it('keeps explicit zero factors', () => {
    const entries = getChannelPriceFactorEntries({ total: 0 })
    expect(entries).toEqual([
      { key: 'total', labelKey: 'Total Factor', value: 0 },
    ])
  })

  it('ignores unknown keys and invalid values', () => {
    const entries = getChannelPriceFactorEntries({
      total: 0.5,
      unknown_key: 2,
      // 运行时可能出现的脏数据
      input: Number.NaN,
      cache_read: '0.2' as unknown as number,
    })

    expect(entries).toEqual([
      { key: 'total', labelKey: 'Total Factor', value: 0.5 },
    ])
  })

  it('has a label for every supported key', () => {
    for (const key of CHANNEL_PRICE_FACTOR_KEYS) {
      expect(CHANNEL_PRICE_FACTOR_LABELS[key]).toBeTruthy()
    }
  })
})