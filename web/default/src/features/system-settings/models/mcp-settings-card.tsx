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
import { useEffect, useMemo } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
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
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { getEnabledModels } from '@/features/channels/api'
import { getGroups } from '@/features/users/api'
import { buildModelNameOptions } from './model-name-options'
import { GroupModelMapEditor } from './group-model-map-editor'
import {
  parseGroupModelMap,
  convertGroupModelRowsToJson,
} from './mcp-setting-validation'

const MCP_MODEL_MAP_KEYS = [
  'mcp_setting.group_image_models',
  'mcp_setting.group_i2i_models',
  'mcp_setting.group_video_t2v_models',
  'mcp_setting.group_video_i2v_models',
  'mcp_setting.group_video_kf2v_models',
  'mcp_setting.group_video_r2v_models',
] as const

type McpModelMapKey = (typeof MCP_MODEL_MAP_KEYS)[number]

type McpModelMapValues = Record<McpModelMapKey, string>

const schema = z.object(
  Object.fromEntries(
    MCP_MODEL_MAP_KEYS.map((key) => [key, z.string()])
  ) as Record<McpModelMapKey, z.ZodString>
)

type McpSettingsFormValues = z.output<typeof schema>

/** Normalize raw stored JSON into a canonical pretty-printed form */
function normalizeMapJson(raw: string | undefined): string {
  const rows = parseGroupModelMap(raw ?? '')
  if (rows === null) {
    return '{}'
  }
  return convertGroupModelRowsToJson(rows)
}

function flattenMcpValues(values: McpSettingsFormValues): McpModelMapValues {
  const flattened = {} as McpModelMapValues
  for (const key of MCP_MODEL_MAP_KEYS) {
    flattened[key] = normalizeMapJson(values[key])
  }
  return flattened
}

type McpSettingsCardProps = {
  defaultValues: McpSettingsFormValues
}

export function McpSettingsCard(props: McpSettingsCardProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { data: groupsData } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })
  const { data: enabledModelsData } = useQuery({
    queryKey: ['channel_models_enabled'],
    queryFn: getEnabledModels,
    staleTime: 5 * 60 * 1000,
  })

  const groupOptions = useMemo(() => {
    if (!groupsData?.data) return []
    return groupsData.data.map((group) => ({ value: group, label: group }))
  }, [groupsData])

  const modelOptions = useMemo(() => {
    const channelModels = enabledModelsData?.success
      ? enabledModelsData.data
      : undefined
    return buildModelNameOptions(channelModels, new Set())
  }, [enabledModelsData])

  const form = useForm<McpSettingsFormValues>({
    resolver: zodResolver(schema),
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reset when remote settings arrive
  }, [props.defaultValues])

  const onSubmit = async (values: McpSettingsFormValues) => {
    const flattenedDefaults = flattenMcpValues(props.defaultValues)
    const flattenedValues = flattenMcpValues(values)
    const updates = MCP_MODEL_MAP_KEYS.filter(
      (key) => flattenedValues[key] !== flattenedDefaults[key]
    )

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: flattenedValues[key],
      })
    }
  }

  const modelMapSections: {
    key: McpModelMapKey
    label: string
    description: string
    tool: string
  }[] = [
    {
      key: 'mcp_setting.group_image_models',
      label: t('Text-to-image models'),
      description: t('Used by generate_image without image_ids'),
      tool: 'generate_image',
    },
    {
      key: 'mcp_setting.group_i2i_models',
      label: t('Image-to-image models'),
      description: t('Used by generate_image with image_ids (max 3)'),
      tool: 'generate_image + image_ids',
    },
    {
      key: 'mcp_setting.group_video_t2v_models',
      label: t('Text-to-video models'),
      description: t('Used by generate_video'),
      tool: 'generate_video',
    },
    {
      key: 'mcp_setting.group_video_i2v_models',
      label: t('Image-to-video models (first frame)'),
      description: t('Used by generate_video_from_frames with first frame only'),
      tool: 'generate_video_from_frames',
    },
    {
      key: 'mcp_setting.group_video_kf2v_models',
      label: t('First-last frame video models'),
      description: t('Used by generate_video_from_frames with first + last frames'),
      tool: 'generate_video_from_frames',
    },
    {
      key: 'mcp_setting.group_video_r2v_models',
      label: t('Reference video models'),
      description: t('Used by generate_video_from_reference (max 3 images)'),
      tool: 'generate_video_from_reference',
    },
  ]

  return (
    <SettingsSection
      title={t('MCP Image Generation')}
      description={t(
        'Configure per-group models for MCP tools (image & video generation). When an MCP client calls a tool, the model of the caller\'s group is used, falling back to the default group.'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
          {modelMapSections.map((section) => (
            <FormField
              key={section.key}
              control={form.control}
              name={section.key}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{section.label}</FormLabel>
                  <FormControl>
                    <GroupModelMapEditor
                      value={field.value ?? '{}'}
                      onChange={field.onChange}
                      groupOptions={groupOptions}
                      modelOptions={modelOptions}
                    />
                  </FormControl>
                  <FormDescription>
                    {section.description} ({section.tool})
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          ))}

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
