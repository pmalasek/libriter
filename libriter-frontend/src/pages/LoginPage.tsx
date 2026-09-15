import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { useAuthConfig, useLogin } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { Logo } from '@/components/layout/Logo'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const login = useLogin()
  const authConfig = useAuthConfig()

  const from = (location.state as { from?: string } | null)?.from ?? '/'

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    login.mutate(
      { email: email.trim(), password },
      {
        onSuccess: (response) => {
          signIn(response)
          navigate(from, { replace: true })
        },
      },
    )
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-muted/30 p-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex justify-center">
          <Logo className="h-9" />
        </div>
        <Card>
          <CardHeader>
            <CardTitle>Přihlášení</CardTitle>
            <CardDescription>Zadejte své přihlašovací údaje.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
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
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>

              {login.error ? (
                <p className="text-sm text-destructive">{login.error.message}</p>
              ) : null}

              <Button type="submit" size="lg" className="w-full" disabled={login.isPending}>
                {login.isPending ? 'Přihlašuji…' : 'Přihlásit se'}
              </Button>
            </form>

            {/* Odkaz se ukáže, až je jasné, že registrace běží – jinak by
                při vypnuté registraci blikl a zmizel. */}
            {authConfig.data?.registration_enabled ? (
              <p className="mt-4 text-center text-sm text-muted-foreground">
                Nemáte účet?{' '}
                <Link to="/register" className="font-medium text-foreground underline-offset-4 hover:underline">
                  Zaregistrujte se
                </Link>
              </p>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
