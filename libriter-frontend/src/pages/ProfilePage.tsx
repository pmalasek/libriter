import { useState } from 'react'
import { toast } from 'sonner'
import { useChangePassword, useUpdateProfile } from '@/api/hooks'
import { loginHint, LOGIN_PATTERN, roleLabel } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { PageHeader } from '@/components/PageHeader'
import { useTranslation } from 'react-i18next'
import { AppearanceSettings } from '@/components/layout/AppearanceSettings'
import { UiLanguageSelect } from '@/components/layout/UiLanguageSelect'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatDate } from '@/lib/format'

const MIN_PASSWORD = 8

export function ProfilePage() {
  const { t } = useTranslation()
  const { user, updateUser } = useAuth()

  // ProfilePage se renderuje jen přihlášenému uživateli, takže stačí inicializace.
  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [login, setLogin] = useState(user?.login ?? '')
  const [password, setPassword] = useState('')
  const [passwordAgain, setPasswordAgain] = useState('')
  const [passwordError, setPasswordError] = useState<string | null>(null)

  const updateProfile = useUpdateProfile(user?.id ?? '')
  const changePassword = useChangePassword(user?.id ?? '')

  if (!user) return null

  function handleProfileSubmit(event: React.FormEvent) {
    event.preventDefault()
    updateProfile.mutate(
      { display_name: displayName.trim(), email: email.trim(), login: login.trim() },
      {
        onSuccess: (updated) => {
          // Server hodnoty normalizuje (trim), proto formulář přepíšeme jeho odpovědí.
          updateUser(updated)
          setDisplayName(updated.display_name)
          setEmail(updated.email)
          setLogin(updated.login)
          toast.success(t('profile.saved'))
        },
        onError: (error) => toast.error(error.message),
      },
    )
  }

  function handlePasswordSubmit(event: React.FormEvent) {
    event.preventDefault()
    setPasswordError(null)

    if (password.length < MIN_PASSWORD) {
      setPasswordError(t('profile.password.tooShort', { min: MIN_PASSWORD }))
      return
    }
    if (password !== passwordAgain) {
      setPasswordError(t('profile.password.mismatch'))
      return
    }

    changePassword.mutate(
      { password },
      {
        onSuccess: () => {
          setPassword('')
          setPasswordAgain('')
          toast.success(t('profile.password.changed'))
        },
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="max-w-2xl">
      <PageHeader
        title={t('profile.title')}
        description={t('profile.created', { date: formatDate(user.created_at) })}
        actions={<Badge variant="secondary">{roleLabel(user.role)}</Badge>}
      />

      <Card>
        <CardHeader>
          <CardTitle>{t('profile.personal.title')}</CardTitle>
          <CardDescription>{t('profile.personal.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleProfileSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="display_name">{t('profile.personal.name')}</Label>
              <Input
                id="display_name"
                required
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">{t('profile.personal.email')}</Label>
              <Input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="login">{t('profile.personal.login')}</Label>
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
            <Button type="submit" disabled={updateProfile.isPending}>
              {updateProfile.isPending ? t('common.saving') : t('profile.personal.submit')}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>{t('profile.appearance.title')}</CardTitle>
          <CardDescription>{t('profile.appearance.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <AppearanceSettings />
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>{t('language.title')}</CardTitle>
          <CardDescription>{t('language.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <UiLanguageSelect />
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>{t('profile.password.title')}</CardTitle>
          <CardDescription>{t('profile.password.hint', { min: MIN_PASSWORD })}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handlePasswordSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="new_password">{t('profile.password.new')}</Label>
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
              <Label htmlFor="new_password_again">{t('profile.password.again')}</Label>
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
              {changePassword.isPending ? t('profile.password.submitting') : t('profile.password.submit')}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
