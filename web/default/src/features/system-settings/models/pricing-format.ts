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
import Decimal from 'decimal.js'

const MAX_DECIMAL_PLACES = 12

function toDecimalOrNull(value: unknown): Decimal | null {
  if (
    value === '' ||
    value === null ||
    value === undefined ||
    value === false
  ) {
    return null
  }

  try {
    const input = value === true ? 1 : (value as Decimal.Value)
    const d = new Decimal(input)
    return d.isFinite() ? d : null
  } catch {
    return null
  }
}

export function formatPricingNumber(value: unknown): string {
  const decimal = toDecimalOrNull(value)
  if (decimal === null) return ''

  const rounded = decimal.toDecimalPlaces(
    MAX_DECIMAL_PLACES,
    Decimal.ROUND_HALF_UP
  )
  return rounded
    .toFixed(MAX_DECIMAL_PLACES)
    .replace(/(\.[0-9]*?)0+$/, '$1')
    .replace(/\.$/, '')
}
