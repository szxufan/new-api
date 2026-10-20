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
import { z } from 'zod'
import { CHANNEL_STATUS, MODEL_FETCHABLE_TYPES } from '../constants'
import type {
  Channel,
  ChannelPriceSettings,
  ChannelPriceTimeWindow,
  ChannelTimeWindow,
} from '../types'

// ============================================================================
// Stored-data Type Coercion（存量数据类型归一）
// ============================================================================
//
// 存量渠道的 setting/settings JSON 可能由旧版本前端或外部 API 写入，
// 数字被存成字符串（如 "3"）、枚举存成非法值（如 on_hit: "error"）、
// 布尔存成字符串（如 "false"——truthy，语义反转）。这些值若原样透传进表单，
// 会被 zod schema 拒绝，且出错字段可能未渲染（如价格系数在折叠区内），
// 表现为“点击更新按钮无反应”。加载层统一在此归一类型。

/** 将存量值安全转换为有限数字；无法转换时返回 fallback。 */
function coerceNumber(value: unknown, fallback: number): number {
  const coerced = coerceOptionalNumber(value)
  return coerced === undefined ? fallback : coerced
}

/** 将存量值安全转换为有限数字；无法转换（含 null/undefined/空串）时返回 undefined。 */
function coerceOptionalNumber(value: unknown): number | undefined {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : undefined
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

/** 枚举字段只接受合法取值，非法存量值回退到默认值。 */
function pickEnum<T extends string>(
  value: unknown,
  allowed: readonly T[],
  fallback: T
): T {
  return allowed.includes(value as T) ? (value as T) : fallback
}

/** 存量布尔可能是字符串（"true"/"false"）；无法识别的值保守取 false。 */
function coerceBoolean(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (value === 'true') return true
  return false
}

// ============================================================================
// Form Validation Schema
// ============================================================================

export const channelFormSchema = z.object({
  name: z.string().min(1, 'Channel name is required'),
  type: z.number().min(0, 'Channel type is required'),
  base_url: z.string().optional(),
  key: z.string(),
  openai_organization: z.string().optional(),
  // models 不在 schema 层强制非空：空 models 也允许提交，
  // 若 model_mapping 中存在缺失的源模型，会在 onSubmit 中弹窗引导用户补全。
  models: z.string(),
  group: z.array(z.string()).min(1, 'At least one group is required'),
  group_blacklist: z.array(z.string()).optional(),
  model_mapping: z.string().optional(),
  model_parser_mapping: z.string().optional(),
  priority: z.number().optional(),
  weight: z.number().optional(),
  test_model: z.string().optional(),
  auto_ban: z.number().optional(),
  disable_429_ban: z.boolean().optional(), // 是否禁用429自动限流；true=跳过限流走正常重试
  retry_times: z.number().min(-1).max(50).optional(), // 渠道专属重试次数：0=全局，-1=禁止重试，N>0=覆盖
  status: z.number(),
  status_code_mapping: z.string().optional(),
  tag: z.string().optional(),
  remark: z
    .string()
    .max(255, 'Remark must be less than 255 characters')
    .optional(),
  setting: z.string().optional(),
  param_override: z.string().optional(),
  header_override: z.string().optional(),
  settings: z.string().optional(),
  other: z.string().optional(),
  // Scheduled time windows (serialized into time_windows JSON)
  timeWindows: z
    .array(
      z
        .object({
          start: z.string().min(1, 'Start time is required'),
          end: z.string().min(1, 'End time is required'),
        })
        .refine((w) => w.start !== w.end, {
          message: 'Start time and end time cannot be the same',
          path: ['end'],
        })
    )
    .optional(),
  // Channel price factors (stored in setting JSON under "price"; empty = 1, 0 = free)
  price_total: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_input: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_completion: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_cache_read: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_cache_write: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_image_input: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_audio_input: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  price_audio_output: z.number().min(0, 'Must be greater than or equal to 0').optional(),
  priceTimeWindows: z
    .array(
      z
        .object({
          start: z.string().min(1, 'Start time is required'),
          end: z.string().min(1, 'End time is required'),
          ratio: z.number().min(0, 'Must be greater than or equal to 0'),
        })
        .refine((w) => w.start !== w.end, {
          message: 'Start time and end time cannot be the same',
          path: ['end'],
        })
    )
    .optional(),
  // Multi-key options (not sent to backend directly)
  multi_key_mode: z.enum(['single', 'batch', 'multi_to_single']).optional(),
  multi_key_type: z.enum(['random', 'polling']).optional(),
  batch_add_set_key_prefix_2_name: z.boolean().optional(),
  key_mode: z.enum(['append', 'replace']).optional(), // For editing multi-key channels
  // Channel extra settings (stored in setting JSON, not sent directly)
  force_format: z.boolean().optional(),
  thinking_to_content: z.boolean().optional(),
  proxy: z.string().optional(),
  pass_through_body_enabled: z.boolean().optional(),
  system_prompt: z.string().optional(),
  system_prompt_override: z.boolean().optional(),
  anti_cache_test: z.boolean().optional(),
  anti_cache_retry_enabled: z.boolean().optional(),
  anti_cache_retry_content: z.string().optional(),
  // Response detection settings (stored in setting JSON)
  response_detection_enabled: z.boolean().optional(),
  response_detection_keywords: z.string().optional(), // comma-separated
  response_detection_max_retries: z.number().optional(),
  response_detection_on_hit: z.enum(['retry', 'abort']).optional(),
  response_detection_treat_empty_as_hit: z.boolean().optional(),
  // Type-specific settings (stored in settings JSON)
  is_enterprise_account: z.boolean().optional(), // OpenRouter specific
  vertex_key_type: z.enum(['json', 'api_key']).optional(), // Vertex AI specific
  aws_key_type: z.enum(['ak_sk', 'api_key']).optional(), // AWS specific
  azure_responses_version: z.string().optional(), // Azure specific
  // Field passthrough controls (stored in settings JSON)
  allow_service_tier: z.boolean().optional(), // OpenAI/Anthropic
  disable_store: z.boolean().optional(), // OpenAI only
  allow_safety_identifier: z.boolean().optional(), // OpenAI only
  allow_include_obfuscation: z.boolean().optional(), // OpenAI: include usage obfuscation
  allow_inference_geo: z.boolean().optional(), // OpenAI/Anthropic: inference geography
  allow_speed: z.boolean().optional(), // Anthropic: speed mode control
  claude_beta_query: z.boolean().optional(), // Anthropic: beta query passthrough
  // Upstream model update settings (stored in settings JSON)
  upstream_model_update_check_enabled: z.boolean().optional(),
  upstream_model_update_auto_sync_enabled: z.boolean().optional(),
  upstream_model_update_ignored_models: z.string().optional(),
  // Fallback channels (stored in other_info JSON)
  fallback_channel_ids: z.array(z.number()).optional(),
  // Periodic balance reset (top-level channel fields; enabled switch is form-only)
  balance_reset_enabled: z.boolean().optional(),
  balance_reset_amount: z
    .number()
    .min(0, 'Must be greater than or equal to 0')
    .nullish(), // 重置目标金额（USD，绝对值）
  // 重置周期（天）。基础校验仅保留数字类型：功能停用时后端约定三个字段存 0
  // （而非 null），若此处强制 min(1)，存量 0 值会导致整个表单校验失败——而该
  // 字段在停用状态下不渲染，错误不可见，表现为“点击更新按钮无反应”。
  // 启用状态的必填/整数/下限校验在下方 superRefine 中执行。
  balance_reset_period_days: z.number().nullish(),
  balance_reset_next_time: z.number().nullish(), // 下次重置时间（unix 秒）
  // 共享余额（顶层渠道字段；enabled 开关仅表单使用，0=未启用）
  balance_share_enabled: z.boolean().optional(),
  balance_share_channel_id: z.number().optional(),
}).superRefine((data, ctx) => {
  // 开启共享余额时必须选择共享目标渠道
  if (data.balance_share_enabled && !data.balance_share_channel_id) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Please select a channel to share balance with',
      path: ['balance_share_channel_id'],
    })
  }
  // 启用周期重置时，金额/周期/下次时间三项必填
  if (!data.balance_reset_enabled) return
  if (data.balance_reset_amount === undefined || data.balance_reset_amount === null) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Reset amount is required',
      path: ['balance_reset_amount'],
    })
  }
  const periodDays = data.balance_reset_period_days
  if (
    typeof periodDays !== 'number' ||
    !Number.isInteger(periodDays) ||
    periodDays < 1
  ) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Reset period must be at least 1 day',
      path: ['balance_reset_period_days'],
    })
  }
  if (!data.balance_reset_next_time || data.balance_reset_next_time <= 0) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'Next reset time is required',
      path: ['balance_reset_next_time'],
    })
  }
})

