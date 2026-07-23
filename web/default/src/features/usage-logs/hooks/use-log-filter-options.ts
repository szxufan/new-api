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
import { useQuery } from '@tanstack/react-query'
import { getGroups } from '@/features/users/api'
import { getChannels } from '@/features/channels/api'
import type { Channel } from '@/features/channels/types'

/**
 * 获取日志筛选用分组列表
 */
export function useGroupOptions() {
  return useQuery({
    queryKey: ['usage-logs', 'groups'],
    queryFn: async () => {
      const res = await getGroups()
      return res.data ?? []
    },
  })
}

/**
 * 获取日志筛选用渠道列表（仅 admin）
 */
export function useChannelOptions(enabled = true) {
  return useQuery({
    queryKey: ['usage-logs', 'channels'],
    queryFn: async () => {
      const res = await getChannels({ p: 1, page_size: 100 })
      return (res.data?.items ?? []) as Pick<Channel, 'id' | 'name'>[]
    },
    enabled,
  })
}
