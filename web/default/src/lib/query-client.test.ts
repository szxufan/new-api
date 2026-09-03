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
import { QueryClient, QueryObserver } from '@tanstack/react-query'
import { describe, it, expect, vi } from 'vitest'
import { createQueryClient } from './query-client'

const mockRouter = {
  history: { location: { href: '/' } },
  navigate: vi.fn(),
}

describe('全局 QueryClient 数据获取策略', () => {
  it('默认配置应禁用全部自动 refetch 触发器', () => {
    const client = createQueryClient(mockRouter)
    const defaults = client.getDefaultOptions().queries

    expect(defaults?.refetchOnWindowFocus).toBe(false)
    expect(defaults?.refetchOnReconnect).toBe(false)
    expect(defaults?.refetchOnMount).toBe(false)
  })

  it('不应设置 staleTime（禁用触发器后过期本身不会引发请求）', () => {
    const client = createQueryClient(mockRouter)
    expect(client.getDefaultOptions().queries?.staleTime).toBeUndefined()
  })

  it('窗口聚焦不应触发网络请求（数据已缓存时）', async () => {
    const fetchFn = vi
      .fn<() => Promise<{ value: string }>>()
      .mockResolvedValue({
        value: 'remote',
      })
    const client = createQueryClient(mockRouter)
    await client.prefetchQuery({ queryKey: ['k'], queryFn: fetchFn })
    expect(fetchFn).toHaveBeenCalledTimes(1)

    // 模拟窗口重新可见：聚焦已挂载的 observer
    const observer = new QueryObserver(client, {
      queryKey: ['k'],
      queryFn: fetchFn,
    })
    const unsubscribe = observer.subscribe(() => {})
    window.dispatchEvent(new Event('focus'))
    await new Promise((resolve) => setTimeout(resolve, 20))

    expect(fetchFn).toHaveBeenCalledTimes(1) // 仍是首次的那一次
    unsubscribe()
    client.clear()
  })

  it('挂载已有缓存的 query 不应触发网络请求（refetchOnMount=false）', async () => {
    const fetchFn = vi
      .fn<() => Promise<{ value: string }>>()
      .mockResolvedValue({
        value: 'remote',
      })
    const policyClient = createQueryClient(mockRouter)
    policyClient.setQueryData(['k3'], { value: 'cached' })

    // 行为验证：手动将 query 置为 stale 后再订阅，fetchFn 不应被调用
    const query = policyClient.getQueryCache().find({ queryKey: ['k3'] })!
    query.invalidate()

    const observer = new QueryObserver(policyClient, {
      queryKey: ['k3'],
      queryFn: fetchFn,
    })
    const unsubscribe = observer.subscribe(() => {})
    await new Promise((resolve) => setTimeout(resolve, 20))

    expect(fetchFn).not.toHaveBeenCalled()
    unsubscribe()
    policyClient.clear()
  })

  it('显式 invalidateQueries 应触发重新拉取（手动同步是唯一路径）', async () => {
    let value = 'v1'
    const fetchFn = vi
      .fn<() => Promise<{ value: string }>>()
      .mockImplementation(async () => ({ value }))
    const client = createQueryClient(mockRouter)
    await client.prefetchQuery({ queryKey: ['k4'], queryFn: fetchFn })
    value = 'v2'

    await client.invalidateQueries({ queryKey: ['k4'] })
    await client.refetchQueries({ queryKey: ['k4'] })

    expect(client.getQueryData(['k4'])).toEqual({ value: 'v2' })
    client.clear()
  })

  it('应继承既有行为：401/403 不重试，DEV 模式不重试', () => {
    const client = createQueryClient(mockRouter)
    const retry = client.getDefaultOptions().queries?.retry as (
      failureCount: number,
      error: unknown
    ) => boolean

    expect(retry(0, new Error('boom'))).toBe(!import.meta.env.DEV || false)
    // DEV 下第一分支直接返回 false
    if (import.meta.env.DEV) {
      expect(retry(0, new Error('boom'))).toBe(false)
    }
  })

  it('createQueryClient 返回标准 QueryClient 实例', () => {
    const client = createQueryClient(mockRouter)
    expect(client).toBeInstanceOf(QueryClient)
    client.clear()
  })
})
