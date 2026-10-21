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
import { RefreshCw, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
import { CopyButton } from '@/components/copy-button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useAccessToken } from '../../hooks'

// ============================================================================
// Access Token Dialog Component
// ============================================================================

interface AccessTokenDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AccessTokenDialog({
  open,
  onOpenChange,
}: AccessTokenDialogProps) {
  const { t } = useTranslation()
  const { token, loading, regenerating, load, regenerate } = useAccessToken()
  const [confirmOpen, setConfirmOpen] = useState(false)

  // Load the current token when dialog opens; never regenerates
  useEffect(() => {
    if (open && !token) {
      load()
    }
  }, [open, token, load])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Access Token')}</DialogTitle>
          <DialogDescription>
            {t(
              "Your system access token for API authentication. Keep it secure and don't share it with others."
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='my-6 space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='token'>{t('Token')}</Label>
            <div className='flex gap-2'>
              <Input
                id='token'
                type='text'
                value={token}
                readOnly
                className='font-mono text-xs'
                placeholder={loading ? t('Loading...') : ''}
              />
              <CopyButton
                value={token}
                variant='outline'
                className='size-9'
                iconClassName='size-4'
                tooltip={t('Copy token')}
                aria-label={t('Copy token')}
              />
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('Use this token for API authentication')}
            </p>
          </div>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Close')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            onClick={() => setConfirmOpen(true)}
            disabled={regenerating || loading}
            className='gap-2'
          >
            {regenerating ? (
              <Loader2 className='h-4 w-4 animate-spin' />
            ) : (
              <RefreshCw className='h-4 w-4' />
            )}
            {regenerating ? t('Generating...') : t('Regenerate')}
          </Button>
        </DialogFooter>
      </DialogContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Regenerate access token?')}
        desc={t(
          'The current access token will be invalidated immediately. Clients or scripts using it must be updated to the new token.'
        )}
        destructive
        confirmText={t('Regenerate')}
        isLoading={regenerating}
        handleConfirm={() => regenerate()}
      />
    </Dialog>
  )
}