export type ChannelFormValues = z.infer<typeof channelFormSchema>

// ============================================================================
// Default Form Values
// ============================================================================

export const CHANNEL_FORM_DEFAULT_VALUES: ChannelFormValues = {
  name: '',
  type: 1,
  base_url: '',
  key: '',
  openai_organization: '',
  models: '',
  group: ['default'],
  group_blacklist: [],
  model_mapping: '',
  model_parser_mapping: '',
  priority: 0,
  weight: 0,
  test_model: '',
  auto_ban: 1,
  disable_429_ban: false, // 默认关闭：维持现状（429限流+fallback）
  retry_times: 0,
  status: CHANNEL_STATUS.ENABLED,
  status_code_mapping: '',
  tag: '',
  remark: '',
  setting: '',
  param_override: '',
  header_override: '',
  settings: '{}',
  other: '',
  timeWindows: [],
  priceTimeWindows: [],
  multi_key_mode: 'single',
  multi_key_type: 'random',
  batch_add_set_key_prefix_2_name: false,
  key_mode: 'append',
  // Channel extra settings
  force_format: false,
  thinking_to_content: false,
  proxy: '',
  pass_through_body_enabled: false,
  system_prompt: '',
  system_prompt_override: false,
  anti_cache_test: false,
  anti_cache_retry_enabled: false,
  anti_cache_retry_content: '',
  // Response detection settings
  response_detection_enabled: false,
  response_detection_keywords: '',
  response_detection_max_retries: 0,
  response_detection_on_hit: 'retry',
  response_detection_treat_empty_as_hit: false,
  // Type-specific settings
  is_enterprise_account: false,
  vertex_key_type: 'json',
  aws_key_type: 'ak_sk',
  azure_responses_version: '',
  // Field passthrough controls
  allow_service_tier: false,
  disable_store: false,
  allow_safety_identifier: false,
  allow_include_obfuscation: false,
  allow_inference_geo: false,
  allow_speed: false,
  claude_beta_query: false,
  upstream_model_update_check_enabled: false,
  upstream_model_update_auto_sync_enabled: false,
  upstream_model_update_ignored_models: '',
  fallback_channel_ids: [],
  // Periodic balance reset
  balance_reset_enabled: false,
  // Shared balance
  balance_share_enabled: false,
  balance_share_channel_id: 0,
}

