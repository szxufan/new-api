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
import { useEffect, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { MultiSelect } from '@/components/multi-select'
import { buildRelayPathOptions, RULE_TEMPLATES } from './constants'
import type { AffinityRule, KeySource } from './types'

const KEY_SOURCE_TYPES = [
  'context_int',
  'context_string',
  'request_header',
  'gjson',
] as const

const CONTEXT_KEY_PRESETS = [
  'id',
  'token_id',
  'token_key',
  'token_group',
  'group',
  'username',
  'user_group',
  'user_email',
  'specific_channel_id',
]

interface RuleFormValues {
  name: string
  enabled: boolean
  model_regex_text: string
  model_regex_exclude_text: string
  user_agent_include_text: string
  value_regex: string
  ttl_seconds: number
  skip_retry_on_failure: boolean
  include_using_group: boolean
  include_model_name: boolean
  include_rule_name: boolean
  param_override_template_json: string
}

function FieldHint({ children }: { children: ReactNode }) {
  return <p className='text-muted-foreground text-xs'>{children}</p>
}

function normalizeStringList(text: string): string[] {
  return text
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
}

function normalizeKeySource(src: Partial<KeySource>): KeySource {
  const type = (src?.type || 'gjson') as KeySource['type']
  if (type === 'gjson') return { type, key: '', path: src?.path || '' }
  return { type, key: src?.key || '', path: '' }
}

function addCustomPattern(list: string[], raw: string): string[] {
  const value = raw.trim()
  if (!value || list.includes(value)) return list
  return [...list, value]
}

interface PathPatternSelectProps {
  label: string
  hint: string
  options: { label: string; value: string }[]
  patterns: string[]
  onChange: (patterns: string[]) => void
  addPlaceholder: string
}

function PathPatternSelect(props: PathPatternSelectProps) {
  const { t } = useTranslation()
  const [custom, setCustom] = useState('')

  const appendCustom = () => {
    if (!custom.trim()) return
    props.onChange(addCustomPattern(props.patterns, custom))
    setCustom('')
  }

  return (
    <div className='grid gap-1.5'>
      <Label>{props.label}</Label>
      <MultiSelect
        options={props.options}
        selected={props.patterns}
        onChange={props.onChange}
      />
      <div className='flex items-center gap-2'>
        <Input
          placeholder={props.addPlaceholder}
          value={custom}
          onChange={(e) => setCustom(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              appendCustom()
            }
          }}
        />
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={appendCustom}
        >
          <Plus className='mr-1 h-3 w-3' />
          {t('Add')}
        </Button>
      </div>
      <FieldHint>{props.hint}</FieldHint>
    </div>
  )
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  rule: AffinityRule | null
  onSave: (rule: AffinityRule) => void
  templateKey?: string | null
}

