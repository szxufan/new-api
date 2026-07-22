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
import { type ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { DataTableColumnHeader } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { VIRTUAL_MODEL_MODE_CONFIG, VIRTUAL_MODEL_STATUSES } from '../constants'
import { parseTargets } from '../lib'
import { type VirtualModel, type VirtualModelMode } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

export function useVirtualModelsColumns(): ColumnDef<VirtualModel>[] {
  const { t } = useTranslation()
  return [
    {
      accessorKey: 'id',
      meta: { label: t('ID'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('ID')} />
      ),
      cell: ({ row }) => {
        return <div className='w-[60px]'>{row.getValue('id')}</div>
      },
    },
    {
      accessorKey: 'name',
      meta: { label: t('Name'), mobileTitle: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Name')} />
      ),
      cell: ({ row }) => {
        return (
          <div className='max-w-[200px] truncate font-medium'>
            {row.getValue('name')}
          </div>
        )
      },
    },
    {
      accessorKey: 'mode',
      meta: { label: t('Mode') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Mode')} />
      ),
      cell: ({ row }) => {
        const mode = row.getValue('mode') as VirtualModelMode
        const modeConfig = VIRTUAL_MODEL_MODE_CONFIG[mode]
        if (!modeConfig) {
          return null
        }
        return (
          <StatusBadge
            label={t(modeConfig.labelKey)}
            variant={mode === 'speed' ? 'info' : 'warning'}
            showDot={false}
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => {
        return value.includes(String(row.getValue(id)))
      },
    },
    {
      id: 'targets_count',
      accessorFn: (row) => parseTargets(row.targets).length,
      meta: { label: t('Sub-models'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Sub-models')} />
      ),
      cell: ({ row }) => {
        const count = parseTargets(row.original.targets).length
        return (
          <StatusBadge
            label={t('{{count}} model(s)', { count })}
            variant='neutral'
            copyable={false}
          />
        )
      },
      enableSorting: false,
    },
    {
      accessorKey: 'status',
      meta: { label: t('Status'), mobileBadge: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Status')} />
      ),
      cell: ({ row }) => {
        const statusValue = row.getValue('status') as number
        const statusConfig = VIRTUAL_MODEL_STATUSES[statusValue]

        if (!statusConfig) {
          return null
        }

        return (
          <StatusBadge
            label={t(statusConfig.labelKey)}
            variant={statusConfig.variant}
            showDot={statusConfig.showDot}
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value) => {
        return value.includes(String(row.getValue(id)))
      },
    },
    {
      accessorKey: 'updated_time',
      meta: { label: t('Updated'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Updated')} />
      ),
      cell: ({ row }) => {
        return (
          <div className='min-w-[140px] font-mono text-sm'>
            {formatTimestampToDate(row.getValue('updated_time'))}
          </div>
        )
      },
    },
    {
      id: 'actions',
      cell: ({ row }) => <DataTableRowActions row={row} />,
    },
  ]
}
