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
import { Clock, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { ChannelTimeWindow } from '../types'

type TimeWindowsEditorProps = {
  value: ChannelTimeWindow[]
  onChange: (value: ChannelTimeWindow[]) => void
  disabled?: boolean
}

// 纯受控组件：表单值数组是唯一数据源，不维护内部行状态
export function TimeWindowsEditor({
  value,
  onChange,
  disabled = false,
}: TimeWindowsEditorProps) {
  const { t } = useTranslation()

  const handleAddRow = () => {
    onChange([...value, { start: '00:00', end: '00:00' }])
  }

  const handleDeleteRow = (index: number) => {
    onChange(value.filter((_, i) => i !== index))
  }

  const handleRowChange = (
    index: number,
    field: 'start' | 'end',
    newValue: string
  ) => {
    onChange(
      value.map((row, i) => (i === index ? { ...row, [field]: newValue } : row))
    )
  }

  return (
    <div className='space-y-2'>
      {value.length > 0 ? (
        <div className='space-y-2'>
          <div className='grid grid-cols-[1fr_1fr_auto] gap-2 text-sm font-medium'>
            <div>{t('Start Time')}</div>
            <div>{t('End Time')}</div>
            <div className='w-10'></div>
          </div>
          {value.map((row, index) => (
            <div
              key={`${row.start}-${row.end}-${index}`}
              className='grid grid-cols-[1fr_1fr_auto] gap-2'
            >
              <Input
                type='time'
                value={row.start}
                onChange={(e) =>
                  handleRowChange(index, 'start', e.target.value)
                }
                disabled={disabled}
              />
              <Input
                type='time'
                value={row.end}
                onChange={(e) => handleRowChange(index, 'end', e.target.value)}
                disabled={disabled}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon'
                onClick={() => handleDeleteRow(index)}
                disabled={disabled}
                className='h-10 w-10'
              >
                <Trash2 className='h-4 w-4' />
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground flex h-20 items-center justify-center gap-2 rounded-md border border-dashed text-sm'>
          <Clock className='h-4 w-4' />
          {t(
            'No time windows configured. The channel is not affected by scheduled switching.'
          )}
        </div>
      )}
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={handleAddRow}
        disabled={disabled}
        className='w-full'
      >
        <Plus className='mr-2 h-4 w-4' />
        {t('Add Time Window')}
      </Button>
    </div>
  )
}
