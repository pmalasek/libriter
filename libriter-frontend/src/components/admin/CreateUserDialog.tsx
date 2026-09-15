import { useState } from 'react'
import { toast } from 'sonner'
import { useCreateUser } from '@/api/adminHooks'
import { ROLE_LABELS, type Role } from '@/api/types'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const MIN_PASSWORD = 8
const ROLES: Role[] = ['reader', 'editor', 'admin']

export function CreateUserDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-md">
        {/* Formulář vzniká až s otevřením dialogu, takže se sám vyprázdní. */}
        {open ? <CreateUserForm onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function CreateUserForm({ onDone }: { onDone: () => void }) {
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('reader')
  const [error, setError] = useState<string | null>(null)

  const createUser = useCreateUser()

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (password.length < MIN_PASSWORD) {
      setError(`Heslo musí mít alespoň ${MIN_PASSWORD} znaků.`)
      return
    }

    createUser.mutate(
      { display_name: displayName.trim(), email: email.trim(), password, role },
      {
        onSuccess: (user) => {
          toast.success(`Účet ${user.email} byl založen.`)
          onDone()
        },
        onError: (err) => setError(err.message),
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <DialogHeader>
        <DialogTitle>Nový uživatel</DialogTitle>
        <DialogDescription>
          Účet je použitelný hned; heslo si uživatel změní v profilu.
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor="new_user_name">Jméno</Label>
        <Input
          id="new_user_name"
          required
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="new_user_email">E-mail</Label>
        <Input
          id="new_user_email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="new_user_password">Heslo</Label>
        <Input
          id="new_user_password"
          type="password"
          autoComplete="new-password"
          required
          minLength={MIN_PASSWORD}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="new_user_role">Role</Label>
        <Select value={role} onValueChange={(value) => setRole(value as Role)}>
          <SelectTrigger id="new_user_role" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {ROLES.map((item) => (
              <SelectItem key={item} value={item}>
                {ROLE_LABELS[item]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone} disabled={createUser.isPending}>
          Zrušit
        </Button>
        <Button type="submit" disabled={createUser.isPending}>
          {createUser.isPending ? 'Zakládám…' : 'Založit účet'}
        </Button>
      </DialogFooter>
    </form>
  )
}