// ============================================================================
// Transform Functions
// ============================================================================

/**
 * Transform Channel from API to Form default values
 */
export function transformChannelToFormDefaults(
  channel: Channel
): ChannelFormValues {
  // Parse channel extra settings from setting field
  let extraSettings = {
    force_format: false,
    thinking_to_content: false,
    proxy: '',
    pass_through_body_enabled: false,
    system_prompt: '',
    system_prompt_override: false,
    anti_cache_test: false,
    anti_cache_retry_enabled: false,
    anti_cache_retry_content: '',
    response_detection_enabled: false,
    response_detection_keywords: '',
    response_detection_max_retries: 0,
    response_detection_on_hit: 'retry' as 'retry' | 'abort',
    response_detection_treat_empty_as_hit: false,
  }
  let channelPrice: ChannelPriceSettings | undefined

  if (channel.setting) {
    try {
      const parsed = JSON.parse(channel.setting)
      channelPrice = parsed.price
      extraSettings = {
        force_format: coerceBoolean(parsed.force_format),
        thinking_to_content: coerceBoolean(parsed.thinking_to_content),
        proxy: parsed.proxy || '',
        pass_through_body_enabled: coerceBoolean(
          parsed.pass_through_body_enabled
        ),
        system_prompt: parsed.system_prompt || '',
        system_prompt_override: coerceBoolean(parsed.system_prompt_override),
        anti_cache_test: coerceBoolean(parsed.anti_cache_test),
        anti_cache_retry_enabled: coerceBoolean(parsed.anti_cache_retry_enabled),
        anti_cache_retry_content: parsed.anti_cache_retry_content || '',
        response_detection_enabled: coerceBoolean(
          parsed.response_detection?.enabled
        ),
        response_detection_keywords: Array.isArray(
          parsed.response_detection?.keywords
        )
          ? parsed.response_detection.keywords.join(',')
          : '',
        response_detection_max_retries: coerceNumber(
          parsed.response_detection?.max_retries,
          0
        ),
        response_detection_on_hit: pickEnum(
          parsed.response_detection?.on_hit,
          ['retry', 'abort'] as const,
          'retry'
        ),
        response_detection_treat_empty_as_hit: coerceBoolean(
          parsed.response_detection?.treat_empty_as_hit
        ),
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to parse channel setting:', error)
    }
  }

  // Parse type-specific settings from settings field
  let vertexKeyType: 'json' | 'api_key' = 'json'
  let azureResponsesVersion = ''
  let isEnterpriseAccount = false
  let awsKeyType: 'ak_sk' | 'api_key' = 'ak_sk'
  let allowServiceTier = false
  let disableStore = false
  let allowSafetyIdentifier = false
  let allowIncludeObfuscation = false
  let allowInferenceGeo = false
  let allowSpeed = false
  let claudeBetaQuery = false
  let upstreamModelUpdateCheckEnabled = false
  let upstreamModelUpdateAutoSyncEnabled = false
  let upstreamModelUpdateIgnoredModels = ''

  if (channel.settings) {
    try {
      const parsed = JSON.parse(channel.settings)
      vertexKeyType = pickEnum(parsed.vertex_key_type, ['json', 'api_key'] as const, 'json')
      azureResponsesVersion =
        typeof parsed.azure_responses_version === 'string'
          ? parsed.azure_responses_version
          : ''
      isEnterpriseAccount = parsed.openrouter_enterprise === true
      awsKeyType = pickEnum(parsed.aws_key_type, ['ak_sk', 'api_key'] as const, 'ak_sk')
      allowServiceTier = parsed.allow_service_tier === true
      disableStore = parsed.disable_store === true
      allowSafetyIdentifier = parsed.allow_safety_identifier === true
      allowIncludeObfuscation = parsed.allow_include_obfuscation === true
      allowInferenceGeo = parsed.allow_inference_geo === true
      allowSpeed = parsed.allow_speed === true
      claudeBetaQuery = parsed.claude_beta_query === true
      upstreamModelUpdateCheckEnabled =
        parsed.upstream_model_update_check_enabled === true
      upstreamModelUpdateAutoSyncEnabled =
        parsed.upstream_model_update_auto_sync_enabled === true
      upstreamModelUpdateIgnoredModels = Array.isArray(
        parsed.upstream_model_update_ignored_models
      )
        ? parsed.upstream_model_update_ignored_models.join(',')
        : ''
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to parse channel settings:', error)
    }
  }

  // Parse fallback_channel_ids from other_info
  let fallbackChannelIds: number[] = []
  if (channel.other_info) {
    try {
      const otherInfo = JSON.parse(channel.other_info)
      if (Array.isArray(otherInfo.fallback_channel_ids)) {
        fallbackChannelIds = otherInfo.fallback_channel_ids
      }
    } catch {
      // ignore
    }
  }

  // Parse scheduled time windows from time_windows JSON
  const timeWindows = parseTimeWindowsJson(channel.time_windows || '')

  // Periodic balance reset (top-level channel fields)
  // 后端约定：功能停用时三个字段存 0（next_time <= 0 视为未启用）。停用状态下
  // 表单值统一归一为 undefined，避免存量 0 值触发 schema 校验或残留在表单状态中。
  const balanceResetEnabled =
    channel.balance_reset_amount != null &&
    channel.balance_reset_period_days != null &&
    channel.balance_reset_period_days > 0 &&
    channel.balance_reset_next_time != null &&
    channel.balance_reset_next_time > 0

  return {
    name: channel.name || '',
    type: channel.type,
    base_url: channel.base_url || '',
    key: '', // Never populate key from backend for security
    openai_organization: channel.openai_organization || '',
    models: channel.models || '',
    group: parseGroups(channel.group || 'default'),
    group_blacklist: parseGroups(channel.group_blacklist || ''),
    model_mapping: channel.model_mapping || '',
    model_parser_mapping: channel.model_parser_mapping || '',
    priority: channel.priority || 0,
    weight: channel.weight || 0,
    test_model: channel.test_model || '',
    auto_ban: channel.auto_ban ?? 1,
    disable_429_ban: channel.channel_info?.disable_429_ban ?? false,
    retry_times: channel.retry_times ?? 0,
    status: channel.status,
    status_code_mapping: channel.status_code_mapping || '',
    tag: channel.tag || '',
    remark: channel.remark || '',
    setting: channel.setting || '',
    param_override: channel.param_override || '',
    header_override: channel.header_override || '',
    settings: channel.settings || '{}',
    other: channel.other || '',
    multi_key_mode: 'single',
    multi_key_type: pickEnum(
      channel.channel_info?.multi_key_mode,
      ['random', 'polling'] as const,
      'random'
    ),
    batch_add_set_key_prefix_2_name: false,
    key_mode: 'append', // Default to append mode for editing multi-key channels
    // Channel extra settings
    ...extraSettings,
    // Type-specific settings
    is_enterprise_account: isEnterpriseAccount,
    vertex_key_type: vertexKeyType,
    azure_responses_version: azureResponsesVersion,
    aws_key_type: awsKeyType,
    allow_service_tier: allowServiceTier,
    disable_store: disableStore,
    allow_include_obfuscation: allowIncludeObfuscation,
    allow_inference_geo: allowInferenceGeo,
    allow_speed: allowSpeed,
    claude_beta_query: claudeBetaQuery,
    allow_safety_identifier: allowSafetyIdentifier,
    upstream_model_update_check_enabled: upstreamModelUpdateCheckEnabled,
    upstream_model_update_auto_sync_enabled: upstreamModelUpdateAutoSyncEnabled,
    upstream_model_update_ignored_models: upstreamModelUpdateIgnoredModels,
    fallback_channel_ids: fallbackChannelIds,
    timeWindows,
    // Periodic balance reset (top-level channel fields)
    balance_reset_enabled: balanceResetEnabled,
    balance_reset_amount: balanceResetEnabled
      ? channel.balance_reset_amount
      : undefined,
    balance_reset_period_days: balanceResetEnabled
      ? channel.balance_reset_period_days
      : undefined,
    balance_reset_next_time: balanceResetEnabled
      ? channel.balance_reset_next_time
      : undefined,
    // 共享余额（顶层渠道字段）
    balance_share_enabled: (channel.balance_share_channel_id ?? 0) > 0,
    balance_share_channel_id: channel.balance_share_channel_id ?? 0,
    // Channel price factors (stored in setting JSON under "price")
    ...parseChannelPriceSettings(channelPrice),
  }
}

/**
 * Parse time_windows JSON string into time window rows for the form editor
 */
export function parseTimeWindowsJson(value: string): ChannelTimeWindow[] {
  if (!value || value.trim() === '') return []
  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (w) =>
          w &&
          typeof w.start === 'string' &&
          typeof w.end === 'string' &&
          w.start !== '' &&
          w.end !== ''
      )
      .map((w) => ({ start: w.start, end: w.end }))
  } catch {
    return []
  }
}

