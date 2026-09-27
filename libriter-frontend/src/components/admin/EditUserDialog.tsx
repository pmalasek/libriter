import { useState } from 'react'
import { toast } from 'sonner'
import { useUpdateUser } from '@/api/adminHooks'
import { LOGIN_HINT, LOGIN_PATTERN, type User } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
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

export function EditUserDialog({
  user,
  onClose,
}: {
  /** null = dialog je zavřený. */
  user: User | null
  onClose: () => void
}) {
  return (
    <Dialog open={user !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-md">
        {/* Formulář vzniká s otevřením dialogu, takže začíná s aktuálními údaji. */}
        {user ? <EditUserForm user={user} onDone={onClose} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function EditUserForm({ user, onDone }: { user: User; onDone: () => void }) {
  const [displayName, setDisplayName] = useState(user.display_name)
  const [email, setEmail] = useState(user.email)
  const [login, setLogin] = useState(user.login)
  const [error, setError] = useState<string | null>(null)

  const { user: currentUser, updateUser } = useAuth()
  const update = useUpdateUser()

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    update.mutate(
      { userId: user.id, display_name: displayName.trim(), email: email.trim(), login: login.trim() },
      {
        onSuccess: (updated) => {
          // Úprava vlastního účtu se musí propsat i do přihlášené session.
          if (updated.id === currentUser?.id) updateUser(updated)
          toast.success(`Účet ${updated.email} byl uložen.`)
          onDone()
        },
        onError: (err) => setError(err.message),
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <DialogHeader>
        <DialogTitle>Upravit uživatele</DialogTitle>
        <DialogDescription>Uživatel se může přihlásit e-mailem i loginem.</DialogDescription>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor="edit_user_name">Jméno</Label>
        <Input
          id="edit_user_name"
          required
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="edit_user_email">E-mail</Label>
        <Input
          id="edit_user_email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="edit_user_login">Login</Label>
        <Input
          id="edit_user_login"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          pattern={LOGIN_PATTERN}
          value={login}
          onChange={(e) => setLogin(e.target.value)}
        />
        <p className="text-xs text-muted-foreground">{LOGIN_HINT}</p>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone} disabled={update.isPending}>
          Zrušit
        </Button>
        <Button type="submit" disabled={update.isPending}>
          {update.isPending ? 'Ukládám…' : 'Uložit'}
        </Button>
      </DialogFooter>
    </form>
  )
}
