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
// ============================================================================
// MCP setting (group → model map) validation utilities
// ============================================================================

export type GroupModelRow = {
  id: string
  group: string
  model: string
}

/**
 * Parse a group → model JSON map string into editor rows.
 * Returns null when the value is not a valid JSON object
 * (callers typically treat null as "start from empty").
 */
export function parseGroupModelMap(json: string): GroupModelRow[] | null {
  try {
    if (!json.trim()) {
      return []
    }
    const parsed = JSON.parse(json)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }
    return Object.entries(parsed).map(([group, model], index) => ({
      id: `row-${index}-${group}`,
      group,
      model: String(model),
    }))
  } catch {
    return null
  }
}

/**
 * Convert editor rows back to a group → model JSON map string.
 * Rows with an empty group name are dropped; the result is
 * pretty-printed with 2-space indent. Empty input yields '{}'.
 */
export function convertGroupModelRowsToJson(rows: GroupModelRow[]): string {
  const obj: Record<string, string> = {}
  rows.forEach((row) => {
    if (row.group.trim()) {
      obj[row.group.trim()] = row.model.trim()
    }
  })
  return JSON.stringify(obj, null, 2)
}