/**
 * Serialize form time window rows into the time_windows JSON string.
 * Returns an empty string when no valid windows remain (feature disabled).
 */
export function formatTimeWindowsJson(
  windows: ChannelTimeWindow[] | undefined
): string {
  const valid = (windows || []).filter((w) => w.start && w.end)
  return valid.length > 0 ? JSON.stringify(valid) : ''
}

// ============================================================================
// Channel Price Factors（渠道价格系数）解析与序列化
// ============================================================================

export type ChannelPriceFormFields = Pick<
  ChannelFormValues,
  | 'price_total'
  | 'price_input'
  | 'price_completion'
  | 'price_cache_read'
  | 'price_cache_write'
  | 'price_image_input'
  | 'price_audio_input'
  | 'price_audio_output'
  | 'priceTimeWindows'
>

/**
 * 解析 setting JSON 中的 price 对象为表单字段。
 * 未配置的系数保持 undefined（与显式 0=免费 区分）。
 */
export function parseChannelPriceSettings(
  price: ChannelPriceSettings | undefined
): ChannelPriceFormFields {
  // 存量数据归一：数字字段可能是字符串（如 "0.5"）；时段行的 ratio 无法转换时
  // 整行丢弃，避免垃圾值被静默归为 0（免费）。
  const timeWindows: ChannelPriceTimeWindow[] = Array.isArray(
    price?.time_windows
  )
    ? price.time_windows.flatMap((w) => {
        if (
          !w ||
          typeof w.start !== 'string' ||
          typeof w.end !== 'string'
        ) {
          return []
        }
        const ratio = coerceOptionalNumber(w.ratio)
        return ratio === undefined ? [] : [{ start: w.start, end: w.end, ratio }]
      })
    : []
  return {
    price_total: coerceOptionalNumber(price?.total),
    price_input: coerceOptionalNumber(price?.input),
    price_completion: coerceOptionalNumber(price?.completion),
    price_cache_read: coerceOptionalNumber(price?.cache_read),
    price_cache_write: coerceOptionalNumber(price?.cache_write),
    price_image_input: coerceOptionalNumber(price?.image_input),
    price_audio_input: coerceOptionalNumber(price?.audio_input),
    price_audio_output: coerceOptionalNumber(price?.audio_output),
    priceTimeWindows: timeWindows,
  }
}

