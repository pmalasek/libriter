import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCreateUser } from '@/api/adminHooks'
import { loginHint, LOGIN_PATTERN, roleLabel, type Role } from '@/api/types'
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
  const { t } = useTranslation()
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('reader')
  const [error, setError] = useState<string | null>(null)

  const createUser = useCreateUser()

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (password.length < MIN_PASSWORD) {
      setError(t('admin.users.passwordTooShort', { min: MIN_PASSWORD }))
      return
    }

    createUser.mutate(
      { display_name: displayName.trim(), email: email.trim(), login: login.trim(), password, role },
      {
        onSuccess: (user) => {
          toast.success(t('admin.users.create.created', { email: user.email }))
          onDone()
        },
        onError: (err) => setError(err.message),
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <DialogHeader>
        <DialogTitle>{t('admin.users.create.title')}</DialogTitle>
        <DialogDescription>{t('admin.users.create.description')}</DialogDescription>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor="new_user_name">{t('admin.users.fields.name')}</Label>
        <Input
          id="new_user_name"
          required
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="new_user_email">{t('admin.users.fields.email')}</Label>
        <Input
          id="new_user_email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="new_user_login">{t('admin.users.fields.login')}</Label>
        <Input
          id="new_user_login"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          pattern={LOGIN_PATTERN}
          value={login}
          onChange={(e) => setLogin(e.target.value)}
        />
        <p className="text-xs text-muted-foreground">{loginHint()}</p>
      </div>

      <div className="space-y-2">
        <Label htmlFor="new_user_password">{t('admin.users.fields.password')}</Label>
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
        <Label htmlFor="new_user_role">{t('admin.users.fields.role')}</Label>
        <Select value={role} onValueChange={(value) => setRole(value as Role)}>
          <SelectTrigger id="new_user_role" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {ROLES.map((item) => (
              <SelectItem key={item} value={item}>
                {roleLabel(item)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone} disabled={createUser.isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={createUser.isPending}>
          {createUser.isPending
            ? t('admin.users.create.submitting')
            : t('admin.users.create.submit')}
        </Button>
      </DialogFooter>
    </form>
  )
}
