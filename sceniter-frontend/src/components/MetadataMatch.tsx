import { ExternalLinkIcon, SearchIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { TitleKind } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import { Input } from 'core-web/src/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from 'core-web/src/components/ui/select'
import { useMatchTitle, useMetadataProviders, useMetadataSearch } from '@/api/hooks'

/**
 * Ruční spárování se zdrojem metadat: výběr zdroje (výchozí je zdroj
 * knihovny titulu), hledání podle názvu a roku, výběr výsledku. Náhledy
 * výsledků jdou přímo ze zdroje (ještě nejsou v knihovně).
 */
export function MetadataMatch({
  kind,
  id,
  initialQuery,
  initialYear,
  onMatched,
}: {
  kind: TitleKind
  id: string
  initialQuery: string
  initialYear?: number
  onMatched?: (newId: string) => void
}) {
  const { t } = useTranslation()
  const providers = (useMetadataProviders().data ?? []).filter((p) => p.kinds.includes(kind))
  // '' = výchozí zdroj knihovny; server v odpovědi řekne, který to je.
  const [provider, setProvider] = useState('')
  const [query, setQuery] = useState(initialQuery)
  const [year, setYear] = useState(initialYear ? String(initialYear) : '')
  const [submitted, setSubmitted] = useState({ q: initialQuery, year: initialYear })
  const results = useMetadataSearch(kind, provider, submitted.q, submitted.year, id)
  const match = useMatchTitle()
  const active = provider || results.data?.provider || ''

  return (
    <div className="grid gap-3">
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setSubmitted({ q: query, year: Number(year) || undefined })
        }}
      >
        {providers.length > 1 ? (
          <Select value={active} onValueChange={setProvider}>
            <SelectTrigger className="w-44" aria-label={t('video.admin.match.source')}>
              <SelectValue placeholder={t('video.admin.match.source')} />
            </SelectTrigger>
            <SelectContent>
              {providers.map((p) => (
                <SelectItem key={p.name} value={p.name}>
                  {p.title}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : null}
        <Input value={query} onChange={(e) => setQuery(e.target.value)} className="min-w-48 flex-1" aria-label={t('video.admin.match.search')} />
        <Input
          value={year}
          onChange={(e) => setYear(e.target.value)}
          inputMode="numeric"
          placeholder={t('video.admin.match.year')}
          aria-label={t('video.admin.match.year')}
          className="w-24"
        />
        <Button type="submit" variant="outline">
          <SearchIcon />
          {t('video.admin.match.search')}
        </Button>
      </form>

      {results.isError ? <p className="text-sm text-destructive">{results.error.message}</p> : null}
      {results.data && results.data.results.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('video.admin.match.noResults')}</p>
      ) : null}

      {/* Výsledky rolují uvnitř dialogu – jinak by dialog přerostl obrazovku. */}
      <ul className="-mr-2 grid max-h-[min(60dvh,40rem)] gap-2 overflow-y-auto overscroll-contain pr-2">
        {(results.data?.results ?? []).slice(0, 10).map((r) => (
          <li key={r.id} className="flex items-start gap-3 rounded-2xl bg-foreground/4 p-2">
            {r.image_url ? (
              <img src={r.image_url} alt="" className="h-20 w-14 shrink-0 rounded-lg object-cover" loading="lazy" referrerPolicy="no-referrer" />
            ) : (
              <div className="h-20 w-14 shrink-0 rounded-lg bg-muted" />
            )}
            <div className="min-w-0 flex-1">
              <p className="font-medium">
                {r.title} {r.year ? <span className="text-muted-foreground">({r.year})</span> : null}
                {r.url ? (
                  <a href={r.url} target="_blank" rel="noreferrer" className="ml-1 inline-flex align-middle text-muted-foreground hover:text-foreground">
                    <ExternalLinkIcon className="size-3.5" />
                  </a>
                ) : null}
              </p>
              {r.original_title && r.original_title !== r.title ? (
                <p className="text-xs text-muted-foreground">{r.original_title}</p>
              ) : null}
              {r.note ? <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{r.note}</p> : null}
            </div>
            <Button
              size="sm"
              disabled={match.isPending || !active}
              onClick={() =>
                match.mutate(
                  { kind, id, provider: active, metaId: r.id },
                  {
                    onSuccess: (res) => {
                      toast.success(t('video.admin.match.matched'))
                      onMatched?.(res.id)
                    },
                    onError: (err) => toast.error(err.message),
                  },
                )
              }
            >
              {t('video.admin.match.choose')}
            </Button>
          </li>
        ))}
      </ul>
    </div>
  )
}