export function RuleEditorDialog(props: Props) {
  const { t } = useTranslation()
  const isEdit = !!props.rule?.name
  const [keySources, setKeySources] = useState<KeySource[]>([
    { type: 'gjson', path: '' },
  ])
  const [pathPatterns, setPathPatterns] = useState<string[]>([])
  const [pathExcludePatterns, setPathExcludePatterns] = useState<string[]>([])
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const relayPathOptions = buildRelayPathOptions(t)

  const form = useForm<RuleFormValues>({
    defaultValues: {
      name: '',
      enabled: true,
      model_regex_text: '',
      model_regex_exclude_text: '',
      user_agent_include_text: '',
      value_regex: '',
      ttl_seconds: 0,
      skip_retry_on_failure: false,
      include_using_group: true,
      include_model_name: false,
      include_rule_name: true,
      param_override_template_json: '',
    },
  })

  const resetFromRule = (r: Partial<AffinityRule>) => {
    form.reset({
      name: r.name || '',
      enabled: r.enabled !== false,
      model_regex_text: (r.model_regex || []).join('\n'),
      model_regex_exclude_text: (r.model_regex_exclude || []).join('\n'),
      user_agent_include_text: (r.user_agent_include || []).join('\n'),
      value_regex: r.value_regex || '',
      ttl_seconds: r.ttl_seconds || 0,
      skip_retry_on_failure: !!r.skip_retry_on_failure,
      include_using_group: r.include_using_group ?? true,
      include_model_name: !!r.include_model_name,
      include_rule_name: r.include_rule_name ?? true,
      param_override_template_json: r.param_override_template
        ? JSON.stringify(r.param_override_template, null, 2)
        : '',
    })
    setPathPatterns(r.path_regex || [])
    setPathExcludePatterns(r.path_regex_exclude || [])
    const sources = (r.key_sources || []).map(normalizeKeySource)
    setKeySources(sources.length > 0 ? sources : [{ type: 'gjson', path: '' }])
    if (r.param_override_template) setAdvancedOpen(true)
  }

  const resetToBlank = () => {
    form.reset({
      name: '',
      enabled: true,
      model_regex_text: '',
      model_regex_exclude_text: '',
      user_agent_include_text: '',
      value_regex: '',
      ttl_seconds: 0,
      skip_retry_on_failure: false,
      include_using_group: true,
      include_model_name: false,
      include_rule_name: true,
      param_override_template_json: '',
    })
    setPathPatterns([])
    setPathExcludePatterns([])
    setKeySources([{ type: 'gjson', path: '' }])
  }

  useEffect(() => {
    if (!props.open) return

    if (props.rule) {
      resetFromRule(props.rule)
    } else if (props.templateKey && RULE_TEMPLATES[props.templateKey]) {
      resetFromRule(RULE_TEMPLATES[props.templateKey])
    } else {
      resetToBlank()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.open, props.rule, props.templateKey])

  const handleSave = (values: RuleFormValues) => {
    const modelRegex = normalizeStringList(values.model_regex_text)
    if (modelRegex.length === 0) {
      toast.error(t('At least one model regex pattern is required'))
      return
    }

    const validKeySources = keySources
      .map(normalizeKeySource)
      .filter((s) => s.type && (s.type === 'gjson' ? s.path : s.key))
    if (validKeySources.length === 0) {
      toast.error(t('At least one valid key source is required'))
      return
    }

    let paramTemplate: Record<string, unknown> | null = null
    if (values.param_override_template_json.trim()) {
      try {
        const parsed = JSON.parse(values.param_override_template_json)
        if (
          typeof parsed !== 'object' ||
          Array.isArray(parsed) ||
          parsed === null
        ) {
          toast.error(t('Parameter override template must be a JSON object'))
          return
        }
        paramTemplate = parsed
      } catch {
        toast.error(t('Invalid JSON in parameter override template'))
        return
      }
    }

    const rule: AffinityRule = {
      id: props.rule?.id,
      name: values.name.trim(),
      enabled: values.enabled,
      model_regex: modelRegex,
      model_regex_exclude: normalizeStringList(values.model_regex_exclude_text),
      path_regex: pathPatterns,
      path_regex_exclude: pathExcludePatterns,
      user_agent_include: normalizeStringList(values.user_agent_include_text),
      key_sources: validKeySources,
      value_regex: values.value_regex.trim(),
      ttl_seconds: Number(values.ttl_seconds || 0),
      skip_retry_on_failure: values.skip_retry_on_failure,
      include_using_group: values.include_using_group,
      include_model_name: values.include_model_name,
      include_rule_name: values.include_rule_name,
      param_override_template: paramTemplate,
    }

    props.onSave(rule)
    props.onOpenChange(false)
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[85vh] max-w-2xl overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{isEdit ? t('Edit Rule') : t('Add Rule')}</DialogTitle>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(handleSave)} className='space-y-4'>
          <div className='grid gap-1.5'>
            <Label>{t('Name')} *</Label>
            <Input
              placeholder='prefer-by-conversation-id'
              {...form.register('name', { required: true })}
            />
          </div>

          <div className='flex items-center gap-2'>
            <Switch
              checked={form.watch('enabled')}
              onCheckedChange={(v) => form.setValue('enabled', v)}
            />
            <Label>{t('Enable Rule')}</Label>
            <FieldHint>
              {t('Disabled rules are kept but never match requests.')}
            </FieldHint>
          </div>

          <Separator />

          <div className='grid grid-cols-2 gap-3'>
            <div className='grid gap-1.5'>
              <Label>{t('Model Regex (one per line)')} *</Label>
              <Textarea
                rows={4}
                placeholder={'^gpt-4o.*$\n^claude-3.*$'}
                {...form.register('model_regex_text', { required: true })}
              />
              <FieldHint>
                {t(
                  'The rule applies only when the request model matches any pattern. Rules are evaluated in order; the first match wins.'
                )}
              </FieldHint>
            </div>
            <div className='grid gap-1.5'>
              <Label>{t('Excluded Model Regex (one per line)')}</Label>
              <Textarea
                rows={4}
                placeholder={'^gpt-image-.*$'}
                {...form.register('model_regex_exclude_text')}
              />
              <FieldHint>
                {t(
                  'Requests whose model matches any pattern here skip this rule, even if they match the patterns above.'
                )}
              </FieldHint>
            </div>
          </div>

          <PathPatternSelect
            label={t('Path Whitelist (select endpoints)')}
            hint={t(
              'Restrict the rule to the selected endpoints; leave empty to match all paths. Prefix matching, e.g. /v1/videos also covers task lookup and remix.'
            )}
            options={relayPathOptions}
            patterns={pathPatterns}
            onChange={setPathPatterns}
            addPlaceholder={t('Custom path regex, press Enter to add')}
          />

          <PathPatternSelect
            label={t('Excluded Paths (select endpoints)')}
            hint={t(
              'Requests to these endpoints skip this rule entirely. Use this to opt image/video endpoints (no upstream cache-hit benefit) out of affinity.'
            )}
            options={relayPathOptions}
            patterns={pathExcludePatterns}
            onChange={setPathExcludePatterns}
            addPlaceholder={t('Custom path regex, press Enter to add')}
          />

          <div className='flex items-center gap-2'>
            <Switch
              checked={form.watch('skip_retry_on_failure')}
              onCheckedChange={(v) => form.setValue('skip_retry_on_failure', v)}
            />
            <Label>{t('Skip retry on failure')}</Label>
            <FieldHint>
              {t(
                'When the affinity channel is unavailable, fail immediately instead of retrying on other channels.'
              )}
            </FieldHint>
          </div>

          <Separator />

          {/* Key Sources */}
          <div>
            <div className='mb-2 flex items-center justify-between'>
              <Label>{t('Key Sources')}</Label>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() =>
                  setKeySources((prev) => [
                    ...prev,
                    { type: 'gjson', path: '' },
                  ])
                }
              >
                <Plus className='mr-1 h-3 w-3' />
                {t('Add')}
              </Button>
            </div>
            <p className='text-muted-foreground mb-1 text-xs'>
              {t('Common Keys')}: {CONTEXT_KEY_PRESETS.join(', ')}
            </p>
            <FieldHint>
              {t(
                'Sources are tried in order; the first non-empty value becomes the affinity key. If none yields a value, the rule is skipped.'
              )}
            </FieldHint>
            <div className='mt-2 space-y-2'>
              {keySources.map((src, idx) => (
                <div key={idx} className='flex items-center gap-2'>
                  <Select
                    items={[
                      ...KEY_SOURCE_TYPES.map((t) => ({ value: t, label: t })),
                    ]}
                    value={src.type}
                    onValueChange={(v) => {
                      if (v === null) return
                      const next = [...keySources]
                      next[idx] = normalizeKeySource({
                        ...src,
                        type: v as KeySource['type'],
                      })
                      setKeySources(next)
                    }}
                  >
                    <SelectTrigger className='w-[160px]'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {KEY_SOURCE_TYPES.map((t) => (
                          <SelectItem key={t} value={t}>
                            {t}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <Input
                    className='flex-1'
                    placeholder={
                      src.type === 'gjson'
                        ? 'metadata.conversation_id'
                        : 'user_id'
                    }
                    value={
                      src.type === 'gjson' ? src.path || '' : src.key || ''
                    }
                    onChange={(e) => {
                      const next = [...keySources]
                      if (src.type === 'gjson') {
                        next[idx] = { ...src, path: e.target.value }
                      } else {
                        next[idx] = { ...src, key: e.target.value }
                      }
                      setKeySources(next)
                    }}
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    onClick={() =>
                      setKeySources((prev) => prev.filter((_, i) => i !== idx))
                    }
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>
              ))}
            </div>
          </div>

          <Separator />

          {/* Advanced */}
          <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
            <CollapsibleTrigger
              render={
                <Button
                  type='button'
                  variant='ghost'
                  className='w-full justify-start'
                />
              }
            >
              {advancedOpen ? '▼' : '▶'} {t('Advanced Settings')}
            </CollapsibleTrigger>
            <CollapsibleContent className='space-y-3 pt-2'>
              <div className='grid gap-1.5'>
                <Label>{t('User-Agent include (one per line)')}</Label>
                <Textarea
                  rows={3}
                  placeholder='curl&#10;PostmanRuntime'
                  {...form.register('user_agent_include_text')}
                />
                <FieldHint>
                  {t(
                    'If set, the rule only applies when the User-Agent contains any of these substrings.'
                  )}
                </FieldHint>
              </div>

              <div className='grid grid-cols-2 gap-3'>
                <div className='grid gap-1.5'>
                  <Label>{t('Value Regex')}</Label>
                  <Input
                    placeholder='^[-0-9A-Za-z._:]{1,128}$'
                    {...form.register('value_regex')}
                  />
                  <FieldHint>
                    {t(
                      'The extracted key must match this regex, otherwise the rule is skipped.'
                    )}
                  </FieldHint>
                </div>
                <div className='grid gap-1.5'>
                  <Label>{t('TTL (seconds, 0 = default)')}</Label>
                  <Input
                    type='number'
                    min={0}
                    {...form.register('ttl_seconds')}
                  />
                  <FieldHint>
                    {t(
                      'How long the affinity entry is kept. 0 = use the global default.'
                    )}
                  </FieldHint>
                </div>
              </div>

              <div className='grid gap-1.5'>
                <Label>{t('Parameter Override Template (JSON)')}</Label>
                <Textarea
                  rows={5}
                  placeholder='{"operations": [...]}'
                  {...form.register('param_override_template_json')}
                  className='font-mono text-xs'
                />
                <FieldHint>
                  {t(
                    'Merged into the channel parameter override when this rule hits.'
                  )}
                </FieldHint>
              </div>

              <div className='grid grid-cols-3 gap-3'>
                <div className='flex items-center gap-2'>
                  <Switch
                    checked={form.watch('include_using_group')}
                    onCheckedChange={(v) =>
                      form.setValue('include_using_group', v)
                    }
                  />
                  <Label className='text-xs'>{t('Include Group')}</Label>
                </div>
                <div className='flex items-center gap-2'>
                  <Switch
                    checked={form.watch('include_model_name')}
                    onCheckedChange={(v) =>
                      form.setValue('include_model_name', v)
                    }
                  />
                  <Label className='text-xs'>{t('Include Model')}</Label>
                </div>
                <div className='flex items-center gap-2'>
                  <Switch
                    checked={form.watch('include_rule_name')}
                    onCheckedChange={(v) =>
                      form.setValue('include_rule_name', v)
                    }
                  />
                  <Label className='text-xs'>{t('Include Rule Name')}</Label>
                </div>
              </div>
              <FieldHint>
                {t(
                  'These dimensions are added to the affinity cache key to separate entries per group/model/rule.'
                )}
              </FieldHint>
            </CollapsibleContent>
          </Collapsible>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => props.onOpenChange(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit'>{t('Save')}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
