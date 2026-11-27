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
import { api } from '@/lib/api'
import type {
  ChannelQuotaDataItem,
  QuotaDataItem,
  UptimeGroupResult,
} from './types'

// ============================================================================
// Dashboard APIs
// ============================================================================

// ----------------------------------------------------------------------------
// Quota & Usage Data
// ----------------------------------------------------------------------------

// Get user quota data within a time range
// Admin users get all users' data by default (matching classic frontend behavior)
export async function getUserQuotaDates(
  params: {
    start_timestamp: number
    end_timestamp: number
    default_time?: string
    username?: string
  },
  isAdmin = false
) {
  const endpoint = isAdmin ? '/api/data/' : '/api/data/self'
  const res = await api.get<{ success: boolean; data: QuotaDataItem[] }>(
    endpoint,
    { params }
  )
  return res.data
}

// Get channel quota stats within a time range (admin only).
// 数据来自 quota_data 预聚合表（含 channel_id 维度），与模型看板同源同保留期；
// 历史存量行无渠道信息，归入 channel_id=0（Unknown）。
export async function getChannelQuotaDates(params: {
  start_timestamp: number
  end_timestamp: number
  default_time?: string
  username?: string
}) {
  const res = await api.get<{
    success: boolean
    data: ChannelQuotaDataItem[]
  }>('/api/data/channels', { params })
  return res.data
}

// 预览重建：对比 logs 与 quota_data 的覆盖情况（管理员）
export async function getQuotaDataRebuildPreview() {
  const res = await api.get<{
    success: boolean
    message?: string
    data: {
      log_consume_rows: number
      log_first_created_at: number
      log_last_created_at: number
      quota_data_rows: number
      quota_data_first_created_at: number
      quota_data_last_created_at: number
    }
  }>('/api/data/rebuild/preview')
  return res.data
}

// 从消费日志全量重建 quota_data（管理员，破坏性操作）
export async function rebuildQuotaData() {
  const res = await api.post<{
    success: boolean
    message?: string
    data: { rows: number }
  }>('/api/data/rebuild?confirm=true')
  return res.data
}

// ----------------------------------------------------------------------------
// System Monitoring
// ----------------------------------------------------------------------------

export async function getUserQuotaDataByUsers(params: {
  start_timestamp: number
  end_timestamp: number
}) {
  const res = await api.get<{ success: boolean; data: QuotaDataItem[] }>(
    '/api/data/users',
    { params }
  )
  return res.data
}

export async function getUserQuotaDataByUserAndModel(params: {
  start_timestamp: number
  end_timestamp: number
}) {
  const res = await api.get<{ success: boolean; data: QuotaDataItem[] }>(
    '/api/data/users/models',
    { params }
  )
  return res.data
}

export async function getUptimeStatus() {
  const res = await api.get<{ success: boolean; data: UptimeGroupResult[] }>(
    '/api/uptime/status'
  )
  return res.data
}
