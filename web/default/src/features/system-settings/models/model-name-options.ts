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
import type { ComboboxInputOption } from '@/components/ui/combobox-input'

/**
 * Build model name dropdown options for the "Add model pricing" editor.
 *
 * Candidates are models configured in enabled channels (including enabled
 * virtual models) that do not have pricing configured in the current draft.
 * Manual free-text input remains available, so this list is only a hint.
 *
 * @param channelModels models returned by GET /api/channel/models_enabled
 * @param pricedModelNames model names that already have pricing entries in
 *   the current settings draft
 * @returns sorted, de-duplicated combobox options ({ value === label })
 */
export function buildModelNameOptions(
  channelModels: string[] | undefined,
  pricedModelNames: Set<string>
): ComboboxInputOption[] {
  if (!channelModels || channelModels.length === 0) return []

  const seen = new Set<string>()
  const options: ComboboxInputOption[] = []

  for (const raw of channelModels) {
    const name = raw.trim()
    if (!name || pricedModelNames.has(name) || seen.has(name)) continue
    seen.add(name)
    options.push({ value: name, label: name })
  }

  return options.sort((a, b) => a.value.localeCompare(b.value))
}