/**
 * 组装 setting JSON 中的 price 对象（仅写入已配置字段；显式 0 必须保留）。
 * 无任何配置时返回 undefined（不写 price 键）。
 */
export function buildChannelPriceSettings(
  formData: ChannelFormValues
): ChannelPriceSettings | undefined {
  const price: ChannelPriceSettings = {}
  if (formData.price_total !== undefined) price.total = formData.price_total
  if (formData.price_input !== undefined) price.input = formData.price_input
  if (formData.price_completion !== undefined) {
    price.completion = formData.price_completion
  }
  if (formData.price_cache_read !== undefined) {
    price.cache_read = formData.price_cache_read
  }
  if (formData.price_cache_write !== undefined) {
    price.cache_write = formData.price_cache_write
  }
  if (formData.price_image_input !== undefined) {
    price.image_input = formData.price_image_input
  }
  if (formData.price_audio_input !== undefined) {
    price.audio_input = formData.price_audio_input
  }
  if (formData.price_audio_output !== undefined) {
    price.audio_output = formData.price_audio_output
  }

  const timeWindows = (formData.priceTimeWindows || []).filter(
    (w) => w.start && w.end
  )
  if (timeWindows.length > 0) {
    price.time_windows = timeWindows.map((w) => ({
      start: w.start,
      end: w.end,
      ratio: w.ratio ?? 0,
    }))
  }

  return Object.keys(price).length > 0 ? price : undefined
}

