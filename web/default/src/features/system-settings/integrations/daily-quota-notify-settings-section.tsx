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
import { useMemo, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { testDailyQuotaNotify } from '../api'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const dailyQuotaNotifySchema = z
  .object({
    daily_quota_notify_setting: z.object({
      enabled: z.boolean(),
      threshold: z.coerce
        .number()
        .min(0, 'Threshold must be non-negative'),
      webhook_url: z.string(),
      secret: z.string(),
      // 纯前端状态：secret 不随 GET 回传，勾选后保存时提交空 secret 以清除已保存的密钥
      clear_secret: z.boolean(),
    }),
  })
  .superRefine((values, ctx) => {
    const { enabled, threshold, webhook_url, secret, clear_secret } =
      values.daily_quota_notify_setting
    if (!enabled) return

    if (threshold <= 0) {
      ctx.addIssue({
        code: 'custom',
        path: ['daily_quota_notify_setting', 'threshold'],
        message: 'Threshold must be greater than 0 when enabled',
      })
    }
    if (!webhook_url.trim()) {
      ctx.addIssue({
        code: 'custom',
        path: ['daily_quota_notify_setting', 'webhook_url'],
        message: 'Webhook URL is required when enabled',
      })
    }
    if (clear_secret && secret.trim() !== '') {
      ctx.addIssue({
        code: 'custom',
        path: ['daily_quota_notify_setting', 'secret'],
        message: 'Enter a new secret or clear the saved one, not both',
      })
    }
  })

type DailyQuotaNotifyFormValues = z.output<typeof dailyQuotaNotifySchema>
type DailyQuotaNotifyFormInput = z.input<typeof dailyQuotaNotifySchema>

type DailyQuotaNotifySettingsSectionProps = {
  defaultValues: {
    'daily_quota_notify_setting.enabled': boolean
    'daily_quota_notify_setting.threshold': number
    'daily_quota_notify_setting.webhook_url': string
    'daily_quota_notify_setting.secret': string
  }
}

type NormalizedDailyQuotaNotifyValues = {
  'daily_quota_notify_setting.enabled': boolean
  'daily_quota_notify_setting.threshold': number
  'daily_quota_notify_setting.webhook_url': string
  'daily_quota_notify_setting.secret': string
}

const buildFormDefaults = (
  defaults: DailyQuotaNotifySettingsSectionProps['defaultValues']
): DailyQuotaNotifyFormInput => ({
  daily_quota_notify_setting: {
    enabled: defaults['daily_quota_notify_setting.enabled'] ?? false,
    threshold: defaults['daily_quota_notify_setting.threshold'] ?? 0,
    webhook_url: defaults['daily_quota_notify_setting.webhook_url'] ?? '',
    secret: defaults['daily_quota_notify_setting.secret'] ?? '',
    clear_secret: false,
  },
})

const normalizeDefaults = (
  defaults: DailyQuotaNotifySettingsSectionProps['defaultValues']
): NormalizedDailyQuotaNotifyValues => ({
  'daily_quota_notify_setting.enabled':
    defaults['daily_quota_notify_setting.enabled'] ?? false,
  'daily_quota_notify_setting.threshold':
    defaults['daily_quota_notify_setting.threshold'] ?? 0,
  'daily_quota_notify_setting.webhook_url': (
    defaults['daily_quota_notify_setting.webhook_url'] ?? ''
  ).trim(),
  'daily_quota_notify_setting.secret': (
    defaults['daily_quota_notify_setting.secret'] ?? ''
  ).trim(),
})

const normalizeFormValues = (
  values: DailyQuotaNotifyFormValues
): NormalizedDailyQuotaNotifyValues => ({
  'daily_quota_notify_setting.enabled':
    values.daily_quota_notify_setting.enabled,
  'daily_quota_notify_setting.threshold':
    values.daily_quota_notify_setting.threshold,
  'daily_quota_notify_setting.webhook_url':
    values.daily_quota_notify_setting.webhook_url.trim(),
  'daily_quota_notify_setting.secret':
    values.daily_quota_notify_setting.secret.trim(),
})

export function DailyQuotaNotifySettingsSection({
  defaultValues,
}: DailyQuotaNotifySettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [testing, setTesting] = useState(false)
  const baselineRef = useRef<NormalizedDailyQuotaNotifyValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<DailyQuotaNotifyFormInput, unknown, DailyQuotaNotifyFormValues>(
    {
      resolver: zodResolver(dailyQuotaNotifySchema),
      defaultValues: formDefaults,
    }
  )

  useResetForm(form, formDefaults)

  const onSubmit = async (values: DailyQuotaNotifyFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedDailyQuotaNotifyValues>
    ).filter(
      (key) =>
        // secret 不会随 GET 回传（敏感键），留空表示沿用已保存的密钥，不能当作"清空"提交
        key !== 'daily_quota_notify_setting.secret' &&
        normalized[key] !== baselineRef.current[key]
    )
    if (values.daily_quota_notify_setting.clear_secret) {
      // 显式勾选清除：提交空 secret 覆盖已保存的密钥
      updates.push('daily_quota_notify_setting.secret')
    } else if (normalized['daily_quota_notify_setting.secret'] !== '') {
      updates.push('daily_quota_notify_setting.secret')
    }

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
    if (values.daily_quota_notify_setting.clear_secret) {
      form.setValue('daily_quota_notify_setting.clear_secret', false)
    }
  }

  const onSendTest = async () => {
    setTesting(true)
    try {
      const res = await testDailyQuotaNotify({
        webhook_url:
          (form.getValues('daily_quota_notify_setting.webhook_url') ?? '')
            .trim()
            .toString(),
        secret: (form.getValues('daily_quota_notify_setting.secret') ?? '')
          .trim()
          .toString(),
      })
      if (res.success) {
        toast.success(res.message)
      } else {
        toast.error(res.message)
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to send test message')
      )
    } finally {
      setTesting(false)
    }
  }

  return (
    <SettingsSection
      title={t('Daily Quota Alert')}
      description={t(
        'Notify a DingTalk robot when an account reaches multiples of the daily quota'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <FormField
            control={form.control}
            name='daily_quota_notify_setting.enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Enable daily quota alert')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Alerts only: reaching the threshold never blocks usage'
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
            name='daily_quota_notify_setting.threshold'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Daily quota threshold')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    step='any'
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
                </FormControl>
                <FormDescription>
                  {t(
                    'The unit follows the site currency display setting; an alert is sent each time an account consumes a multiple of it in one day'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='daily_quota_notify_setting.webhook_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('DingTalk webhook URL')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='https://oapi.dingtalk.com/robot/send?access_token=...'
                    value={field.value}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Webhook address of the DingTalk group robot that receives the alerts'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='daily_quota_notify_setting.secret'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('DingTalk signing secret')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder='SEC...'
                    value={field.value}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Required only when the robot security setting is "Sign"; leave blank to keep the saved secret'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='daily_quota_notify_setting.clear_secret'
            render={({ field }) => (
              <FormItem className='flex flex-row items-start gap-3 space-y-0'>
                <FormControl>
                  <Checkbox
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <div className='space-y-0.5'>
                  <FormLabel className='text-sm font-normal'>
                    {t('Clear saved signing secret')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Check to remove the saved secret when saving; use this after switching the robot security setting away from sign mode'
                    )}
                  </FormDescription>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='flex flex-wrap items-center gap-3'>
            <Button type='submit' disabled={updateOption.isPending}>
              {updateOption.isPending
                ? t('Saving...')
                : t('Save daily quota alert settings')}
            </Button>
            <Button
              type='button'
              variant='outline'
              onClick={onSendTest}
              disabled={testing}
            >
              {testing ? t('Sending...') : t('Send test message')}
            </Button>
          </div>
        </form>
      </Form>
    </SettingsSection>
  )
}
