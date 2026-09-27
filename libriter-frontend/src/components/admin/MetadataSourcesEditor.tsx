import { ArrowDownIcon, ArrowUpIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSaveMetadataSettings } from '@/api/adminHooks'
import { useLanguages } from '@/api/hooks'
import {
  METADATA_SOURCE_LABELS,
  type AdminProvider,
  type MetadataLanguageProfile,
  type MetadataSettings,
} from '@/api/types'
import { LanguageSelect } from '@/components/LanguageSelect'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { languageLabel } from '@/lib/format'

function sourceLabel(name: string) {
  return METADATA_SOURCE_LABELS[name] ?? name
}

const toRequest = (providers: AdminProvider[]) => providers.map(({ name, enabled }) => ({ name, enabled }))

/**
 * Seřaditelný seznam zdrojů s vypínačem. Pořadí je pořadí, ve kterém se zdroje
 * zkoušejí; vypnuté zdroje v pořadí nečíslujeme.
 */
function ProviderList({
  providers,
  onChange,
}: {
  providers: AdminProvider[]
  onChange: (providers: AdminProvider[]) => void
}) {
  const { t } = useTranslation()

  function move(index: number, direction: -1 | 1) {
    const target = index + direction
    if (target < 0 || target >= providers.length) return

    const next = [...providers]
    ;[next[index], next[target]] = [next[target], next[index]]
    onChange(next)
  }

  function toggle(index: number, enabled: boolean) {
    onChange(providers.map((p, i) => (i === index ? { ...p, enabled } : p)))
  }

  const enabledCount = providers.filter((p) => p.enabled).length

  return (
    <>
      <ol className="divide-y rounded-lg border">
        {providers.map((provider, index) => (
          <li key={provider.name} className="flex items-center gap-3 px-3 py-2.5">
            <span className="w-5 text-sm text-muted-foreground tabular-nums">
              {provider.enabled ? `${providers.slice(0, index + 1).filter((p) => p.enabled).length}.` : '–'}
            </span>

            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">{sourceLabel(provider.name)}</p>
              <p className="text-xs text-muted-foreground">
                {provider.supports_authors
                  ? t('admin.metadata.supportsAuthors')
                  : t('admin.metadata.booksOnly')}
              </p>
            </div>

            {!provider.enabled ? <Badge variant="outline">{t('admin.metadata.disabled')}</Badge> : null}

            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                aria-label={t('admin.metadata.moveUp', { source: sourceLabel(provider.name) })}
                disabled={index === 0}
                onClick={() => move(index, -1)}
              >
                <ArrowUpIcon />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t('admin.metadata.moveDown', { source: sourceLabel(provider.name) })}
                disabled={index === providers.length - 1}
                onClick={() => move(index, 1)}
              >
                <ArrowDownIcon />
              </Button>
              <Switch
                checked={provider.enabled}
                aria-label={t('admin.metadata.enable', { source: sourceLabel(provider.name) })}
                onCheckedChange={(checked) => toggle(index, checked)}
              />
            </div>
          </li>
        ))}
      </ol>

      {enabledCount === 0 ? (
        <p className="text-sm text-destructive">{t('admin.metadata.allDisabled')}</p>
      ) : null}
    </>
  )
}

export function MetadataSourcesEditor({ settings }: { settings: MetadataSettings }) {
  const { t } = useTranslation()
  const languageList = useLanguages()
  const [providers, setProviders] = useState<AdminProvider[]>(settings.providers)
  const [languages, setLanguages] = useState<MetadataLanguageProfile[]>(settings.languages)
  const [apiKey, setApiKey] = useState(settings.google_books_api_key)
  const [baseline, setBaseline] = useState(settings)

  // Po uložení (nebo změně z jiného okna) převezmeme stav ze serveru. React
  // Query udržuje stejnou referenci, dokud se data opravdu nezmění, takže
  // rozepsané úpravy běžné obnovení nepřepíše.
  if (baseline !== settings) {
    setBaseline(settings)
    setProviders(settings.providers)
    setLanguages(settings.languages)
    setApiKey(settings.google_books_api_key)
  }

  const save = useSaveMetadataSettings()

  const dirty =
    JSON.stringify(providers) !== JSON.stringify(settings.providers) ||
    JSON.stringify(languages) !== JSON.stringify(settings.languages) ||
    apiKey !== settings.google_books_api_key

  function addLanguage(code: string) {
    if (languages.some((l) => l.language === code)) {
      toast.error(t('admin.metadata.languageExists', { language: languageLabel(code, languageList.data) }))
      return
    }
    // Nový profil začíná jako kopie výchozího pořadí – admin ho jen upraví.
    setLanguages(
      [...languages, { language: code, providers }].sort((a, b) => a.language.localeCompare(b.language)),
    )
  }

  function updateLanguage(code: string, next: AdminProvider[]) {
    setLanguages(languages.map((l) => (l.language === code ? { ...l, providers: next } : l)))
  }

  function removeLanguage(code: string) {
    setLanguages(languages.filter((l) => l.language !== code))
  }

  function handleSave() {
    save.mutate(
      {
        providers: toRequest(providers),
        languages: languages.map((l) => ({ language: l.language, providers: toRequest(l.providers) })),
        google_books_api_key: apiKey.trim(),
      },
      {
        onSuccess: () => toast.success(t('admin.metadata.saved')),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t('admin.metadata.title')}</CardTitle>
          <CardDescription>{t('admin.metadata.description')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <ProviderList providers={providers} onChange={setProviders} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('admin.metadata.languagesTitle')}</CardTitle>
          <CardDescription>{t('admin.metadata.languagesDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {languages.map((profile) => (
            <section key={profile.language} className="space-y-3">
              <div className="flex items-center gap-2">
                <h3 className="flex-1 font-medium">
                  {languageLabel(profile.language, languageList.data)}
                  <span className="ml-2 font-mono text-xs text-muted-foreground">{profile.language}</span>
                </h3>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => removeLanguage(profile.language)}
                  aria-label={t('admin.metadata.removeLanguage', {
                    language: languageLabel(profile.language, languageList.data),
                  })}
                >
                  <Trash2Icon />
                  {t('admin.metadata.removeLanguageShort')}
                </Button>
              </div>
              <ProviderList
                providers={profile.providers}
                onChange={(next) => updateLanguage(profile.language, next)}
              />
            </section>
          ))}

          {languages.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('admin.metadata.noLanguages')}</p>
          ) : null}

          <div className="space-y-2">
            <Label htmlFor="metadata_add_language">{t('admin.metadata.addLanguage')}</Label>
            <div className="max-w-xs">
              <LanguageSelect id="metadata_add_language" value="" onChange={addLanguage} />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Google Books</CardTitle>
          <CardDescription>{t('admin.metadata.googleDescription')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Label htmlFor="google_books_api_key">{t('admin.metadata.apiKey')}</Label>
            <Input
              id="google_books_api_key"
              value={apiKey}
              placeholder={t('admin.metadata.optional')}
              onChange={(e) => setApiKey(e.target.value)}
            />
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