/**
 * Build the setting JSON string from form extra settings
 */
function buildSettingJSON(formData: ChannelFormValues): string {
  const settingObj: Record<string, unknown> = {
    force_format: formData.force_format || false,
    thinking_to_content: formData.thinking_to_content || false,
    proxy: formData.proxy || '',
    pass_through_body_enabled: formData.pass_through_body_enabled || false,
    system_prompt: formData.system_prompt || '',
    system_prompt_override: formData.system_prompt_override || false,
    anti_cache_test: formData.anti_cache_test || false,
    anti_cache_retry_enabled: formData.anti_cache_retry_enabled || false,
    anti_cache_retry_content: formData.anti_cache_retry_content || '',
  }

  // Only include response_detection if enabled
  if (formData.response_detection_enabled) {
    const keywords = formData.response_detection_keywords
      ? formData.response_detection_keywords
          .split(',')
          .map((k) => k.trim())
          .filter((k) => k.length > 0)
      : []
    settingObj.response_detection = {
      enabled: true,
      keywords,
      max_retries: formData.response_detection_max_retries || 0,
      on_hit: formData.response_detection_on_hit || 'retry',
      treat_empty_as_hit:
        formData.response_detection_treat_empty_as_hit || false,
    }
  }

  // 渠道价格系数：仅有配置时写入 price 键（显式 0=免费 必须保留）
  const priceSettings = buildChannelPriceSettings(formData)
  if (priceSettings) {
    settingObj.price = priceSettings
  }

  return JSON.stringify(settingObj)
}

/**
 * Build the settings JSON string (for type-specific config like vertex_key_type)
 */
function buildSettingsJSON(formData: ChannelFormValues): string {
  let settingsObj: Record<string, unknown> = {}

  // Try to parse existing settings first
  if (formData.settings && formData.settings !== '{}') {
    try {
      settingsObj = JSON.parse(formData.settings)
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to parse existing settings:', error)
    }
  }

  // Add vertex_key_type for Vertex AI channels (type 41)
  if (formData.type === 41) {
    settingsObj.vertex_key_type = formData.vertex_key_type || 'json'
  } else if ('vertex_key_type' in settingsObj) {
    delete settingsObj.vertex_key_type
  }

  // Add azure_responses_version for Azure channels (type 3)
  if (formData.type === 3 && formData.azure_responses_version) {
    settingsObj.azure_responses_version = formData.azure_responses_version
  } else if ('azure_responses_version' in settingsObj) {
    delete settingsObj.azure_responses_version
  }

  // Add enterprise account setting for OpenRouter (type 20)
  if (formData.type === 20) {
    settingsObj.openrouter_enterprise = formData.is_enterprise_account === true
  } else if ('openrouter_enterprise' in settingsObj) {
    delete settingsObj.openrouter_enterprise
  }

  // Add aws_key_type for AWS channels (type 33)
  if (formData.type === 33) {
    settingsObj.aws_key_type = formData.aws_key_type || 'ak_sk'
  } else if ('aws_key_type' in settingsObj) {
    delete settingsObj.aws_key_type
  }

  // Field passthrough controls:
  // - OpenAI (type 1) and Anthropic (type 14): allow_service_tier
  // - OpenAI only: disable_store, allow_safety_identifier
  if (formData.type === 1 || formData.type === 14) {
    settingsObj.allow_service_tier = formData.allow_service_tier === true
  } else if ('allow_service_tier' in settingsObj) {
    delete settingsObj.allow_service_tier
  }

  if (formData.type === 1) {
    settingsObj.disable_store = formData.disable_store === true
    settingsObj.allow_safety_identifier =
      formData.allow_safety_identifier === true
    settingsObj.allow_include_obfuscation =
      formData.allow_include_obfuscation === true
    settingsObj.allow_inference_geo = formData.allow_inference_geo === true
  } else {
    if ('disable_store' in settingsObj) delete settingsObj.disable_store
    if ('allow_safety_identifier' in settingsObj)
      delete settingsObj.allow_safety_identifier
    if ('allow_include_obfuscation' in settingsObj)
      delete settingsObj.allow_include_obfuscation
    if (formData.type !== 14 && 'allow_inference_geo' in settingsObj)
      delete settingsObj.allow_inference_geo
  }

  // Anthropic (type 14): claude_beta_query, allow_inference_geo, allow_speed
  if (formData.type === 14) {
    settingsObj.allow_inference_geo = formData.allow_inference_geo === true
    settingsObj.allow_speed = formData.allow_speed === true
    settingsObj.claude_beta_query = formData.claude_beta_query === true
  } else {
    if ('allow_speed' in settingsObj) delete settingsObj.allow_speed
    if ('claude_beta_query' in settingsObj) delete settingsObj.claude_beta_query
  }

  // Upstream model update settings (for model-fetchable channel types)
  if (MODEL_FETCHABLE_TYPES.has(formData.type)) {
    settingsObj.upstream_model_update_check_enabled =
      formData.upstream_model_update_check_enabled === true
    settingsObj.upstream_model_update_auto_sync_enabled =
      settingsObj.upstream_model_update_check_enabled === true &&
      formData.upstream_model_update_auto_sync_enabled === true
    settingsObj.upstream_model_update_ignored_models = Array.from(
      new Set(
        String(formData.upstream_model_update_ignored_models || '')
          .split(',')
          .map((model) => model.trim())
          .filter(Boolean)
      )
    )
    if (
      !Array.isArray(settingsObj.upstream_model_update_last_detected_models) ||
      settingsObj.upstream_model_update_check_enabled !== true
    ) {
      settingsObj.upstream_model_update_last_detected_models = []
    }
    if (typeof settingsObj.upstream_model_update_last_check_time !== 'number') {
      settingsObj.upstream_model_update_last_check_time = 0
    }
  }

  return JSON.stringify(settingsObj)
}

