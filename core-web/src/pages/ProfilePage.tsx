import { useState } from 'react'
import { toast } from 'sonner'
import { MonitorIcon, SmartphoneIcon } from 'lucide-react'
import {
  useChangePassword,
  useLogoutAll,
  useRevokeDevice,
  useUpdateProfile,
  useUserDevices,
} from '../api/hooks'
import { formatDateTime, loginHint, LOGIN_PATTERN, roleLabel, type DeviceSession } from 'core-shared'
import { useAuth } from '../auth/AuthContext'
import { PageHeader } from '../components/PageHeader'
import { useTranslation } from 'react-i18next'
import { AppearanceSettings } from '../components/layout/AppearanceSettings'
import { UiLanguageSelect } from '../components/layout/UiLanguageSelect'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { formatDate } from 'core-shared'

const MIN_PASSWORD = 8

export function ProfilePage() {
  const { t } = useTranslation()
  const { user, updateUser } = useAuth()

  // ProfilePage se renderuje jen přihlášenému uživateli, takže stačí inicializace.
  const [displayName, setDisplayName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [login, setLogin] = useState(user?.login ?? '')
  const [profilePassword, setProfilePassword] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [password, setPassword] = useState('')
  const [passwordAgain, setPasswordAgain] = useState('')
  const [passwordError, setPasswordError] = useState<string | null>(null)

  const updateProfile = useUpdateProfile(user?.id ?? '')
  const changePassword = useChangePassword(user?.id ?? '')

  if (!user) return null

  // Změna e-mailu nebo loginu je změna přihlašovacích údajů – server chce
  // současné heslo. U samotného jména ho nevyžaduje, pole se pak nezobrazí.
  const credentialsChanged = email.trim() !== user.email || login.trim() !== user.login

  function handleProfileSubmit(event: React.FormEvent) {
    event.preventDefault()
    updateProfile.mutate(
      {
        display_name: displayName.trim(),
        email: email.trim(),
        login: login.trim(),
        ...(credentialsChanged ? { current_password: profilePassword } : {}),
      },
      {
        onSuccess: (updated) => {
          // Server hodnoty normalizuje (trim), proto formulář přepíšeme jeho odpovědí.
          updateUser(updated)
          setDisplayName(updated.display_name)
          setEmail(updated.email)
          setLogin(updated.login)
          setProfilePassword('')
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
      { password, current_password: currentPassword },
      {
        onSuccess: () => {
          setCurrentPassword('')
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
            {credentialsChanged ? (
              <div className="space-y-2">
                <Label htmlFor="profile_current_password">{t('profile.personal.currentPassword')}</Label>
                <Input
                  id="profile_current_password"
                  type="password"
                  autoComplete="current-password"
                  required
                  value={profilePassword}
                  onChange={(e) => setProfilePassword(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">{t('profile.personal.currentPasswordHint')}</p>
              </div>
            ) : null}
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
              <Label htmlFor="current_password">{t('profile.password.current')}</Label>
              <Input
                id="current_password"
                type="password"
                autoComplete="current-password"
                required
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
              />
            </div>
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

      <DevicesCard userId={user.id} />
    </div>
  )
}

/** Přihlášená zařízení s odhlášením jednotlivě i všude. */
function DevicesCard({ userId }: { userId: string }) {
  const { t } = useTranslation()
  const devices = useUserDevices(userId)
  const revoke = useRevokeDevice(userId)
  const logoutAll = useLogoutAll(userId)
  const [confirmOpen, setConfirmOpen] = useState(false)

  function handleRevoke(device: DeviceSession) {
    revoke.mutate(device.id, {
      onSuccess: () => toast.success(t('profile.devices.revoked')),
      onError: (error) => toast.error(error.message),
    })
  }

  function handleLogoutAll() {
    logoutAll.mutate(undefined, {
      onSuccess: () => {
        setConfirmOpen(false)
        toast.success(t('profile.logoutAll.done'))
      },
      onError: (error) => {
        setConfirmOpen(false)
        toast.error(error.message)
      },
    })
  }

  return (
    <>
      <Card className="mt-6">
        <CardHeader>
          <CardTitle>{t('profile.devices.title')}</CardTitle>
          <CardDescription>{t('profile.devices.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          {devices.isError ? (
            <p className="text-sm text-destructive">{t('profile.devices.loadFailed')}</p>
          ) : devices.data && devices.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('profile.devices.empty')}</p>
          ) : (
            <ul className="divide-y divide-border">
              {(devices.data ?? []).map((device) => {
                const Icon = device.scope === 'tv' ? MonitorIcon : SmartphoneIcon
                return (
                  <li key={device.id} className="flex items-center gap-3 py-3 first:pt-0 last:pb-0">
                    <Icon className="size-5 shrink-0 text-muted-foreground" aria-hidden />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">
                        {device.device_name || t('profile.devices.unnamed')}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {t(`profile.devices.type.${device.scope}`)}
                        {' · '}
                        {t('profile.devices.created', { date: formatDateTime(device.created_at) })}
                        {' · '}
                        {t('profile.devices.lastSeen', { date: formatDateTime(device.last_seen_at) })}
                      </p>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={revoke.isPending && revoke.variables === device.id}
                      onClick={() => handleRevoke(device)}
                    >
                      {t('profile.devices.revoke')}
                    </Button>
                  </li>
                )
              })}
            </ul>
          )}
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>{t('profile.logoutAll.title')}</CardTitle>
          <CardDescription>{t('profile.logoutAll.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="destructive" onClick={() => setConfirmOpen(true)}>
            {t('profile.logoutAll.submit')}
          </Button>
        </CardContent>
      </Card>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('profile.logoutAll.confirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('profile.logoutAll.confirm')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={logoutAll.isPending}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={logoutAll.isPending}
              onClick={(event) => {
                // Dialog zavře až odpověď serveru.
                event.preventDefault()
                handleLogoutAll()
              }}
            >
              {logoutAll.isPending ? t('profile.logoutAll.submitting') : t('profile.logoutAll.submit')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
