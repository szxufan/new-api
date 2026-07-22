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
import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import { type VirtualModel, type VirtualModelsDialogType } from '../types'

type VirtualModelsContextType = {
  open: VirtualModelsDialogType | null
  setOpen: (str: VirtualModelsDialogType | null) => void
  currentRow: VirtualModel | null
  setCurrentRow: React.Dispatch<React.SetStateAction<VirtualModel | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const VirtualModelsContext =
  React.createContext<VirtualModelsContextType | null>(null)

export function VirtualModelsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<VirtualModelsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<VirtualModel | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <VirtualModelsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </VirtualModelsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useVirtualModels = () => {
  const virtualModelsContext = React.useContext(VirtualModelsContext)

  if (!virtualModelsContext) {
    throw new Error(
      'useVirtualModels has to be used within <VirtualModelsProvider>'
    )
  }

  return virtualModelsContext
}
