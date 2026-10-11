import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DetailsDialog } from './details-dialog'
import type { UsageLog } from '../../data/schema'

function makeConsumeLog(other: Record<string, unknown>): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 0,
    type: 2,
    content: '',
    username: 'test-user',
    token_name: 'sk-test',
    model_name: 'gpt-4',
    quota: 1000,
    prompt_tokens: 100,
    completion_tokens: 50,
    use_time: 1,
    is_stream: false,
    channel: 1,
    channel_name: 'test-channel',
    token_id: 1,
    group: 'default',
    other: JSON.stringify(other),
  } as unknown as UsageLog
}

function renderDialog(other: Record<string, unknown>) {
  return render(
    <DetailsDialog
      log={makeConsumeLog(other)}
      isAdmin={true}
      open={true}
      onOpenChange={vi.fn()}
    />
  )
}

describe('DetailsDialog billing breakdown with channel price factors', () => {
  it('shows channel price factor rows when other.channel_price_factors is present', () => {
    renderDialog({
      model_ratio: 1,
      completion_ratio: 2,
      group_ratio: 1,
      channel_price_factors: {
        total: 0.9,
        time_window: 0.8,
        input: 0.5,
      },
    })

    expect(screen.getByText('Billing Details')).toBeInTheDocument()
    expect(screen.getByText('Total Factor')).toBeInTheDocument()
    expect(screen.getByText('0.9000x')).toBeInTheDocument()
    expect(screen.getByText('Time Window Factor')).toBeInTheDocument()
    expect(screen.getByText('0.8000x')).toBeInTheDocument()
    expect(screen.getByText('Input Factor')).toBeInTheDocument()
    expect(screen.getByText('0.5000x')).toBeInTheDocument()
  })

  it('hides channel price factor rows when absent (legacy logs)', () => {
    renderDialog({
      model_ratio: 1,
      completion_ratio: 2,
      group_ratio: 1,
    })

    expect(screen.getByText('Billing Details')).toBeInTheDocument()
    expect(screen.queryByText('Total Factor')).toBeNull()
    expect(screen.queryByText('Input Factor')).toBeNull()
  })
})