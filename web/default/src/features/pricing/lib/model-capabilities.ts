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
import type { TFunction } from 'i18next'
import {
  getModelPurposeOptions,
  MODEL_PURPOSES,
} from '@/features/models/constants'
import type { Modality, PricingModel } from '../types'

// ----------------------------------------------------------------------------
// Model capability display helpers
// ----------------------------------------------------------------------------
//
// These read the capabilities configured in the model metadata (returned by
// `/api/pricing`). Nothing is inferred from the model name: an unset field is
// reported as "not configured" so the UI never shows made-up capabilities.

/** Non-text input types that can be enabled in the model metadata. */
export type ConfigurableInputModality = Extract<Modality, 'image' | 'video'>

const CONFIGURABLE_INPUT_MODALITIES: ConfigurableInputModality[] = [
  'image',
  'video',
]

/**
 * Non-text input types enabled for the model, in display order.
 * Empty means the model is not marked as multimodal.
 */
export function getConfiguredInputModalities(
  model: PricingModel
): ConfigurableInputModality[] {
  const enabled: Record<ConfigurableInputModality, boolean | undefined> = {
    image: model.support_image_input,
    video: model.support_video_input,
  }
  return CONFIGURABLE_INPUT_MODALITIES.filter((modality) => enabled[modality])
}

/**
 * The model whose capabilities `model` inherits, i.e. its billing follow
 * target when the backend resolved the capabilities from there. Undefined
 * when the capabilities are the model's own (or not configured at all).
 */
export function getInheritedCapabilityTarget(
  model: PricingModel
): string | undefined {
  return model.capability_source === 'follow' ? model.follow_target : undefined
}

/**
 * The model whose purposes `model` inherits, i.e. its billing follow target
 * when the backend resolved the purposes from there. Undefined when the
 * purposes are the model's own (or not configured at all).
 */
export function getInheritedPurposeTarget(
  model: PricingModel
): string | undefined {
  return model.purpose_source === 'follow' ? model.follow_target : undefined
}

/**
 * Purposes of a model de-duplicated and in their preset display order; values
 * without a preset keep their relative order after the preset ones.
 */
export function getOrderedPurposes(purposes: string[] | undefined): string[] {
  const rank = new Map<string, number>(
    MODEL_PURPOSES.map((purpose, index) => [purpose, index])
  )
  return [...new Set(purposes ?? [])]
    .map((purpose, index) => ({
      purpose,
      index,
      rank: rank.get(purpose) ?? Number.MAX_SAFE_INTEGER,
    }))
    .sort((a, b) => a.rank - b.rank || a.index - b.index)
    .map((entry) => entry.purpose)
}

/** Translated label of every preset purpose, keyed by purpose value. */
export function getPurposeLabels(t: TFunction): Record<string, string> {
  return Object.fromEntries(
    getModelPurposeOptions(t).map((option) => [option.value, option.label])
  )
}

/** Rank of well-known thinking levels, lowest effort first. */
const THINKING_LEVEL_RANK: Record<string, number> = {
  none: 0,
  minimal: 1,
  low: 2,
  medium: 3,
  high: 4,
  xhigh: 5,
  max: 6,
}

const UNRANKED_LEVEL = Number.MAX_SAFE_INTEGER

/**
 * Thinking levels selectable for the model, lowest effort first. Levels are
 * compared case-insensitively for ranking and de-duplication but keep the
 * spelling of their first occurrence; unknown custom levels stay in their
 * given order after the well-known ones.
 */
export function getThinkingLevels(model: PricingModel): string[] {
  const levels: string[] = []
  const seen = new Set<string>()
  for (const option of model.reasoning_options ?? []) {
    if (typeof option !== 'string') continue
    const level = option.trim()
    const key = level.toLowerCase()
    if (!level || seen.has(key)) continue
    seen.add(key)
    levels.push(level)
  }

  return levels
    .map((level, index) => ({
      level,
      index,
      rank: THINKING_LEVEL_RANK[level.toLowerCase()] ?? UNRANKED_LEVEL,
    }))
    .sort((a, b) => a.rank - b.rank || a.index - b.index)
    .map((entry) => entry.level)
}

/** Smallest / largest context window the context meter is scaled to. */
const METER_MIN_CONTEXT = 8_192
const METER_MAX_CONTEXT = 1_048_576

/**
 * Position of a context window on a log scale from 8K to 1M, clamped to
 * 0–1, for comparing context sizes across model cards at a glance.
 */
export function getContextMeterRatio(contextLength: number): number {
  if (!Number.isFinite(contextLength) || contextLength <= 0) return 0
  const ratio =
    Math.log2(contextLength / METER_MIN_CONTEXT) /
    Math.log2(METER_MAX_CONTEXT / METER_MIN_CONTEXT)
  return Math.min(1, Math.max(0, ratio))
}
