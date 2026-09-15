import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { useAuthConfig, useRegister } from '@/api/hooks'
import { ROLE_LABELS } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { Logo } from '@/components/layout/Logo'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const MIN_PASSWORD = 8

export function RegisterPage() {
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const register = useRegister()
  const authConfig = useAuthConfig()

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setLocalError(null)

    if (password.length < MIN_PASSWORD) {
      setLocalError(`Heslo musí mít alespoň ${MIN_PASSWORD} znaků.`)
      return
    }

    register.mutate(
      { display_name: displayName.trim(), email: email.trim(), password },
      {
        onSuccess: (response) => {
          signIn(response)
          navigate('/', { replace: true })
        },
      },
    )
  }

  const error = localError ?? register.error?.message ?? null
  const disabled = authConfig.data?.registration_enabled === false
  const defaultRole = authConfig.data?.default_role

  return (
    <div className="flex min-h-svh items-center justify-center bg-muted/30 p-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex justify-center">
          <Logo className="h-9" />
        </div>
        <Card>
          <CardHeader>
            <CardTitle>Registrace</CardTitle>
            <CardDescription>
              {disabled
                ? 'Nové účty zakládá administrátor.'
                : `Nový účet získá roli ${defaultRole ? ROLE_LABELS[defaultRole].toLowerCase() : 'čtenáře'}.`}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {disabled ? (
              <p className="text-sm text-muted-foreground">
                Registrace nových účtů je vypnutá. Účet vám založí administrátor.
              </p>
            ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="display_name">Jméno</Label>
                <Input
                  id="display_name"
                  autoComplete="name"
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
                  autoComplete="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">Heslo</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={MIN_PASSWORD}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">Minimálně {MIN_PASSWORD} znaků.</p>
              </div>

              {error ? <p className="text-sm text-destructive">{error}</p> : null}

              <Button type="submit" size="lg" className="w-full" disabled={register.isPending}>
                {register.isPending ? 'Zakládám účet…' : 'Vytvořit účet'}
              </Button>
            </form>
            )}

            <p className="mt-4 text-center text-sm text-muted-foreground">
              Už máte účet?{' '}
              <Link to="/login" className="font-medium text-foreground underline-offset-4 hover:underline">
                Přihlaste se
              </Link>
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
