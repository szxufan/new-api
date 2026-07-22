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
import { type TFunction } from 'i18next'
import { type StatusBadgeProps } from '@/components/status-badge'

// ============================================================================
// Virtual Model Status Configuration
// ============================================================================

export const VIRTUAL_MODEL_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
} as const

export const VIRTUAL_MODEL_STATUS_VALUES = Object.values(
  VIRTUAL_MODEL_STATUS
).map((value) => String(value)) as `${number}`[]

// labelKey values are i18n keys; use t(config.labelKey) in components
export const VIRTUAL_MODEL_STATUSES: Record<
  number,
  Pick<StatusBadgeProps, 'variant' | 'showDot'> & {
    labelKey: string
    value: number
  }
> = {
  [VIRTUAL_MODEL_STATUS.ENABLED]: {
    labelKey: 'Enabled',
    variant: 'success',
    value: VIRTUAL_MODEL_STATUS.ENABLED,
    showDot: true,
  },
  [VIRTUAL_MODEL_STATUS.DISABLED]: {
    labelKey: 'Disabled',
    variant: 'neutral',
    value: VIRTUAL_MODEL_STATUS.DISABLED,
    showDot: true,
  },
} as const

export function getVirtualModelStatusOptions(t: TFunction) {
  return Object.values(VIRTUAL_MODEL_STATUSES).map((config) => ({
    label: t(config.labelKey),
    value: String(config.value),
  }))
}

// ============================================================================
// Virtual Model Mode Configuration
// ============================================================================

// labelKey values are i18n keys; use t(config.labelKey) in components
export const VIRTUAL_MODEL_MODE_CONFIG = {
  speed: {
    labelKey: 'Speed Mode',
    descriptionKey:
      'Concurrently request all sub-models and return the fastest response.',
    minTargets: 2,
  },
  quality: {
    labelKey: 'Quality Mode',
    descriptionKey:
      'Collect responses from all sub-models, then generate the final answer with the aggregator model.',
    minTargets: 1,
  },
} as const

// ============================================================================
// Success Messages (i18n keys; use t(SUCCESS_MESSAGES.xxx) when displaying)
// ============================================================================

export const SUCCESS_MESSAGES = {
  VIRTUAL_MODEL_CREATED: 'Virtual model created successfully',
  VIRTUAL_MODEL_UPDATED: 'Virtual model updated successfully',
  VIRTUAL_MODEL_DELETED: 'Virtual model deleted successfully',
  VIRTUAL_MODEL_ENABLED: 'Virtual model enabled successfully',
  VIRTUAL_MODEL_DISABLED: 'Virtual model disabled successfully',
} as const
