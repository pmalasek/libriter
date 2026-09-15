import { ArrowDownIcon, ArrowUpIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { useSaveMetadataSettings } from '@/api/adminHooks'
import {
  METADATA_SOURCE_LABELS,
  type AdminProvider,
  type MetadataSettings,
} from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

function sourceLabel(name: string) {
  return METADATA_SOURCE_LABELS[name] ?? name
}

export function MetadataSourcesEditor({ settings }: { settings: MetadataSettings }) {
  const [providers, setProviders] = useState<AdminProvider[]>(settings.providers)
  const [apiKey, setApiKey] = useState(settings.google_books_api_key)
  const [baseline, setBaseline] = useState(settings)

  // Po uložení (nebo změně z jiného okna) převezmeme stav ze serveru. React
  // Query udržuje stejnou referenci, dokud se data opravdu nezmění, takže
  // rozepsané úpravy běžné obnovení nepřepíše.
  if (baseline !== settings) {
    setBaseline(settings)
    setProviders(settings.providers)
    setApiKey(settings.google_books_api_key)
  }

  const save = useSaveMetadataSettings()

  const dirty =
    JSON.stringify(providers) !== JSON.stringify(settings.providers) ||
    apiKey !== settings.google_books_api_key

  function move(index: number, direction: -1 | 1) {
    const target = index + direction
    if (target < 0 || target >= providers.length) return

    const next = [...providers]
    ;[next[index], next[target]] = [next[target], next[index]]
    setProviders(next)
  }

  function toggle(index: number, enabled: boolean) {
    setProviders(providers.map((p, i) => (i === index ? { ...p, enabled } : p)))
  }

  function handleSave() {
    save.mutate(
      {
        providers: providers.map(({ name, enabled }) => ({ name, enabled })),
        google_books_api_key: apiKey.trim(),
      },
      {
        onSuccess: () => toast.success('Zdroje metadat uloženy – změna platí okamžitě.'),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  const enabledCount = providers.filter((p) => p.enabled).length

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Zdroje metadat</CardTitle>
          <CardDescription>
            Zdroje se zkoušejí shora dolů a vyhrají výsledky prvního, který něco najde. Vypnutý
            zdroj se nepoužije ani pro stahování fotek autorů.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <ol className="divide-y rounded-lg border">
            {providers.map((provider, index) => (
              <li key={provider.name} className="flex items-center gap-3 px-3 py-2.5">
                <span className="w-5 text-sm text-muted-foreground tabular-nums">
                  {provider.enabled ? `${providers.slice(0, index + 1).filter((p) => p.enabled).length}.` : '–'}
                </span>

                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{sourceLabel(provider.name)}</p>
                  <p className="text-xs text-muted-foreground">
                    {provider.supports_authors ? 'knihy, autoři i fotky' : 'jen knihy'}
                  </p>
                </div>

                {!provider.enabled ? <Badge variant="outline">vypnuto</Badge> : null}

                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Posunout ${sourceLabel(provider.name)} nahoru`}
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                  >
                    <ArrowUpIcon />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Posunout ${sourceLabel(provider.name)} dolů`}
                    disabled={index === providers.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    <ArrowDownIcon />
                  </Button>
                  <Switch
                    checked={provider.enabled}
                    aria-label={`Zapnout zdroj ${sourceLabel(provider.name)}`}
                    onCheckedChange={(checked) => toggle(index, checked)}
                  />
                </div>
              </li>
            ))}
          </ol>

          {enabledCount === 0 ? (
            <p className="text-sm text-destructive">
              Všechny zdroje jsou vypnuté – načítání metadat nebude fungovat.
            </p>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Google Books</CardTitle>
          <CardDescription>
            Bez klíče platí anonymní denní kvóta sdílená pro celou IP adresu, která se snadno
            vyčerpá. Klíč se získá v Google Cloud konzoli po zapnutí Books API.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Label htmlFor="google_books_api_key">Klíč API</Label>
            <Input
              id="google_books_api_key"
              value={apiKey}
              placeholder="nepovinné"
              onChange={(e) => setApiKey(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <Button disabled={!dirty || save.isPending} onClick={handleSave}>
          {save.isPending ? 'Ukládám…' : 'Uložit'}
        </Button>
        {dirty ? <p className="text-sm text-muted-foreground">Máte neuložené změny.</p> : null}
      </div>
    </div>
  )
}
