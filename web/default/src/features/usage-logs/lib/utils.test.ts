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
import { describe, it, expect, vi, afterEach } from 'vitest'
import { getDefaultTimeRange } from './utils'

describe('getDefaultTimeRange', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('should set start to today 00:00:00', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-30T10:30:00'))

    const { start } = getDefaultTimeRange()

    expect(start.getFullYear()).toBe(2026)
    expect(start.getMonth()).toBe(4)
    expect(start.getDate()).toBe(30)
    expect(start.getHours()).toBe(0)
    expect(start.getMinutes()).toBe(0)
    expect(start.getSeconds()).toBe(0)
    expect(start.getMilliseconds()).toBe(0)
  })

  it('should set end to tomorrow 00:00 when current hour < 23', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-30T10:30:00'))

    const { end } = getDefaultTimeRange()

    expect(end.getFullYear()).toBe(2026)
    expect(end.getMonth()).toBe(4)
    expect(end.getDate()).toBe(31)
    expect(end.getHours()).toBe(0)
    expect(end.getMinutes()).toBe(0)
    expect(end.getSeconds()).toBe(0)
    expect(end.getMilliseconds()).toBe(0)
  })

  it('should set end to tomorrow 00:00 when current hour is 22', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-30T22:59:00'))

    const { end } = getDefaultTimeRange()

    expect(end.getDate()).toBe(31)
    expect(end.getHours()).toBe(0)
  })

  it('should set end to day after tomorrow 00:00 when current hour is 23', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-30T23:00:00'))

    const { end } = getDefaultTimeRange()

    expect(end.getFullYear()).toBe(2026)
    expect(end.getMonth()).toBe(5)
    expect(end.getDate()).toBe(1)
    expect(end.getHours()).toBe(0)
    expect(end.getMinutes()).toBe(0)
    expect(end.getSeconds()).toBe(0)
    expect(end.getMilliseconds()).toBe(0)
  })

  it('should set end to day after tomorrow 00:00 when current hour is 23:59', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-30T23:59:00'))

    const { end } = getDefaultTimeRange()

    expect(end.getFullYear()).toBe(2026)
    expect(end.getMonth()).toBe(5)
    expect(end.getDate()).toBe(1)
    expect(end.getHours()).toBe(0)
  })

  it('should handle month boundary correctly', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-01-31T15:00:00'))

    const { start, end } = getDefaultTimeRange()

    expect(start.getDate()).toBe(31)
    expect(start.getMonth()).toBe(0)

    expect(end.getDate()).toBe(1)
    expect(end.getMonth()).toBe(1)
  })

  it('should handle year boundary correctly when hour >= 23', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-12-31T23:30:00'))

    const { end } = getDefaultTimeRange()

    expect(end.getFullYear()).toBe(2027)
    expect(end.getMonth()).toBe(0)
    expect(end.getDate()).toBe(2)
    expect(end.getHours()).toBe(0)
  })
})
