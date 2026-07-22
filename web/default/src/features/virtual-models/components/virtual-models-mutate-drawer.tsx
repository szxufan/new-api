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
import { useEffect, useState } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
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
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import {
  createVirtualModel,
  updateVirtualModel,
  getChannelOptions,
  getVirtualModel,
  type ChannelOption,
} from '../api'
import { SUCCESS_MESSAGES, VIRTUAL_MODEL_MODE_CONFIG } from '../constants'
import {
  getVirtualModelFormSchema,
  type VirtualModelFormValues,
  VIRTUAL_MODEL_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformVirtualModelToFormDefaults,
} from '../lib'
import { VIRTUAL_MODEL_MODES, type VirtualModel } from '../types'
import { useVirtualModels } from './virtual-models-provider'

type VirtualModelsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: VirtualModel
}

function ChannelSelect({
  value,
  onChange,
  channels,
  placeholder,
}: {
  value: number
  onChange: (value: number) => void
  channels: ChannelOption[]
  placeholder: string
}) {
  const { t } = useTranslation()
  return (
    <Select
      items={[
        { value: '0', label: t('Auto Select') },
        ...channels.map((channel) => ({
          value: String(channel.id),
          label: channel.name,
        })),
      ]}
      value={String(value)}
      onValueChange={(v) => v !== null && onChange(parseInt(v))}
    >
      <SelectTrigger className='w-full'>
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        <SelectGroup>
          <SelectItem value='0'>{t('Auto Select')}</SelectItem>
          {channels.map((channel) => (
            <SelectItem key={channel.id} value={String(channel.id)}>
              {channel.name}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

export function VirtualModelsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: VirtualModelsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useVirtualModels()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<VirtualModelFormValues>({
    resolver: zodResolver(getVirtualModelFormSchema(t)),
    defaultValues: VIRTUAL_MODEL_FORM_DEFAULT_VALUES,
  })

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'targets',
  })

  const mode = form.watch('mode')

  // Load channel options for the "specify channel" dropdowns
  const { data: channels = [] } = useQuery({
    queryKey: ['virtual-models-channel-options'],
    queryFn: getChannelOptions,
    staleTime: 60_000,
  })

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      getVirtualModel(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformVirtualModelToFormDefaults(result.data))
        }
      })
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(VIRTUAL_MODEL_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form])

  const onSubmit = async (data: VirtualModelFormValues) => {
    setIsSubmitting(true)
    try {
      if (isUpdate && currentRow) {
        const result = await updateVirtualModel({
          ...transformFormDataToPayload(data, currentRow.status),
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.VIRTUAL_MODEL_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        }
      } else {
        const result = await createVirtualModel(
          transformFormDataToPayload(data)
        )
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.VIRTUAL_MODEL_CREATED))
          onOpenChange(false)
          triggerRefresh()
        }
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
        <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
          <SheetTitle>
            {isUpdate ? t('Update Virtual Model') : t('Create Virtual Model')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the virtual model by providing necessary info.')
              : t('Add a new virtual model by providing necessary info.')}{' '}
            {t('Click save when you&apos;re done.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='virtual-model-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder={t('Enter a unique virtual model name')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('The name clients use to call this virtual model.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='mode'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Mode')}</FormLabel>
                  <FormControl>
                    <RadioGroup
                      value={field.value}
                      onValueChange={(value) =>
                        field.onChange(
                          value as (typeof VIRTUAL_MODEL_MODES)[number]
                        )
                      }
                      className='grid gap-3 sm:grid-cols-2'
                    >
                      {VIRTUAL_MODEL_MODES.map((modeValue) => {
                        const modeConfig = VIRTUAL_MODEL_MODE_CONFIG[modeValue]
                        return (
                          <Label
                            key={modeValue}
                            htmlFor={`virtual-model-mode-${modeValue}`}
                            className={cn(
                              'hover:border-primary/40 focus-within:border-primary/50 has-data-[checked]:border-primary has-data-[checked]:ring-primary/20 group bg-card border-muted flex cursor-pointer flex-col gap-2 rounded-xl border p-4 font-normal transition-all has-data-[checked]:ring-2'
                            )}
                          >
                            <div className='flex items-center gap-3'>
                              <RadioGroupItem
                                id={`virtual-model-mode-${modeValue}`}
                                value={modeValue}
                              />
                              <Label
                                htmlFor={`virtual-model-mode-${modeValue}`}
                                className='text-sm leading-none font-semibold'
                              >
                                {t(modeConfig.labelKey)}
                              </Label>
                            </div>
                            <p className='text-muted-foreground text-xs'>
                              {t(modeConfig.descriptionKey)}
                            </p>
                          </Label>
                        )
                      })}
                    </RadioGroup>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='targets'
              render={() => (
                <FormItem>
                  <FormLabel>{t('Sub-models')}</FormLabel>
                  <div className='space-y-3'>
                    {fields.map((fieldItem, index) => (
                      <div
                        key={fieldItem.id}
                        className='bg-card space-y-2 rounded-lg border p-3'
                      >
                        <div className='flex items-center justify-between gap-2'>
                          <span className='text-muted-foreground text-xs font-medium'>
                            {t('Sub-model {{index}}', { index: index + 1 })}
                          </span>
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            onClick={() => remove(index)}
                            disabled={fields.length <= 1}
                            aria-label={t('Remove sub-model')}
                          >
                            <Trash2 className='text-destructive h-4 w-4' />
                          </Button>
                        </div>
                        <FormField
                          control={form.control}
                          name={`targets.${index}.model`}
                          render={({ field }) => (
                            <FormItem>
                              <FormControl>
                                <Input
                                  {...field}
                                  placeholder={t(
                                    'Real model name, e.g. gpt-4o'
                                  )}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
                          <FormField
                            control={form.control}
                            name={`targets.${index}.channel_id`}
                            render={({ field }) => (
                              <FormItem>
                                <FormControl>
                                  <ChannelSelect
                                    value={field.value}
                                    onChange={field.onChange}
                                    channels={channels}
                                    placeholder={t('Auto Select')}
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`targets.${index}.group`}
                            render={({ field }) => (
                              <FormItem>
                                <FormControl>
                                  <Input
                                    {...field}
                                    placeholder={t(
                                      'Group (empty = follow requester)'
                                    )}
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                        </div>
                      </div>
                    ))}
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        append({ model: '', channel_id: 0, group: '' })
                      }
                    >
                      <Plus className='h-4 w-4' />
                      {t('Add Sub-model')}
                    </Button>
                  </div>
                  <FormDescription>
                    {t(
                      'Leave channel empty to auto select by group; leave group empty to follow the requester group.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {mode === 'quality' && (
              <div className='space-y-4 rounded-lg border p-4'>
                <div className='text-sm font-medium'>
                  {t('Aggregator Settings')}
                </div>
                <FormField
                  control={form.control}
                  name='aggregator_model'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Aggregator Model')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Real model name, e.g. gpt-4o')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'The model that generates the final answer from all sub-model responses.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='aggregator_channel_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Channel')}</FormLabel>
                        <FormControl>
                          <ChannelSelect
                            value={field.value}
                            onChange={field.onChange}
                            channels={channels}
                            placeholder={t('Auto Select')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='aggregator_group'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Group')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder={t('Group (empty = follow requester)')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                <FormField
                  control={form.control}
                  name='aggregator_prompt_template'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Prompt Template')}</FormLabel>
                      <FormControl>
                        <Textarea
                          {...field}
                          rows={4}
                          placeholder={t(
                            'Optional. Use {{answers}} as the placeholder for all sub-model responses; leave empty to use the built-in default.'
                          )}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}
          </form>
        </Form>
        <SheetFooter className='grid grid-cols-2 gap-2 border-t px-4 py-3 sm:flex sm:px-6 sm:py-4'>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='virtual-model-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
