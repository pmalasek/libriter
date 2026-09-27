import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { useAuthConfig, useRegister } from '@/api/hooks'
import { currentLanguage, loginHint, LOGIN_PATTERN, roleLabel } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { AuthShell } from '@/components/layout/AuthShell'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const MIN_PASSWORD = 8

export function RegisterPage() {
  const { t } = useTranslation()
  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [login, setLogin] = useState('')
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
      setLocalError(t('auth.register.passwordTooShort', { min: MIN_PASSWORD }))
      return
    }

    register.mutate(
      {
        display_name: displayName.trim(),
        email: email.trim(),
        login: login.trim(),
        password,
        ui_language: currentLanguage(),
      },
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
    <AuthShell>
      <Card surface="strong" className="rounded-3xl shadow-glass-lg [--card-spacing:--spacing(6)]">
        <CardHeader>
          <CardTitle className="text-2xl">{t('auth.register.title')}</CardTitle>
          <CardDescription>
            {disabled
              ? t('auth.register.disabledDescription')
              : t('auth.register.defaultRole', {
                  role: defaultRole
                    ? roleLabel(defaultRole).toLowerCase()
                    : t('auth.register.defaultRoleFallback'),
                })}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {disabled ? (
            <p className="text-sm text-muted-foreground">
              {t('auth.register.disabled')}
            </p>
          ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="display_name">{t('auth.register.name')}</Label>
              <Input
                id="display_name"
                autoComplete="name"
                required
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">{t('auth.register.email')}</Label>
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
              <Label htmlFor="login">{t('auth.register.login')}</Label>
              <Input
                id="login"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                pattern={LOGIN_PATTERN}
                value={login}
                onChange={(e) => setLogin(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">{loginHint()}</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">{t('auth.register.password')}</Label>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                required
                minLength={MIN_PASSWORD}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">{t('auth.register.passwordHint', { min: MIN_PASSWORD })}</p>
            </div>

            {error ? <p className="text-sm text-destructive">{error}</p> : null}

            <Button type="submit" size="lg" className="w-full" disabled={register.isPending}>
              {register.isPending ? t('auth.register.submitting') : t('auth.register.submit')}
            </Button>
          </form>
          )}

          <p className="mt-4 text-center text-sm text-muted-foreground">
            {t('auth.register.haveAccount')}{' '}
            <Link to="/login" className="font-medium text-primary underline-offset-4 hover:underline">
              {t('auth.register.loginLink')}
            </Link>
          </p>
        </CardContent>
      </Card>
    </AuthShell>
  )
}
