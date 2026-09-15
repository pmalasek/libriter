import { useState } from 'react'
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
  const [password, setPassword] = useState('')
  const [passwordAgain, setPasswordAgain] = useState('')
  const [error, setError] = useState<string | null>(null)

  const resetPassword = useResetUserPassword()

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (password.length < MIN_PASSWORD) {
      setError(`Heslo musí mít alespoň ${MIN_PASSWORD} znaků.`)
      return
    }
    if (password !== passwordAgain) {
      setError('Hesla se neshodují.')
      return
    }

    resetPassword.mutate(
      { userId: user.id, password },
      {
        onSuccess: () => {
          toast.success(`Heslo účtu ${user.email} bylo změněno.`)
          onDone()
        },
        onError: (err) => setError(err.message),
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <DialogHeader>
        <DialogTitle>Reset hesla</DialogTitle>
        <DialogDescription>
          Nové heslo pro účet {user.email}. Předejte ho uživateli bezpečnou cestou.
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor="reset_password">Nové heslo</Label>
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
        <Label htmlFor="reset_password_again">Nové heslo znovu</Label>
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
          Zrušit
        </Button>
        <Button type="submit" disabled={resetPassword.isPending}>
          {resetPassword.isPending ? 'Měním heslo…' : 'Změnit heslo'}
        </Button>
      </DialogFooter>
    </form>
  )
}
