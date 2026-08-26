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
import { describe, it, expect } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { vendorsQueryKeys } from '../../lib'

describe('VendorMutateDialog - 编辑后详情缓存失效', () => {
  it('编辑 vendor 成功后应移除该 vendor 详情缓存', () => {
    const queryClient = new QueryClient()
    const vendorId = 10

    queryClient.setQueryData(vendorsQueryKeys.detail(vendorId), {
      success: true,
      data: { id: vendorId, name: 'Old Vendor' },
    })

    expect(queryClient.getQueryData(vendorsQueryKeys.detail(vendorId))).toBeTruthy()

    queryClient.removeQueries({
      queryKey: vendorsQueryKeys.detail(vendorId),
    })

    expect(queryClient.getQueryData(vendorsQueryKeys.detail(vendorId))).toBeUndefined()
  })
})
