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
import { useState, useEffect, useRef } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  ComboboxInput,
  type ComboboxInputOption,
} from '@/components/ui/combobox-input'
import {
  parseGroupModelMap,
  convertGroupModelRowsToJson,
  type GroupModelRow,
} from './mcp-setting-validation'

type GroupModelMapEditorProps = {
  /** JSON string of group → model map (controlled) */
  value: string
  onChange: (value: string) => void
  groupOptions: ComboboxInputOption[]
  modelOptions: ComboboxInputOption[]
  disabled?: boolean
  groupPlaceholder?: string
  modelPlaceholder?: string
}

/**
 * Structured row editor for a group → model JSON map.
 * Both columns are comboboxes with searchable candidates and
 * free-text input (allowCustomValue) for custom values.
 */
export function GroupModelMapEditor(props: GroupModelMapEditorProps) {
  const { t } = useTranslation()
  const { value, onChange, disabled = false } = props
  const [rows, setRows] = useState<GroupModelRow[]>([])
  const internalChangeRef = useRef(false)

  useEffect(() => {
    if (internalChangeRef.current) {
      internalChangeRef.current = false
      return
    }
    const parsed = parseGroupModelMap(value)
    if (parsed !== null) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- syncing internal rows state from external value prop change
      setRows(parsed)
    }
  }, [value])

  const emitRows = (updatedRows: GroupModelRow[]) => {
    setRows(updatedRows)
    const json = convertGroupModelRowsToJson(updatedRows)
    internalChangeRef.current = true
    onChange(json)
  }

  const handleAddRow = () => {
    const newRow: GroupModelRow = {
      id: `row-new-${rows.length}-${crypto.randomUUID()}`,
      group: '',
      model: '',
    }
    emitRows([...rows, newRow])
  }

  const handleDeleteRow = (id: string) => {
    emitRows(rows.filter((row) => row.id !== id))
  }

  const handleRowChange = (
    id: string,
    field: 'group' | 'model',
    newValue: string
  ) => {
    emitRows(
      rows.map((row) => (row.id === id ? { ...row, [field]: newValue } : row))
    )
  }

  return (
    <div className='space-y-2'>
      {rows.length > 0 ? (
        <div className='space-y-2'>
          <div className='grid grid-cols-[1fr_1fr_auto] gap-2 text-sm font-medium'>
            <div>{t('Group')}</div>
            <div>{t('Model')}</div>
            <div className='w-10' />
          </div>
          {rows.map((row) => (
            <div key={row.id} className='grid grid-cols-[1fr_1fr_auto] gap-2'>
              <ComboboxInput
                options={props.groupOptions}
                value={row.group}
                onValueChange={(newValue) =>
                  handleRowChange(row.id, 'group', newValue)
                }
                placeholder={props.groupPlaceholder ?? t('Select group')}
                allowCustomValue
              />
              <ComboboxInput
                options={props.modelOptions}
                value={row.model}
                onValueChange={(newValue) =>
                  handleRowChange(row.id, 'model', newValue)
                }
                placeholder={props.modelPlaceholder ?? t('Select model')}
                allowCustomValue
              />
              <Button
                type='button'
                variant='ghost'
                size='icon'
                onClick={() => handleDeleteRow(row.id)}
                disabled={disabled}
                className='h-10 w-10'
                aria-label={t('Delete')}
              >
                <Trash2 className='h-4 w-4' />
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground flex h-20 items-center justify-center rounded-md border border-dashed text-sm'>
          {t('No mapping configured. Click "Add" to get started.')}
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
        {t('Add')}
      </Button>
    </div>
  )
}
