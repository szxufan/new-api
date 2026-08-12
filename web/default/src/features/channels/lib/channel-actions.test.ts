import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('i18next', () => ({
  default: {
    t: (key: string) => key,
  },
}))

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    testChannel: vi.fn(),
  }
})

import { toast } from 'sonner'
import { testChannel } from '../api'
import { handleTestChannel } from './channel-actions'
import type { ChannelTestResponse } from '../types'

const mockedTestChannel = vi.mocked(testChannel)

const mockSuccessResponse = (overrides: Partial<ChannelTestResponse> = {}): ChannelTestResponse => ({
  success: true,
  message: '',
  time: 1.25, // 后端单位为秒
  ...overrides,
})

describe('handleTestChannel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should convert backend time (seconds) to milliseconds in callback', async () => {
    mockedTestChannel.mockResolvedValue(mockSuccessResponse({ time: 1.25 }))
    const onTestComplete = vi.fn()

    await handleTestChannel(1, undefined, onTestComplete)

    expect(onTestComplete).toHaveBeenCalledWith(true, 1250, undefined, undefined, undefined)
  })

  it('should pass through fingerprint in callback', async () => {
    mockedTestChannel.mockResolvedValue(
      mockSuccessResponse({ time: 0.5, fingerprint: 'abc123' })
    )
    const onTestComplete = vi.fn()

    await handleTestChannel(1, undefined, onTestComplete)

    expect(onTestComplete).toHaveBeenCalledWith(true, 500, undefined, undefined, 'abc123')
  })

  it('should call testChannel without payload when no options given', async () => {
    mockedTestChannel.mockResolvedValue(mockSuccessResponse())
    const onTestComplete = vi.fn()

    await handleTestChannel(1, undefined, onTestComplete)

    expect(mockedTestChannel).toHaveBeenCalledWith(1, undefined)
    expect(toast.success).toHaveBeenCalled()
  })

  it('should forward test model, endpoint type and stream options as payload', async () => {
    mockedTestChannel.mockResolvedValue(mockSuccessResponse())
    const onTestComplete = vi.fn()

    await handleTestChannel(
      1,
      { testModel: 'gpt-4o', endpointType: 'openai', stream: true },
      onTestComplete
    )

    expect(mockedTestChannel).toHaveBeenCalledWith(1, {
      model: 'gpt-4o',
      endpoint_type: 'openai',
      stream: true,
    })
  })

  it('should report failure with message and error code', async () => {
    mockedTestChannel.mockResolvedValue({
      success: false,
      message: 'upstream error',
      error_code: 'bad_response',
    })
    const onTestComplete = vi.fn()

    await handleTestChannel(1, undefined, onTestComplete)

    expect(toast.error).toHaveBeenCalled()
    expect(onTestComplete).toHaveBeenCalledWith(false, undefined, 'upstream error', 'bad_response')
  })

  it('should report failure when API throws', async () => {
    mockedTestChannel.mockRejectedValue({
      response: { data: { message: 'network error' } },
    })
    const onTestComplete = vi.fn()

    await handleTestChannel(1, undefined, onTestComplete)

    expect(onTestComplete).toHaveBeenCalledWith(false, undefined, 'network error')
  })
})
