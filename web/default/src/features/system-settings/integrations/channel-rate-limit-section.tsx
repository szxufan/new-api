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
import { useMemo, useRef } from 'react'
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
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const channelRateLimitSchema = z.object({
  RateLimit429Enabled: z.boolean(),
  RateLimit429DurationMinutes: z.coerce
    .number()
    .int()
    .min(1, 'Duration must be at least 1 minute')
    .max(60, 'Duration must be at most 60 minutes'),
})

type ChannelRateLimitFormValues = z.output<typeof channelRateLimitSchema>
type ChannelRateLimitFormInput = z.input<typeof channelRateLimitSchema>

type ChannelRateLimitSectionProps = {
  defaultValues: {
    RateLimit429Enabled: boolean
    RateLimit429DurationMinutes: number
  }
}

const buildFormDefaults = (
  defaults: ChannelRateLimitSectionProps['defaultValues']
): ChannelRateLimitFormInput => ({
  RateLimit429Enabled: defaults.RateLimit429Enabled ?? true,
  RateLimit429DurationMinutes: defaults.RateLimit429DurationMinutes ?? 1,
})

type NormalizedValues = {
  RateLimit429Enabled: boolean
  RateLimit429DurationMinutes: number
}

const normalizeDefaults = (
  defaults: ChannelRateLimitSectionProps['defaultValues']
): NormalizedValues => ({
  RateLimit429Enabled: defaults.RateLimit429Enabled ?? true,
  RateLimit429DurationMinutes: defaults.RateLimit429DurationMinutes ?? 1,
})

const normalizeFormValues = (
  values: ChannelRateLimitFormValues
): NormalizedValues => ({
  RateLimit429Enabled: values.RateLimit429Enabled,
  RateLimit429DurationMinutes: values.RateLimit429DurationMinutes,
})

export function ChannelRateLimitSection({
  defaultValues,
}: ChannelRateLimitSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedValues>(normalizeDefaults(defaultValues))

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<
    ChannelRateLimitFormInput,
    unknown,
    ChannelRateLimitFormValues
  >({
    resolver: zodResolver(channelRateLimitSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const onSubmit = async (values: ChannelRateLimitFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }

    baselineRef.current = normalized
  }

  return (
    <SettingsSection
      title={t('Channel Rate Limiting')}
      description={t(
        'Configure automatic rate limiting when receiving HTTP 429 responses from upstream providers'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='RateLimit429Enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Auto 429 Rate Limit')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Automatically rate limit channels when receiving HTTP 429 from upstream'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='RateLimit429DurationMinutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Auto 429 Rate Limit Duration (minutes)')}
                  </FormLabel>
                  <FormControl>
                    <div className='flex items-center gap-2'>
                      <Input
                        type='number'
                        min={1}
                        max={60}
                        step={1}
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                      <span className='text-muted-foreground text-sm'>
                        {t('minutes')}
                      </span>
                    </div>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'How long to rate limit a channel after receiving 429 (1-60 minutes)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending
              ? t('Saving...')
              : t('Save rate limit settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
