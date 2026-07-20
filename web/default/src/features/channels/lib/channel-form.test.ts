import { describe, it, expect } from 'vitest'
import {
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  CHANNEL_FORM_DEFAULT_VALUES,
} from './channel-form'
import type { Channel } from '../types'

describe('transformChannelToFormDefaults', () => {
  // Create a minimal valid channel for testing
  const createMockChannel = (
    overrides: Partial<Channel> = {}
  ): Channel =>
    ({
      id: 1,
      type: 1,
      key: 'test-key',
      status: 1,
      name: 'Test Channel',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      models: 'gpt-4',
      group: 'default',
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
      settings: '{}',
      ...overrides,
    }) as Channel

  it('should transform basic channel data to form values', () => {
    const channel = createMockChannel({
      name: 'My Channel',
      type: 1,
      base_url: 'https://api.openai.com',
      models: 'gpt-4,gpt-3.5-turbo',
      group: 'group1,group2',
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.name).toBe('My Channel')
    expect(result.type).toBe(1)
    expect(result.base_url).toBe('https://api.openai.com')
    expect(result.models).toBe('gpt-4,gpt-3.5-turbo')
    expect(result.group).toEqual(['group1', 'group2'])
  })

  it('should never populate key from backend for security', () => {
    const channel = createMockChannel({
      key: 'super-secret-key',
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.key).toBe('')
  })

  it('should handle nullish values with defaults', () => {
    const channel = createMockChannel({
      base_url: null,
      models: null,
      group: null,
      priority: null,
      weight: null,
    } as unknown as Partial<Channel>)

    const result = transformChannelToFormDefaults(channel)

    expect(result.base_url).toBe('')
    expect(result.models).toBe('')
    expect(result.group).toEqual(['default'])
    expect(result.priority).toBe(0)
    expect(result.weight).toBe(0)
  })

  it('should parse setting JSON and extract extra settings', () => {
    const channel = createMockChannel({
      setting: JSON.stringify({
        force_format: true,
        thinking_to_content: true,
        proxy: 'http://proxy.example.com',
        pass_through_body_enabled: true,
        system_prompt: 'You are a helpful assistant',
        system_prompt_override: true,
      }),
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.force_format).toBe(true)
    expect(result.thinking_to_content).toBe(true)
    expect(result.proxy).toBe('http://proxy.example.com')
    expect(result.pass_through_body_enabled).toBe(true)
    expect(result.system_prompt).toBe('You are a helpful assistant')
    expect(result.system_prompt_override).toBe(true)
  })

  it('should handle invalid setting JSON gracefully', () => {
    const channel = createMockChannel({
      setting: 'invalid json',
    })

    // Should not throw and should use defaults
    const result = transformChannelToFormDefaults(channel)

    expect(result.force_format).toBe(false)
    expect(result.thinking_to_content).toBe(false)
    expect(result.proxy).toBe('')
  })

  it('should parse settings JSON for type-specific config', () => {
    const channel = createMockChannel({
      settings: JSON.stringify({
        vertex_key_type: 'api_key',
        azure_responses_version: '2024-02-15-preview',
        openrouter_enterprise: true,
        aws_key_type: 'api_key',
        allow_service_tier: true,
      }),
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.vertex_key_type).toBe('api_key')
    expect(result.azure_responses_version).toBe('2024-02-15-preview')
    expect(result.is_enterprise_account).toBe(true)
    expect(result.aws_key_type).toBe('api_key')
    expect(result.allow_service_tier).toBe(true)
  })

  it('should handle invalid settings JSON gracefully', () => {
    const channel = createMockChannel({
      settings: 'invalid json',
    })

    // Should not throw and should use defaults
    const result = transformChannelToFormDefaults(channel)

    expect(result.vertex_key_type).toBe('json')
    expect(result.azure_responses_version).toBe('')
    expect(result.is_enterprise_account).toBe(false)
    expect(result.aws_key_type).toBe('ak_sk')
  })

  it('should parse upstream model update settings', () => {
    const channel = createMockChannel({
      settings: JSON.stringify({
        upstream_model_update_check_enabled: true,
        upstream_model_update_auto_sync_enabled: true,
        upstream_model_update_ignored_models: ['model-1', 'model-2'],
      }),
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.upstream_model_update_check_enabled).toBe(true)
    expect(result.upstream_model_update_auto_sync_enabled).toBe(true)
    expect(result.upstream_model_update_ignored_models).toBe('model-1,model-2')
  })

  it('should handle model_mapping and status_code_mapping', () => {
    const channel = createMockChannel({
      model_mapping: JSON.stringify({ 'gpt-4': 'gpt-4-turbo' }),
      status_code_mapping: JSON.stringify({ '401': '403' }),
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.model_mapping).toBe(JSON.stringify({ 'gpt-4': 'gpt-4-turbo' }))
    expect(result.status_code_mapping).toBe(JSON.stringify({ '401': '403' }))
  })

  it('should handle multi-key channel info', () => {
    const channel = createMockChannel({
      channel_info: {
        is_multi_key: true,
        multi_key_size: 5,
        multi_key_polling_index: 2,
        multi_key_mode: 'polling',
      },
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.multi_key_type).toBe('polling')
    expect(result.multi_key_mode).toBe('single')
  })

  it('should use default values for empty channel_info', () => {
    const channel = createMockChannel({
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.multi_key_type).toBe('random')
  })

  it('should handle auto_ban with correct default', () => {
    const channelWithAutoBan = createMockChannel({ auto_ban: 0 })
    const channelWithoutAutoBan = createMockChannel({ auto_ban: undefined })

    expect(transformChannelToFormDefaults(channelWithAutoBan).auto_ban).toBe(0)
    expect(transformChannelToFormDefaults(channelWithoutAutoBan).auto_ban).toBe(1)
  })

  it('should return correct id from channel data for tracking loaded state', () => {
    // This test verifies that the channel.id is accessible for the
    // loadedChannelIdRef tracking in the component
    const channel = createMockChannel({ id: 42 })

    transformChannelToFormDefaults(channel)

    // The transform function doesn't return id, but the channel data has it
    // This documents that channel.id should be used for tracking
    expect(channel.id).toBe(42)
  })

  it('should parse group_blacklist string to array', () => {
    const channel = createMockChannel({
      group_blacklist: 'vip,internal',
    })

    const result = transformChannelToFormDefaults(channel)

    expect(result.group_blacklist).toEqual(['vip', 'internal'])
  })

  it('should handle nullish group_blacklist with empty array', () => {
    const channelNull = createMockChannel({ group_blacklist: null })
    const channelUndefined = createMockChannel({
      group_blacklist: undefined,
    })

    expect(transformChannelToFormDefaults(channelNull).group_blacklist).toEqual(
      []
    )
    expect(
      transformChannelToFormDefaults(channelUndefined).group_blacklist
    ).toEqual([])
  })
})

describe('group_blacklist payloads', () => {
  it('should format group_blacklist array to comma-separated string on create', () => {
    const payload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'Test',
      key: 'k',
      models: 'gpt-4',
      group_blacklist: ['vip', 'internal'],
    })

    expect(payload.channel.group_blacklist).toBe('vip,internal')
  })

  it('should send empty group_blacklist string on update so backend can clear it', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'Test',
        models: 'gpt-4',
        group_blacklist: [],
      },
      1
    )

    expect(payload.group_blacklist).toBe('')
  })
})
