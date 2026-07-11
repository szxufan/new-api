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
import { useEffect } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const groupImageModelsExample = JSON.stringify(
  {
    default: 'dall-e-3',
    vip: 'dall-e-3',
    svip: 'gpt-image-1',
  },
  null,
  2
)

const jsonString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  try {
    const parsed = JSON.parse(trimmed)
    if (typeof parsed !== 'object' || parsed === null) {
      return false
    }
    return true
  } catch {
    return false
  }
}, 'Invalid JSON format (must be a JSON object)')

const schema = z.object({
  mcp: z.object({
    group_image_models: jsonString,
  }),
})

type McpSettingsFormValues = z.output<typeof schema>
type McpSettingsFormInput = z.input<typeof schema>

type FlatMcpSettings = {
  'mcp_setting.group_image_models': string
}

const flattenMcpValues = (values: McpSettingsFormValues): FlatMcpSettings => ({
  'mcp_setting.group_image_models': normalizeJsonText(
    values.mcp.group_image_models,
    '{}'
  ),
})

function normalizeJsonText(value: string, fallback: string) {
  const trimmed = (value ?? '').toString().trim()
  return trimmed ? trimmed : fallback
}

type McpSettingsCardProps = {
  defaultValues: McpSettingsFormValues
}

export function McpSettingsCard({ defaultValues }: McpSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<McpSettingsFormInput, unknown, McpSettingsFormValues>({
    resolver: zodResolver(schema),
    defaultValues: defaultValues as McpSettingsFormInput,
  })

  useEffect(() => {
    form.reset(defaultValues as McpSettingsFormInput)
  }, [defaultValues, form])

  const formatJsonField = () => {
    const raw = form.getValues('mcp.group_image_models')
    if (!raw || !raw.trim()) return
    try {
      const formatted = JSON.stringify(JSON.parse(raw), null, 2)
      form.setValue('mcp.group_image_models', formatted, {
        shouldDirty: true,
      })
    } catch {
      toast.error(t('Invalid JSON format'))
    }
  }

  const fillExample = () => {
    form.setValue('mcp.group_image_models', groupImageModelsExample, {
      shouldDirty: true,
    })
  }

  const onSubmit = async (values: McpSettingsFormValues) => {
    const flattenedDefaults = flattenMcpValues(defaultValues)
    const flattenedValues = flattenMcpValues(values)
    const updates = Object.entries(flattenedValues).filter(
      ([key, value]) =>
        value !== flattenedDefaults[key as keyof FlatMcpSettings]
    )

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({
        key,
        value,
      })
    }
  }

  return (
    <SettingsSection
      title={t('MCP Image Generation')}
      description={t(
        'Configure per-group image generation models for MCP endpoints'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <FormField
            control={form.control}
            name='mcp.group_image_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group image models')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={8}
                    placeholder={`${t('Example:')}\n${groupImageModelsExample}`}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'JSON map of group → image model name. When an MCP client calls generate_image, the model for the caller\'s group is used.'
                  )}
                </FormDescription>
                <div className='flex flex-wrap gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={fillExample}
                  >
                    {t('Fill example')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={formatJsonField}
                  >
                    {t('Format JSON')}
                  </Button>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
