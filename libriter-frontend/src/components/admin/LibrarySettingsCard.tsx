import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useLibrarySettings, useSaveLibrarySettings } from '@/api/adminHooks'
import type { LibrarySettings } from '@/api/types'
import { ErrorState } from '@/components/ErrorState'
import { LanguageSelect } from '@/components/LanguageSelect'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'

/** Nastavení knihovny jako celku – zatím výchozí jazyk nových knih. */
export function LibrarySettingsCard() {
  const { t } = useTranslation()
  const settings = useLibrarySettings()

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.library.title')}</CardTitle>
        <CardDescription>{t('admin.library.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        {settings.isPending ? (
          <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
        ) : settings.error ? (
          <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
        ) : (
          <LibrarySettingsForm settings={settings.data} />
        )}
      </CardContent>
    </Card>
  )
}

function LibrarySettingsForm({ settings }: { settings: LibrarySettings }) {
  const { t } = useTranslation()
  const [language, setLanguage] = useState(settings.default_language)
  const [baseline, setBaseline] = useState(settings)

  // Po uložení převezmeme hodnotu ze serveru; React Query drží stejnou
  // referenci, dokud se data nezmění.
  if (baseline !== settings) {
    setBaseline(settings)
    setLanguage(settings.default_language)
  }

  const save = useSaveLibrarySettings()
  const dirty = language !== settings.default_language

  function handleSave() {
    save.mutate(
      { default_language: language },
      {
        onSuccess: () => toast.success(t('admin.library.saved')),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="flex flex-wrap items-end gap-3">
      <div className="w-full space-y-2 sm:w-64">
        <Label htmlFor="library_default_language">{t('admin.library.defaultLanguage')}</Label>
        <LanguageSelect id="library_default_language" value={language} onChange={setLanguage} />
      </div>
      <Button disabled={!dirty || save.isPending} onClick={handleSave}>
        {save.isPending ? t('common.saving') : t('common.save')}
      </Button>
    </div>
  )
}
