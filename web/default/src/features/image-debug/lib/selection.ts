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
import type { ImageMode } from '../types'

/** 按模式返回需要过滤的端点类型：文生图 → image-generation；图生图 → image-edit */
export function getModeEndpoints(mode: ImageMode): string[] {
  return mode === 'edits' ? ['image-edit'] : ['image-generation']
}

/** 当前选中值不在选项中时回退到第一个选项；选项为空时返回 fallback */
export function resolveSelection(
  selected: string,
  options: { value: string }[],
  fallback: string
): string {
  if (options.some((option) => option.value === selected)) return selected
  return options[0]?.value ?? fallback
}