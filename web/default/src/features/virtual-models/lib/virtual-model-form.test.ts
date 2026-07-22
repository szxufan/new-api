import { describe, it, expect } from 'vitest'
import { VIRTUAL_MODEL_STATUS } from '../constants'
import type { VirtualModel } from '../types'
import {
  parseTargets,
  parseAggregator,
  transformFormDataToPayload,
  transformVirtualModelToFormDefaults,
  type VirtualModelFormValues,
} from './index'

describe('parseTargets', () => {
  it('parses a valid JSON array', () => {
    const targets = parseTargets(
      '[{"model":"gpt-4o","channel_id":1,"group":"vip"}]'
    )
    expect(targets).toEqual([{ model: 'gpt-4o', channel_id: 1, group: 'vip' }])
  })

  it('returns empty array for invalid or empty input', () => {
    expect(parseTargets('')).toEqual([])
    expect(parseTargets('not-json')).toEqual([])
    expect(parseTargets('{"model":"gpt-4o"}')).toEqual([])
  })
})

describe('parseAggregator', () => {
  it('parses a valid JSON object', () => {
    const aggregator = parseAggregator(
      '{"model":"gpt-4o-mini","prompt_template":"{{answers}}"}'
    )
    expect(aggregator).toEqual({
      model: 'gpt-4o-mini',
      prompt_template: '{{answers}}',
    })
  })

  it('returns empty object for invalid or empty input', () => {
    expect(parseAggregator('')).toEqual({})
    expect(parseAggregator('not-json')).toEqual({})
    expect(parseAggregator('[]')).toEqual({})
  })
})

describe('transformFormDataToPayload', () => {
  const formValues: VirtualModelFormValues = {
    name: '  my-virtual  ',
    mode: 'quality',
    targets: [
      { model: ' gpt-4o ', channel_id: 0, group: '' },
      { model: 'claude-sonnet', channel_id: 3, group: ' vip ' },
    ],
    aggregator_model: ' gpt-4o-mini ',
    aggregator_channel_id: 0,
    aggregator_group: '',
    aggregator_prompt_template: 'Answer: {{answers}}',
  }

  it('stringifies targets and aggregator and trims strings', () => {
    const payload = transformFormDataToPayload(formValues)

    expect(payload.name).toBe('my-virtual')
    expect(payload.mode).toBe('quality')
    expect(payload.status).toBe(VIRTUAL_MODEL_STATUS.ENABLED)
    expect(JSON.parse(payload.targets)).toEqual([
      { model: 'gpt-4o', channel_id: 0, group: '' },
      { model: 'claude-sonnet', channel_id: 3, group: 'vip' },
    ])
    expect(JSON.parse(payload.aggregator)).toEqual({
      model: 'gpt-4o-mini',
      channel_id: 0,
      group: '',
      prompt_template: 'Answer: {{answers}}',
    })
  })

  it('keeps the provided status', () => {
    const payload = transformFormDataToPayload(
      formValues,
      VIRTUAL_MODEL_STATUS.DISABLED
    )
    expect(payload.status).toBe(VIRTUAL_MODEL_STATUS.DISABLED)
  })
})

describe('transformVirtualModelToFormDefaults', () => {
  it('parses JSON string fields back into form values', () => {
    const virtualModel: VirtualModel = {
      id: 1,
      name: 'my-virtual',
      mode: 'speed',
      targets: '[{"model":"gpt-4o"},{"model":"claude-sonnet","channel_id":2}]',
      aggregator: '{}',
      status: VIRTUAL_MODEL_STATUS.ENABLED,
      created_time: 100,
      updated_time: 200,
    }

    const values = transformVirtualModelToFormDefaults(virtualModel)

    expect(values.name).toBe('my-virtual')
    expect(values.mode).toBe('speed')
    expect(values.targets).toEqual([
      { model: 'gpt-4o', channel_id: 0, group: '' },
      { model: 'claude-sonnet', channel_id: 2, group: '' },
    ])
    expect(values.aggregator_model).toBe('')
    expect(values.aggregator_channel_id).toBe(0)
  })
})
