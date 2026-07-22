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
import { z } from 'zod'

// ============================================================================
// Virtual Model Schema & Types
// ============================================================================

export const VIRTUAL_MODEL_MODES = ['speed', 'quality'] as const

export type VirtualModelMode = (typeof VIRTUAL_MODEL_MODES)[number]

export const virtualModelTargetSchema = z.object({
  model: z.string(),
  channel_id: z.number().optional(), // 0 or empty = auto select by group
  group: z.string().optional(), // empty = follow requester group
})

export type VirtualModelTarget = z.infer<typeof virtualModelTargetSchema>

export const virtualModelAggregatorSchema = z.object({
  model: z.string(),
  channel_id: z.number().optional(),
  group: z.string().optional(),
  prompt_template: z.string().optional(),
})

export type VirtualModelAggregator = z.infer<
  typeof virtualModelAggregatorSchema
>

// Note: targets/aggregator are JSON strings serialized by the backend (gorm text fields)
export const virtualModelSchema = z.object({
  id: z.number(),
  name: z.string(),
  mode: z.enum(VIRTUAL_MODEL_MODES),
  targets: z.string(),
  aggregator: z.string(),
  head_start_stream_ms: z.number(), // speed mode: head start (ms) for streaming requests, 0 = disabled
  head_start_non_stream_ms: z.number(), // speed mode: head start (ms) for non-streaming requests, 0 = disabled
  status: z.number(), // 1: enabled, 2: disabled
  created_time: z.number(),
  updated_time: z.number(),
})

export type VirtualModel = z.infer<typeof virtualModelSchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface VirtualModelPayload {
  id?: number
  name: string
  mode: VirtualModelMode
  targets: string
  aggregator: string
  head_start_stream_ms: number
  head_start_non_stream_ms: number
  status: number
}

// ============================================================================
// Dialog Types
// ============================================================================

export type VirtualModelsDialogType = 'create' | 'update' | 'delete'
