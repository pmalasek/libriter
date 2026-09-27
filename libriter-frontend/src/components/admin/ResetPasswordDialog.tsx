import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useResetUserPassword } from '@/api/adminHooks'
import type { User } from '@/api/types'
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

const MIN_PASSWORD = 8

export function ResetPasswordDialog({
  user,
  onClose,
}: {
  /** null = dialog je zavřený. */
  user: User | null
  onClose: () => void
}) {
  return (
    <Dialog open={user !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        {user ? <ResetPasswordForm user={user} onDone={onClose} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function ResetPasswordForm({ user, onDone }: { user: User; onDone: () => void }) {
  const { t } = useTranslation()
  const [password, setPassword] = useState('')
  const [passwordAgain, setPasswordAgain] = useState('')
  const [error, setError] = useState<string | null>(null)

  const resetPassword = useResetUserPassword()

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (password.length < MIN_PASSWORD) {
      setError(t('admin.users.passwordTooShort', { min: MIN_PASSWORD }))
      return
    }
    if (password !== passwordAgain) {
      setError(t('admin.users.resetPassword.mismatch'))
      return
    }

    resetPassword.mutate(
      { userId: user.id, password },
      {
        onSuccess: () => {
          toast.success(t('admin.users.resetPassword.changed', { email: user.email }))
          onDone()
        },
        onError: (err) => setError(err.message),
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <DialogHeader>
        <DialogTitle>{t('admin.users.resetPassword.title')}</DialogTitle>
        <DialogDescription>
          {t('admin.users.resetPassword.description', { email: user.email })}
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor="reset_password">{t('admin.users.resetPassword.newPassword')}</Label>
        <Input
          id="reset_password"
          type="password"
          autoComplete="new-password"
          required
          minLength={MIN_PASSWORD}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="reset_password_again">{t('admin.users.resetPassword.newPasswordAgain')}</Label>
        <Input
          id="reset_password_again"
          type="password"
          autoComplete="new-password"
          required
          value={passwordAgain}
          onChange={(e) => setPasswordAgain(e.target.value)}
        />
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone} disabled={resetPassword.isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={resetPassword.isPending}>
          {resetPassword.isPending
            ? t('admin.users.resetPassword.submitting')
            : t('admin.users.resetPassword.submit')}
        </Button>
      </DialogFooter>
    </form>
  )
}
