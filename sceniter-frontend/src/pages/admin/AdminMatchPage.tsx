import { EyeOffIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { posterUrl, type Movie, type Show, type TitleKind } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { useAdminUnmatched, useSetMatchState } from '@/api/hooks'
import { Poster } from '@/components/Poster'
import { MetadataMatch } from '@/components/MetadataMatch'

function UnmatchedRow({ kind, title }: { kind: TitleKind; title: Movie | Show }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const setState = useSetMatchState()
  return (
    <li className="glass inset-shadow-glass rounded-2xl p-3 ring-1 ring-glass-edge">
      <div className="flex items-center gap-3">
        <div className="w-12 shrink-0">
          <Poster src={posterUrl(title)} title={title.title} className="rounded-lg" />
        </div>
        <div className="min-w-0 flex-1">
          <Link to={`/${kind === 'movie' ? 'movies' : 'shows'}/${title.id}`} className="font-medium hover:underline">
            {title.title} {title.year ? <span className="text-muted-foreground">({title.year})</span> : null}
          </Link>
          <p className="truncate font-mono text-xs text-muted-foreground">{title.source_key}</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => setOpen((o) => !o)}>
          {t('video.admin.match.search')}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() =>
            setState.mutate(
              { kind, id: title.id, state: 'ignored' },
              { onSuccess: () => toast.success(t('video.admin.match.ignored')), onError: (e) => toast.error(e.message) },
            )
          }
        >
          <EyeOffIcon />
          {t('video.admin.match.ignore')}
        </Button>
      </div>
      {open ? (
        <div className="mt-3">
          <MetadataMatch kind={kind} id={title.id} initialQuery={title.original_title || title.title} initialYear={title.year} onMatched={() => setOpen(false)} />
        </div>
      ) : null}
    </li>
  )
}

export function AdminMatchPage() {
  const { t } = useTranslation()
  const unmatched = useAdminUnmatched()
  if (unmatched.isError) return <ErrorState error={unmatched.error} onRetry={() => void unmatched.refetch()} />
  const d = unmatched.data
  if (!d) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  if (d.movies.length + d.shows.length === 0) return <EmptyState title={t('video.admin.match.empty')} />

  return (
    <div className="grid gap-8">
      {d.movies.length > 0 ? (
        <section>
          <h2 className="font-heading mb-3 text-lg font-semibold">{t('video.admin.match.movies')}</h2>
          <ul className="grid gap-2">
            {d.movies.map((m) => (
              <UnmatchedRow key={m.id} kind="movie" title={m} />
            ))}
          </ul>
        </section>
      ) : null}
      {d.shows.length > 0 ? (
        <section>
          <h2 className="font-heading mb-3 text-lg font-semibold">{t('video.admin.match.shows')}</h2>
          <ul className="grid gap-2">
            {d.shows.map((s) => (
              <UnmatchedRow key={s.id} kind="show" title={s} />
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}
