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
import type { TFunction } from 'i18next'
import { VIRTUAL_MODEL_MODE_CONFIG, VIRTUAL_MODEL_STATUS } from '../constants'
import {
  VIRTUAL_MODEL_MODES,
  type VirtualModel,
  type VirtualModelPayload,
  type VirtualModelTarget,
} from '../types'
import { parseAggregator, parseTargets } from './utils'

// ============================================================================
// Form Schema (use getVirtualModelFormSchema(t) in components for i18n messages)
// ============================================================================

export function getVirtualModelFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().min(1, t('Name is required')),
      mode: z.enum(VIRTUAL_MODEL_MODES),
      targets: z
        .array(
          z.object({
            model: z.string().min(1, t('Model name is required')),
            channel_id: z.number(),
            group: z.string(),
          })
        )
        .min(1, t('At least one sub-model is required')),
      aggregator_model: z.string(),
      aggregator_channel_id: z.number(),
      aggregator_group: z.string(),
      aggregator_prompt_template: z.string(),
      head_start_stream_ms: z
        .number()
        .min(0, t('Head start time cannot be negative')),
      head_start_non_stream_ms: z
        .number()
        .min(0, t('Head start time cannot be negative')),
      quality_trigger_count: z
        .number()
        .min(0, t('Trigger count cannot be negative')),
      quality_wait_ms: z
        .number()
        .min(0, t('Wait time cannot be negative')),
    })
    .superRefine((data, ctx) => {
      const modeConfig = VIRTUAL_MODEL_MODE_CONFIG[data.mode]
      if (data.targets.length < modeConfig.minTargets) {
        ctx.addIssue({
          code: 'custom',
          path: ['targets'],
          message:
            data.mode === 'speed'
              ? t('Speed mode requires at least 2 sub-models')
              : t('Quality mode requires at least 1 sub-model'),
        })
      }
      if (data.mode === 'quality' && !data.aggregator_model.trim()) {
        ctx.addIssue({
          code: 'custom',
          path: ['aggregator_model'],
          message: t('Aggregator model is required in quality mode'),
        })
      }
    })
}

export type VirtualModelFormValues = {
  name: string
  mode: (typeof VIRTUAL_MODEL_MODES)[number]
  targets: Array<{
    model: string
    channel_id: number
    group: string
  }>
  aggregator_model: string
  aggregator_channel_id: number
  aggregator_group: string
  aggregator_prompt_template: string
  head_start_stream_ms: number
  head_start_non_stream_ms: number
  quality_trigger_count: number
  quality_wait_ms: number
}

// ============================================================================
// Form Defaults
// ============================================================================

export const VIRTUAL_MODEL_FORM_DEFAULT_VALUES: VirtualModelFormValues = {
  name: '',
  mode: 'speed',
  targets: [
    { model: '', channel_id: 0, group: '' },
    { model: '', channel_id: 0, group: '' },
  ],
  aggregator_model: '',
  aggregator_channel_id: 0,
  aggregator_group: '',
  aggregator_prompt_template: '',
  head_start_stream_ms: 0,
  head_start_non_stream_ms: 0,
  quality_trigger_count: 1,
  quality_wait_ms: 0,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload (targets/aggregator as JSON strings)
 */
export function transformFormDataToPayload(
  data: VirtualModelFormValues,
  status: number = VIRTUAL_MODEL_STATUS.ENABLED
): VirtualModelPayload {
  const targets: VirtualModelTarget[] = data.targets.map((target) => ({
    model: target.model.trim(),
    channel_id: target.channel_id || 0,
    group: target.group.trim(),
  }))

  const aggregator = {
    model: data.aggregator_model.trim(),
    channel_id: data.aggregator_channel_id || 0,
    group: data.aggregator_group.trim(),
    prompt_template: data.aggregator_prompt_template,
  }

  return {
    name: data.name.trim(),
    mode: data.mode,
    targets: JSON.stringify(targets),
    aggregator: JSON.stringify(aggregator),
    head_start_stream_ms: data.head_start_stream_ms || 0,
    head_start_non_stream_ms: data.head_start_non_stream_ms || 0,
    quality_trigger_count: data.quality_trigger_count || 1,
    quality_wait_ms: data.quality_wait_ms || 0,
    status,
  }
}

/**
 * Transform virtual model data to form defaults
 */
export function transformVirtualModelToFormDefaults(
  virtualModel: VirtualModel
): VirtualModelFormValues {
  const targets = parseTargets(virtualModel.targets)
  const aggregator = parseAggregator(virtualModel.aggregator)

  return {
    name: virtualModel.name,
    mode: virtualModel.mode,
    targets: targets.map((target) => ({
      model: target.model || '',
      channel_id: target.channel_id || 0,
      group: target.group || '',
    })),
    aggregator_model: aggregator.model || '',
    aggregator_channel_id: aggregator.channel_id || 0,
    aggregator_group: aggregator.group || '',
    aggregator_prompt_template: aggregator.prompt_template || '',
    head_start_stream_ms: virtualModel.head_start_stream_ms || 0,
    head_start_non_stream_ms: virtualModel.head_start_non_stream_ms || 0,
    quality_trigger_count: virtualModel.quality_trigger_count || 1,
    quality_wait_ms: virtualModel.quality_wait_ms || 0,
  }
}
