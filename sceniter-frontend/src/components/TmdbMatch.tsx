import { SearchIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { TitleKind } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import { Input } from 'core-web/src/components/ui/input'
import { useMatchTitle, useTmdbSearch } from '@/api/hooks'

const TMDB_IMAGE = 'https://image.tmdb.org/t/p/w92'

/**
 * Ruční spárování s TMDB: hledání podle názvu a roku, výběr výsledku.
 * Plakáty výsledků jdou přímo z CDN TMDB (ještě nejsou v knihovně).
 */
export function TmdbMatch({
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
  const [query, setQuery] = useState(initialQuery)
  const [year, setYear] = useState(initialYear ? String(initialYear) : '')
  const [submitted, setSubmitted] = useState({ q: initialQuery, year: initialYear })
  const results = useTmdbSearch(kind, submitted.q, submitted.year)
  const match = useMatchTitle()

  return (
    <div className="grid gap-3">
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          setSubmitted({ q: query, year: Number(year) || undefined })
        }}
      >
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
      {results.data && results.data.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('video.admin.match.noResults')}</p>
      ) : null}

      <ul className="grid gap-2">
        {(results.data ?? []).slice(0, 8).map((r) => (
          <li key={r.id} className="flex items-start gap-3 rounded-2xl bg-foreground/4 p-2">
            {r.poster_path ? (
              <img src={TMDB_IMAGE + r.poster_path} alt="" className="h-20 w-14 shrink-0 rounded-lg object-cover" loading="lazy" />
            ) : (
              <div className="h-20 w-14 shrink-0 rounded-lg bg-muted" />
            )}
            <div className="min-w-0 flex-1">
              <p className="font-medium">
                {r.title} {r.year ? <span className="text-muted-foreground">({r.year})</span> : null}
              </p>
              {r.original_title && r.original_title !== r.title ? (
                <p className="text-xs text-muted-foreground">{r.original_title}</p>
              ) : null}
              <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{r.overview}</p>
            </div>
            <Button
              size="sm"
              disabled={match.isPending}
              onClick={() =>
                match.mutate(
                  { kind, id, tmdbId: r.id },
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

