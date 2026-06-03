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
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2, Timer, TimerOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { rateLimitChannel, unrateLimitChannel } from '../../api'
import { channelsQueryKeys } from '../../lib'
import { useChannels } from '../channels-provider'

type RateLimitDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function RateLimitDialog({ open, onOpenChange }: RateLimitDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const queryClient = useQueryClient()
  const [durationHours, setDurationHours] = useState('1')
  const [reason, setReason] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!currentRow) return null

  const isRateLimited = currentRow.status === 4 || currentRow.status === 5

  const handleRateLimit = async () => {
    const hours = parseFloat(durationHours)
    if (isNaN(hours) || hours <= 0) {
      toast.error(t('Please enter a valid duration'))
      return
    }

    setIsSubmitting(true)
    try {
      const res = await rateLimitChannel(currentRow.id, {
        duration_hours: hours,
        reason: reason.trim() || undefined,
      })
      if (res.success) {
        toast.success(t('Channel has been rate limited'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        onOpenChange(false)
        setDurationHours('1')
        setReason('')
      } else {
        toast.error(res.message || t('Failed to rate limit channel'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to rate limit channel')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleUnrateLimit = async () => {
    setIsSubmitting(true)
    try {
      const res = await unrateLimitChannel(currentRow.id)
      if (res.success) {
        toast.success(t('Channel rate limit removed'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        onOpenChange(false)
      } else {
        toast.error(res.message || t('Failed to remove rate limit'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to remove rate limit')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleClose = () => {
    onOpenChange(false)
    setDurationHours('1')
    setReason('')
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isRateLimited ? t('Remove Rate Limit') : t('Set Rate Limit')}
          </DialogTitle>
          <DialogDescription>
            {isRateLimited
              ? t('Remove rate limit for:')
              : t('Set rate limit for:')}{' '}
            <strong>{currentRow.name}</strong>
          </DialogDescription>
        </DialogHeader>

        {isRateLimited ? (
          <div className='space-y-4 py-4'>
            <div className='bg-muted/50 rounded-lg border p-4'>
              <div className='flex items-center gap-2 text-sm'>
                <TimerOff className='h-4 w-4 text-amber-500' />
                <span>{t('This channel is currently rate limited.')}</span>
              </div>
            </div>
          </div>
        ) : (
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label htmlFor='duration'>
                {t('Rate Limit Duration (hours)')}
              </Label>
              <Input
                id='duration'
                type='number'
                step='0.5'
                min='0.5'
                value={durationHours}
                onChange={(e) => setDurationHours(e.target.value)}
                disabled={isSubmitting}
                placeholder={t('Enter duration in hours')}
              />
              <p className='text-muted-foreground text-xs'>
                {t('Minimum 0.5 hours')}
              </p>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='reason'>{t('Rate Limit Reason')}</Label>
              <Textarea
                id='reason'
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                disabled={isSubmitting}
                placeholder={t('Optional: enter reason for rate limiting')}
                rows={2}
              />
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={handleClose} disabled={isSubmitting}>
            {t('Cancel')}
          </Button>
          {isRateLimited ? (
            <Button onClick={handleUnrateLimit} disabled={isSubmitting} variant='default'>
              {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('Remove Rate Limit')}
            </Button>
          ) : (
            <Button onClick={handleRateLimit} disabled={isSubmitting}>
              {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              <Timer className='mr-2 h-4 w-4' />
              {t('Set Rate Limit')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