/**
 * Transform form data to API payload for creating channel
 */
export function transformFormDataToCreatePayload(formData: ChannelFormValues): {
  mode: 'single' | 'batch' | 'multi_to_single'
  multi_key_mode?: 'random' | 'polling'
  batch_add_set_key_prefix_2_name?: boolean
  channel: Partial<Channel>
  disable_429_ban?: boolean
} {
  const mode = formData.multi_key_mode || 'single'

  const channel: Partial<Channel> = {
    name: formData.name,
    type: formData.type,
    base_url: formData.base_url || null,
    key: formData.key,
    openai_organization: formData.openai_organization || null,
    models: formData.models,
    group: formatGroups(formData.group),
    group_blacklist: formatGroups(formData.group_blacklist || []),
    model_mapping: formData.model_mapping || null,
    model_parser_mapping: formData.model_parser_mapping || null,
    priority: formData.priority || null,
    weight: formData.weight || null,
    test_model: formData.test_model || null,
    auto_ban: formData.auto_ban ?? 1,
    status: formData.status,
    status_code_mapping: formData.status_code_mapping || null,
    tag: formData.tag || null,
    remark: formData.remark || '',
    retry_times: formData.retry_times ?? 0,
    setting: buildSettingJSON(formData),
    param_override: formData.param_override || null,
    header_override: formData.header_override || null,
    time_windows: formatTimeWindowsJson(formData.timeWindows),
    settings: buildSettingsJSON(formData),
    other: formData.other || '',
    balance_reset_amount: formData.balance_reset_enabled
      ? (formData.balance_reset_amount ?? null)
      : null,
    balance_reset_period_days: formData.balance_reset_enabled
      ? (formData.balance_reset_period_days ?? null)
      : null,
    balance_reset_next_time: formData.balance_reset_enabled
      ? (formData.balance_reset_next_time ?? null)
      : null,
    // 共享余额：始终显式提交数字（0=未启用）
    balance_share_channel_id: formData.balance_share_enabled
      ? (formData.balance_share_channel_id ?? 0)
      : 0,
  }

  // Write fallback_channel_ids into other_info
  if (
    formData.fallback_channel_ids &&
    formData.fallback_channel_ids.length > 0
  ) {
    channel.other_info = JSON.stringify({
      fallback_channel_ids: formData.fallback_channel_ids,
    })
  }

  // Clean up empty strings to null for optional fields
  Object.keys(channel).forEach((key) => {
    if (channel[key as keyof typeof channel] === '') {
      ;(channel as Record<string, unknown>)[key] = null
    }
  })

  return {
    mode,
    multi_key_mode:
      mode === 'multi_to_single' ? formData.multi_key_type : undefined,
    batch_add_set_key_prefix_2_name:
      mode === 'batch' ? formData.batch_add_set_key_prefix_2_name : undefined,
    channel,
    disable_429_ban: formData.disable_429_ban ?? false,
  }
}

/**
 * Transform form data to API payload for updating channel
 */
