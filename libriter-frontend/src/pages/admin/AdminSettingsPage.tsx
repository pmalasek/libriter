import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useRegistrationSettings, useSaveRegistrationSettings } from '@/api/adminHooks'
import { roleLabel, type RegistrationSettings } from '@/api/types'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

// Roli admin přes registraci rozdávat nelze – server ji odmítne.
const DEFAULT_ROLES: RegistrationSettings['default_role'][] = ['reader', 'editor']

export function AdminSettingsPage() {
  const settings = useRegistrationSettings()

  if (settings.isPending) return <LoadingList count={2} />
  if (settings.error) {
    return <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
  }

  return <RegistrationForm settings={settings.data} />
}

function RegistrationForm({ settings }: { settings: RegistrationSettings }) {
  const { t } = useTranslation()
  const [enabled, setEnabled] = useState(settings.enabled)
  const [defaultRole, setDefaultRole] = useState(settings.default_role)
  const [baseline, setBaseline] = useState(settings)

  // Po uložení převezmeme hodnoty ze serveru; React Query drží stejnou
  // referenci, dokud se data nezmění.
  if (baseline !== settings) {
    setBaseline(settings)
    setEnabled(settings.enabled)
    setDefaultRole(settings.default_role)
  }

  const save = useSaveRegistrationSettings()
  const dirty = enabled !== settings.enabled || defaultRole !== settings.default_role

  function handleSave() {
    save.mutate(
      { enabled, default_role: defaultRole },
      {
        onSuccess: () => toast.success(t('admin.registration.saved')),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="max-w-2xl space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t('admin.registration.title')}</CardTitle>
          <CardDescription>
            <Trans i18nKey="admin.registration.description" components={{ code: <code /> }} />
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between gap-4">
            <Label htmlFor="registration_enabled" className="font-normal">
              {t('admin.registration.enable')}
            </Label>
            <Switch id="registration_enabled" checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="registration_role">{t('admin.registration.defaultRole')}</Label>
            <Select
              value={defaultRole}
              onValueChange={(value) =>
                setDefaultRole(value as RegistrationSettings['default_role'])
              }
            >
              <SelectTrigger id="registration_role" className="w-full sm:w-64">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DEFAULT_ROLES.map((role) => (
                  <SelectItem key={role} value={role}>
                    {roleLabel(role)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-sm text-muted-foreground">
              {t('admin.registration.roleHint')}
            </p>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <Button disabled={!dirty || save.isPending} onClick={handleSave}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </Button>
        {dirty ? (
          <p className="text-sm text-muted-foreground">{t('admin.unsavedChanges')}</p>
        ) : null}
      </div>
    </div>
  )
}
