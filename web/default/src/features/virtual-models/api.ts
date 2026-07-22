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
import type { ApiResponse, VirtualModel, VirtualModelPayload } from './types'

// ============================================================================
// Virtual Model Management
// ============================================================================

// Get all virtual models
export async function getVirtualModels(): Promise<ApiResponse<VirtualModel[]>> {
  const res = await api.get('/api/virtual_models/')
  return res.data
}

// Get single virtual model by ID
export async function getVirtualModel(
  id: number
): Promise<ApiResponse<VirtualModel>> {
  const res = await api.get(`/api/virtual_models/${id}`)
  return res.data
}

// Create virtual model
export async function createVirtualModel(
  data: VirtualModelPayload
): Promise<ApiResponse<VirtualModel>> {
  const res = await api.post('/api/virtual_models/', data)
  return res.data
}

// Update virtual model (full object, including status toggle)
export async function updateVirtualModel(
  data: VirtualModelPayload & { id: number }
): Promise<ApiResponse<VirtualModel>> {
  const res = await api.put('/api/virtual_models/', data)
  return res.data
}

// Delete virtual model
export async function deleteVirtualModel(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/virtual_models/${id}`)
  return res.data
}

// ============================================================================
// Channel Options (for "specify channel" dropdowns)
// ============================================================================

export interface ChannelOption {
  id: number
  name: string
}

// Get channel list for dropdowns (id + name only)
export async function getChannelOptions(): Promise<ChannelOption[]> {
  const res = await api.get('/api/channel/', {
    params: { p: 1, page_size: 1000 },
  })
  const data = res.data?.data
  const items = Array.isArray(data) ? data : data?.items || []
  return items.map((channel: { id: number; name: string }) => ({
    id: channel.id,
    name: channel.name,
  }))
}
