import { describe, it, expect, vi } from 'vitest'
import {
  formatBalance,
  getBalanceVariant,
  formatQuota,
  isTagAggregateRow,
  type TagRow,
} from './channel-utils'

vi.mock('@/lib/currency', () => ({
  formatCurrencyFromUSD: (value: number) => `$${value.toFixed(2)}`,
  formatQuotaWithCurrency: (value: number) => `${value} quota`,
}))

describe('formatBalance', () => {
  it('should format positive balance correctly', () => {
    const result = formatBalance(100)
    expect(result).toBe('$100.00')
  })

  it('should format zero balance', () => {
    const result = formatBalance(0)
    expect(result).toBe('$0.00')
  })

  it('should return dash for null balance', () => {
    const result = formatBalance(null)
    expect(result).toBe('-')
  })

  it('should return dash for undefined balance', () => {
    const result = formatBalance(undefined)
    expect(result).toBe('-')
  })

  it('should return dash for NaN', () => {
    const result = formatBalance(NaN)
    expect(result).toBe('-')
  })
})

describe('getBalanceVariant', () => {
  it('should return neutral for zero balance', () => {
    expect(getBalanceVariant(0)).toBe('neutral')
  })

  it('should return danger for balance less than 1', () => {
    expect(getBalanceVariant(0.5)).toBe('danger')
    expect(getBalanceVariant(0.99)).toBe('danger')
  })

  it('should return warning for balance between 1 and 10', () => {
    expect(getBalanceVariant(1)).toBe('warning')
    expect(getBalanceVariant(5)).toBe('warning')
    expect(getBalanceVariant(9.99)).toBe('warning')
  })

  it('should return success for balance 10 or greater', () => {
    expect(getBalanceVariant(10)).toBe('success')
    expect(getBalanceVariant(100)).toBe('success')
    expect(getBalanceVariant(1000)).toBe('success')
  })

  it('should handle negative balance as danger', () => {
    expect(getBalanceVariant(-5)).toBe('danger')
  })
})

describe('formatQuota', () => {
  it('should format quota with currency', () => {
    const result = formatQuota(1000)
    expect(result).toBe('1000 quota')
  })

  it('should format zero quota', () => {
    const result = formatQuota(0)
    expect(result).toBe('0 quota')
  })

  it('should format large quota values', () => {
    const result = formatQuota(1000000)
    expect(result).toBe('1000000 quota')
  })
})

describe('isTagAggregateRow', () => {
  it('should return true for tag aggregate rows with children array', () => {
    const tagRow = {
      id: 1,
      children: [],
    } as unknown as TagRow
    expect(isTagAggregateRow(tagRow)).toBe(true)
  })

  it('should return true for tag aggregate rows with children', () => {
    const tagRow = {
      id: 1,
      children: [{ id: 2 }],
    } as unknown as TagRow
    expect(isTagAggregateRow(tagRow)).toBe(true)
  })

  it('should return false for regular channel rows without children', () => {
    const channel = {
      id: 1,
    } as any
    expect(isTagAggregateRow(channel)).toBe(false)
  })

  it('should return false when children is not an array', () => {
    const channel = {
      id: 1,
      children: 'not-an-array',
    } as any
    expect(isTagAggregateRow(channel)).toBe(false)
  })

  it('should return false when children is null', () => {
    const channel = {
      id: 1,
      children: null,
    } as any
    expect(isTagAggregateRow(channel)).toBe(false)
  })
})
