import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RateLimitDialog } from './rate-limit-dialog'
import * as api from '../../api'
import type { Channel } from '../../types'
import * as provider from '../channels-provider'

vi.mock('../channels-provider', () => ({
  useChannels: vi.fn(),
}))

vi.mock('../../api', () => ({
  rateLimitChannel: vi.fn(),
  unrateLimitChannel: vi.fn(),
}))

const mockToast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast: mockToast }))

describe('RateLimitDialog', () => {
  const queryClient = new QueryClient()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  const renderDialog = (status = 1) => {
    vi.mocked(provider.useChannels).mockReturnValue({
      currentRow: {
        id: 1,
        name: 'Test Channel',
        status,
      } as Channel,
    } as unknown as ReturnType<typeof provider.useChannels>)

    return render(
      <QueryClientProvider client={queryClient}>
        <RateLimitDialog open={true} onOpenChange={vi.fn()} />
      </QueryClientProvider>
    )
  }

  describe('Set Rate Limit mode (status not rate limited)', () => {
    it('should render set rate limit form', () => {
      renderDialog(1)
      expect(
        screen.getByRole('heading', { name: 'Set Rate Limit' })
      ).toBeInTheDocument()
      expect(screen.getByLabelText(/Rate Limit Duration/i)).toBeInTheDocument()
      expect(screen.getByLabelText(/Rate Limit Reason/i)).toBeInTheDocument()
    })

    it('should call rateLimitChannel with correct params on submit', async () => {
      vi.mocked(api.rateLimitChannel).mockResolvedValue({ success: true })
      renderDialog(1)

      const durationInput = screen.getByLabelText(/Rate Limit Duration/i)
      fireEvent.change(durationInput, { target: { value: '2.5' } })

      const reasonInput = screen.getByLabelText(/Rate Limit Reason/i)
      fireEvent.change(reasonInput, { target: { value: 'Test reason' } })

      const submitButton = screen.getByRole('button', { name: /Set Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(api.rateLimitChannel).toHaveBeenCalledWith(1, {
          duration_hours: 2.5,
          reason: 'Test reason',
        })
      })
    })

    it('should show error for invalid duration', async () => {
      renderDialog(1)

      const durationInput = screen.getByLabelText(/Rate Limit Duration/i)
      fireEvent.change(durationInput, { target: { value: '-1' } })

      const submitButton = screen.getByRole('button', { name: /Set Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Please enter a valid duration')
      })
      expect(api.rateLimitChannel).not.toHaveBeenCalled()
    })

    it('should show error for zero duration', async () => {
      renderDialog(1)

      const durationInput = screen.getByLabelText(/Rate Limit Duration/i)
      fireEvent.change(durationInput, { target: { value: '0' } })

      const submitButton = screen.getByRole('button', { name: /Set Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Please enter a valid duration')
      })
    })

    it('should handle API failure', async () => {
      vi.mocked(api.rateLimitChannel).mockResolvedValue({
        success: false,
        message: 'Server error',
      })
      renderDialog(1)

      const submitButton = screen.getByRole('button', { name: /Set Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Server error')
      })
    })

    it('should handle network error', async () => {
      vi.mocked(api.rateLimitChannel).mockRejectedValue(new Error('Network failed'))
      renderDialog(1)

      const submitButton = screen.getByRole('button', { name: /Set Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Network failed')
      })
    })
  })

  describe('Remove Rate Limit mode (status rate limited)', () => {
    it('should render remove rate limit form for status 4', () => {
      renderDialog(4)
      expect(
        screen.getByRole('heading', { name: 'Remove Rate Limit' })
      ).toBeInTheDocument()
      expect(screen.getByText(/This channel is currently rate limited/i)).toBeInTheDocument()
    })

    it('should render remove rate limit form for status 5', () => {
      renderDialog(5)
      expect(
        screen.getByRole('heading', { name: 'Remove Rate Limit' })
      ).toBeInTheDocument()
    })

    it('should call unrateLimitChannel on submit', async () => {
      vi.mocked(api.unrateLimitChannel).mockResolvedValue({ success: true })
      renderDialog(4)

      const submitButton = screen.getByRole('button', { name: /Remove Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(api.unrateLimitChannel).toHaveBeenCalledWith(1)
      })
      expect(mockToast.success).toHaveBeenCalledWith('Channel rate limit removed')
    })

    it('should handle remove rate limit API failure', async () => {
      vi.mocked(api.unrateLimitChannel).mockResolvedValue({
        success: false,
        message: 'Cannot remove',
      })
      renderDialog(4)

      const submitButton = screen.getByRole('button', { name: /Remove Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Cannot remove')
      })
    })

    it('should handle remove rate limit network error', async () => {
      vi.mocked(api.unrateLimitChannel).mockRejectedValue(new Error('Connection lost'))
      renderDialog(4)

      const submitButton = screen.getByRole('button', { name: /Remove Rate Limit/i })
      fireEvent.click(submitButton)

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith('Connection lost')
      })
    })
  })

  describe('Cancel button', () => {
    it('should call onOpenChange with false when cancel clicked', () => {
      const onOpenChange = vi.fn()
      vi.mocked(provider.useChannels).mockReturnValue({
        currentRow: { id: 1, name: 'Test', status: 1 } as Channel,
      } as unknown as ReturnType<typeof provider.useChannels>)

      render(
        <QueryClientProvider client={queryClient}>
          <RateLimitDialog open={true} onOpenChange={onOpenChange} />
        </QueryClientProvider>
      )

      const cancelButton = screen.getByRole('button', { name: /Cancel/i })
      fireEvent.click(cancelButton)

      expect(onOpenChange).toHaveBeenCalledWith(false)
    })
  })
})
