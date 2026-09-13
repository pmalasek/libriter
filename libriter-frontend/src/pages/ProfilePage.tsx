import { useState } from 'react'
import { toast } from 'sonner'
import { useChangePassword, useUpdateProfile } from '@/api/hooks'
import { ROLE_LABELS } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatDate } from '@/lib/format'

const MIN_PASSWORD = 8

export function ProfilePage() {
  const { user, updateUser } = useAuth()

  // ProfilePage se renderuje jen přihlášenému uživateli, takže stačí inicializace.
  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [password, setPassword] = useState('')
  const [passwordAgain, setPasswordAgain] = useState('')
  const [passwordError, setPasswordError] = useState<string | null>(null)

  const updateProfile = useUpdateProfile(user?.id ?? '')
  const changePassword = useChangePassword(user?.id ?? '')

  if (!user) return null

  function handleProfileSubmit(event: React.FormEvent) {
    event.preventDefault()
    updateProfile.mutate(
      { display_name: displayName.trim(), email: email.trim() },
      {
        onSuccess: (updated) => {
          // Server hodnoty normalizuje (trim), proto formulář přepíšeme jeho odpovědí.
          updateUser(updated)
          setDisplayName(updated.display_name)
          setEmail(updated.email)
          toast.success('Profil byl uložen.')
        },
        onError: (error) => toast.error(error.message),
      },
    )
  }

  function handlePasswordSubmit(event: React.FormEvent) {
    event.preventDefault()
    setPasswordError(null)

    if (password.length < MIN_PASSWORD) {
      setPasswordError(`Heslo musí mít alespoň ${MIN_PASSWORD} znaků.`)
      return
    }
    if (password !== passwordAgain) {
      setPasswordError('Hesla se neshodují.')
      return
    }

    changePassword.mutate(
      { password },
      {
        onSuccess: () => {
          setPassword('')
          setPasswordAgain('')
          toast.success('Heslo bylo změněno.')
        },
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="max-w-2xl">
      <PageHeader
        title="Profil"
        description={`Účet vytvořen ${formatDate(user.created_at)}`}
        actions={<Badge variant="secondary">{ROLE_LABELS[user.role] ?? user.role}</Badge>}
      />

      <Card>
        <CardHeader>
          <CardTitle>Osobní údaje</CardTitle>
          <CardDescription>Změna zobrazovaného jména a e-mailu.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleProfileSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="display_name">Jméno</Label>
              <Input
                id="display_name"
                required
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">E-mail</Label>
              <Input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={updateProfile.isPending}>
              {updateProfile.isPending ? 'Ukládám…' : 'Uložit změny'}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Změna hesla</CardTitle>
          <CardDescription>Minimálně {MIN_PASSWORD} znaků.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handlePasswordSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="new_password">Nové heslo</Label>
              <Input
                id="new_password"
                type="password"
                autoComplete="new-password"
                required
                minLength={MIN_PASSWORD}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new_password_again">Nové heslo znovu</Label>
              <Input
                id="new_password_again"
                type="password"
                autoComplete="new-password"
                required
                value={passwordAgain}
                onChange={(e) => setPasswordAgain(e.target.value)}
              />
            </div>

            {passwordError ? <p className="text-sm text-destructive">{passwordError}</p> : null}

            <Button type="submit" disabled={changePassword.isPending}>
              {changePassword.isPending ? 'Měním heslo…' : 'Změnit heslo'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
