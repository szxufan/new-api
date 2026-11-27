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
import { describe, expect, it } from 'vitest'
import { buildApiPath, buildStatsPath, withTrailingSlash } from './api-path'

describe('withTrailingSlash', () => {
  it('should keep an endpoint that already ends with a slash', () => {
    expect(withTrailingSlash('/api/task/')).toBe('/api/task/')
  })

  it('should append a slash when it is missing', () => {
    expect(withTrailingSlash('/api/task')).toBe('/api/task/')
  })
})

describe('buildApiPath', () => {
  it('should return the group path for admins', () => {
    expect(buildApiPath('/api/task/', true)).toBe('/api/task/')
    expect(buildApiPath('/api/log/', true)).toBe('/api/log/')
  })

  it('should append the self sub-path for non-admins', () => {
    expect(buildApiPath('/api/task/', false)).toBe('/api/task/self')
    expect(buildApiPath('/api/log/', false)).toBe('/api/log/self')
  })

  // Regression: `/api/task` without the trailing slash produced
  // `/api/taskself`, which matches no Gin route and returned 404 for
  // every non-admin visiting /usage-logs/task.
  it('should normalise endpoints declared without a trailing slash', () => {
    expect(buildApiPath('/api/task', false)).toBe('/api/task/self')
    expect(buildApiPath('/api/mj', false)).toBe('/api/mj/self')
    expect(buildApiPath('/api/task', true)).toBe('/api/task/')
  })
})

describe('buildStatsPath', () => {
  it('should build the admin stats path', () => {
    expect(buildStatsPath('/api/log/', true)).toBe('/api/log/stat')
  })

  it('should build the self stats path for non-admins', () => {
    expect(buildStatsPath('/api/log/', false)).toBe('/api/log/self/stat')
  })

  it('should normalise endpoints declared without a trailing slash', () => {
    expect(buildStatsPath('/api/log', true)).toBe('/api/log/stat')
    expect(buildStatsPath('/api/log', false)).toBe('/api/log/self/stat')
  })
})
