import { describe, it, expect } from 'vitest'
import { formatPricingNumber } from './pricing-format'

describe('formatPricingNumber', () => {
  it('returns empty string for empty/ null/ undefined/ false', () => {
    expect(formatPricingNumber('')).toBe('')
    expect(formatPricingNumber(null)).toBe('')
    expect(formatPricingNumber(undefined)).toBe('')
    expect(formatPricingNumber(false)).toBe('')
  })

  it('returns empty string for NaN and Infinity', () => {
    expect(formatPricingNumber(NaN)).toBe('')
    expect(formatPricingNumber(Infinity)).toBe('')
    expect(formatPricingNumber(-Infinity)).toBe('')
  })

  it('formats integer values', () => {
    expect(formatPricingNumber(0)).toBe('0')
    expect(formatPricingNumber(1)).toBe('1')
    expect(formatPricingNumber(100)).toBe('100')
    expect(formatPricingNumber(-5)).toBe('-5')
  })

  it('formats decimal values without trailing zeros', () => {
    expect(formatPricingNumber(0.3)).toBe('0.3')
    expect(formatPricingNumber(1.5)).toBe('1.5')
    expect(formatPricingNumber(123.456)).toBe('123.456')
  })

  it('handles floating point arithmetic cleanly', () => {
    expect(formatPricingNumber(0.1 + 0.2)).toBe('0.3')
    expect(formatPricingNumber(0.1 * 0.2)).toBe('0.02')
  })

  it('handles string inputs', () => {
    expect(formatPricingNumber('42')).toBe('42')
    expect(formatPricingNumber('0.5')).toBe('0.5')
    expect(formatPricingNumber('3.140000000000')).toBe('3.14')
  })

  it('handles true as 1', () => {
    expect(formatPricingNumber(true)).toBe('1')
  })

  it('rounds to at most 12 decimal places', () => {
    expect(formatPricingNumber(1 / 3)).toBe('0.333333333333')
    expect(formatPricingNumber(2 / 3)).toBe('0.666666666667')
  })

  it('handles very small values', () => {
    expect(formatPricingNumber('0.000000000001')).toBe('0.000000000001')
    expect(formatPricingNumber(0.0000000000001)).toBe('0')
  })

  it('handles large values without scientific notation', () => {
    expect(formatPricingNumber(1e15)).toBe('1000000000000000')
    expect(formatPricingNumber(999999999999999)).toBe('999999999999999')
  })

  it('rejects non-numeric strings', () => {
    expect(formatPricingNumber('abc')).toBe('')
    expect(formatPricingNumber('12abc')).toBe('')
  })
})