export function transformFormDataToUpdatePayload(
  formData: ChannelFormValues,
  channelId: number
): Partial<Channel> & { disable_429_ban?: boolean } {
  const payload: Partial<Channel> & { disable_429_ban?: boolean } = {
    id: channelId,
    name: formData.name,
    type: formData.type,
    base_url: formData.base_url || null,
    openai_organization: formData.openai_organization || null,
    models: formData.models,
    group: formatGroups(formData.group),
    group_blacklist: formatGroups(formData.group_blacklist || []),
    model_mapping: formData.model_mapping || null,
    model_parser_mapping: formData.model_parser_mapping || null,
    priority: formData.priority ?? 0,
    weight: formData.weight ?? 0,
    test_model: formData.test_model || null,
    auto_ban: formData.auto_ban ?? 1,
    status: formData.status,
    status_code_mapping: formData.status_code_mapping || null,
    tag: formData.tag || null,
    remark: formData.remark || '',
    retry_times: formData.retry_times ?? 0,
    setting: buildSettingJSON(formData),
    param_override: formData.param_override || null,
    header_override: formData.header_override || null,
    time_windows: formatTimeWindowsJson(formData.timeWindows),
    settings: buildSettingsJSON(formData),
    other: formData.other || '',
    balance_reset_amount: formData.balance_reset_enabled
      ? (formData.balance_reset_amount ?? null)
      : null,
    balance_reset_period_days: formData.balance_reset_enabled
      ? (formData.balance_reset_period_days ?? null)
      : null,
    balance_reset_next_time: formData.balance_reset_enabled
      ? (formData.balance_reset_next_time ?? null)
      : null,
  }

  // Only include key if it was changed (not empty)
  if (formData.key && formData.key.trim()) {
    payload.key = formData.key
  }

  // Clean up empty strings to null for optional fields
  Object.keys(payload).forEach((key) => {
    if (payload[key as keyof typeof payload] === '') {
      ;(payload as Record<string, unknown>)[key] = null
    }
  })

  // Send explicit empty strings for nullable fields so GORM updates can clear them.
  payload.base_url = formData.base_url || ''
  payload.group_blacklist = formatGroups(formData.group_blacklist || [])
  payload.openai_organization = formData.openai_organization || ''
  payload.test_model = formData.test_model || ''
  payload.tag = formData.tag || ''
  payload.remark = formData.remark || ''
  payload.model_mapping = formData.model_mapping || ''
  // 工具调用降级映射：空时提交空字符串（而非 null），让 GORM Updates 能清空该字段
  payload.model_parser_mapping = formData.model_parser_mapping || ''
  payload.status_code_mapping = formData.status_code_mapping || ''
  payload.param_override = formData.param_override || ''
  payload.header_override = formData.header_override || ''
  // time_windows 为空时提交空字符串（而非 null），让 GORM Updates 能清空该字段
  payload.time_windows = formatTimeWindowsJson(formData.timeWindows)
  // 周期重置余额：关闭时显式提交 0（而非 null），让 GORM Updates 能清空旧值；
  // 后端约定 next_time <= 0 视为未启用。共享余额与周期重置互斥，
  // 开启共享时强制提交 0 清空周期重置配置（后端也会跳过共享渠道的重置）。
  const resetEnabled =
    formData.balance_reset_enabled && !formData.balance_share_enabled
  payload.balance_reset_amount = resetEnabled
    ? (formData.balance_reset_amount ?? 0)
    : 0
  payload.balance_reset_period_days = resetEnabled
    ? (formData.balance_reset_period_days ?? 0)
    : 0
  payload.balance_reset_next_time = resetEnabled
    ? (formData.balance_reset_next_time ?? 0)
    : 0
  // 共享余额：始终显式提交数字（0=关闭），让 GORM Updates 能清空旧值
  payload.balance_share_channel_id = formData.balance_share_enabled
    ? (formData.balance_share_channel_id ?? 0)
    : 0

  // Write fallback_channel_ids into other_info
  if (
    formData.fallback_channel_ids &&
    formData.fallback_channel_ids.length > 0
  ) {
    payload.other_info = JSON.stringify({
      fallback_channel_ids: formData.fallback_channel_ids,
    })
  } else {
    payload.other_info = '{}'
  }

  // 顶层 disable_429_ban 字段（与 channel 平级，由后端 PatchChannel 接收）
  payload.disable_429_ban = formData.disable_429_ban ?? false

  return payload
}

// ============================================================================
// Validation Helpers
// ============================================================================

/**
 * Validate JSON string
 */
export function validateJSON(value: string): boolean {
  if (!value || value.trim() === '') return true
  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

/**
 * Validate model mapping format
 */
export function validateModelMapping(value: string): boolean {
  if (!value || value.trim() === '') return true
  return validateJSON(value)
}

/**
 * Parse models string to array
 */
export function parseModels(models: string): string[] {
  if (!models) return []
  return models
    .split(',')
    .map((m) => m.trim())
    .filter((m) => m.length > 0)
}

/**
 * Parse groups string to array
 */
export function parseGroups(groups: string): string[] {
  if (!groups) return []
  return groups
    .split(',')
    .map((g) => g.trim())
    .filter((g) => g.length > 0)
}

/**
 * Format models array to string
 */
export function formatModels(models: string[]): string {
  return models.join(',')
}

/**
 * Format groups array to string
 */
export function formatGroups(groups: string[]): string {
  return groups.join(',')
}
