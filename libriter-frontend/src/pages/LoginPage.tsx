import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useNavigate } from 'react-router'
import { useAuthConfig, useLogin } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { AuthShell } from '@/components/layout/AuthShell'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export function LoginPage() {
  const { t } = useTranslation()
  const [identifier, setIdentifier] = useState('')
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
      { login: identifier.trim(), password },
      {
        onSuccess: (response) => {
          signIn(response)
          navigate(from, { replace: true })
        },
      },
    )
  }

  return (
    <AuthShell>
      <Card surface="strong" className="rounded-3xl shadow-glass-lg [--card-spacing:--spacing(6)]">
        <CardHeader>
          <CardTitle className="text-2xl">{t('auth.login.title')}</CardTitle>
          <CardDescription>{t('auth.login.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="login">{t('auth.login.identifier')}</Label>
              <Input
                id="login"
                type="text"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                required
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">{t('auth.login.password')}</Label>
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
              {login.isPending ? t('auth.login.submitting') : t('auth.login.submit')}
            </Button>
          </form>

          {/* Odkaz se ukáže, až je jasné, že registrace běží – jinak by
              při vypnuté registraci blikl a zmizel. */}
          {authConfig.data?.registration_enabled ? (
            <p className="mt-4 text-center text-sm text-muted-foreground">
              {t('auth.login.noAccount')}{' '}
              <Link to="/register" className="font-medium text-primary underline-offset-4 hover:underline">
                {t('auth.login.registerLink')}
              </Link>
            </p>
          ) : null}
        </CardContent>
      </Card>
    </AuthShell>
  )
}
