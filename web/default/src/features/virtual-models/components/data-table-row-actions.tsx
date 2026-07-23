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
import { type Row } from '@tanstack/react-table'
import {
  Trash2,
  Edit,
  Power,
  PowerOff,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { updateVirtualModel } from '../api'
import { SUCCESS_MESSAGES, VIRTUAL_MODEL_STATUS } from '../constants'
import { virtualModelSchema } from '../types'
import { useVirtualModels } from './virtual-models-provider'

interface DataTableRowActionsProps<TData> {
  row: Row<TData>
}

export function DataTableRowActions<TData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const virtualModel = virtualModelSchema.parse(row.original)
  const { setOpen, setCurrentRow, triggerRefresh } = useVirtualModels()
  const isEnabled = virtualModel.status === VIRTUAL_MODEL_STATUS.ENABLED

  const handleToggleStatus = async () => {
    const newStatus = isEnabled
      ? VIRTUAL_MODEL_STATUS.DISABLED
      : VIRTUAL_MODEL_STATUS.ENABLED

    const result = await updateVirtualModel({
      ...virtualModel,
      status: newStatus,
    })
    if (result.success) {
      const message = isEnabled
        ? t(SUCCESS_MESSAGES.VIRTUAL_MODEL_DISABLED)
        : t(SUCCESS_MESSAGES.VIRTUAL_MODEL_ENABLED)
      toast.success(message)
      triggerRefresh()
    }
  }

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        render={
          <Button
            variant='ghost'
            className='data-popup-open:bg-muted flex h-8 w-8 p-0'
          />
        }
      >
        <DotsHorizontalIcon className='h-4 w-4' />
        <span className='sr-only'>{t('Open menu')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-[160px]'>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(virtualModel)
            setOpen('update')
          }}
        >
          {t('Edit')}
          <DropdownMenuShortcut>
            <Edit size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleToggleStatus}>
          {isEnabled ? (
            <>
              {t('Disable')}
              <DropdownMenuShortcut>
                <PowerOff size={16} />
              </DropdownMenuShortcut>
            </>
          ) : (
            <>
              {t('Enable')}
              <DropdownMenuShortcut>
                <Power size={16} />
              </DropdownMenuShortcut>
            </>
          )}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(virtualModel)
            setOpen('delete')
          }}
          className='text-destructive focus:text-destructive'
        >
          {t('Delete')}
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
