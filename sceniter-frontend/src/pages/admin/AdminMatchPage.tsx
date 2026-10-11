import { EyeOffIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { formatDateTime, posterUrl, type MovieSummary, type ShowSummary, type TitleKind } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Skeleton } from 'core-web/src/components/ui/skeleton'
import { VirtualGrid } from 'core-web/src/components/VirtualGrid'
import type { PagedList } from 'core-web/src/lib/paged'
import { useAdminUnmatched, useSetMatchState } from '@/api/hooks'
import { Poster } from '@/components/Poster'
import { MetadataMatch } from '@/components/MetadataMatch'

function UnmatchedRow({ kind, title }: { kind: TitleKind; title: MovieSummary | ShowSummary }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const setState = useSetMatchState()
  return (
    <div className="glass inset-shadow-glass rounded-2xl p-3 ring-1 ring-glass-edge">
      <div className="flex items-center gap-3">
        <div className="w-12 shrink-0">
          <Poster src={posterUrl(title, 48)} title={title.title} kind={kind} year={title.year} className="rounded-lg" />
        </div>
        <div className="min-w-0 flex-1">
          <Link to={`/${kind === 'movie' ? 'movies' : 'shows'}/${title.id}`} className="font-medium hover:underline">
            {title.title} {title.year ? <span className="text-muted-foreground">({title.year})</span> : null}
          </Link>
          <p className="truncate font-mono text-xs text-muted-foreground">{title.source_key}</p>
          {title.match_attempted_at ? (
            <p className="text-xs text-muted-foreground">{t('video.admin.match.attempted', { date: formatDateTime(title.match_attempted_at) })}</p>
          ) : null}
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
    </div>
  )
}

/** Seznam nespárovaných titulů jednoho druhu – virtualizovaný, stránkovaný. */
function UnmatchedList({ kind, list }: { kind: TitleKind; list: PagedList<MovieSummary | ShowSummary> }) {
  const { t } = useTranslation()
  if (!list.total) return null
  return (
    <div>
      <h3 className="mb-2 text-sm font-medium text-muted-foreground">
        {kind === 'movie' ? t('video.admin.match.movies') : t('video.admin.match.shows')}
      </h3>
      <VirtualGrid
        count={list.total}
        columns={() => 1}
        gap={0}
        rowGap={8}
        estimateRowHeight={() => 96}
        onRangeChange={list.setRange}
        renderItem={(index) => {
          const title = list.getItem(index)
          return title ? <UnmatchedRow kind={kind} title={title} /> : <Skeleton className="h-24 rounded-2xl" />
        }}
      />
    </div>
  )
}

function TitleGroup({
  heading,
  hint,
  movies,
  shows,
}: {
  heading: string
  hint?: string
  movies: PagedList<MovieSummary>
  shows: PagedList<ShowSummary>
}) {
  if ((movies.total ?? 0) + (shows.total ?? 0) === 0) return null
  return (
    <section className="grid gap-4">
      <div>
        <h2 className="font-heading text-lg font-semibold">{heading}</h2>
        {hint ? <p className="text-sm text-muted-foreground">{hint}</p> : null}
      </div>
      <UnmatchedList kind="movie" list={movies} />
      <UnmatchedList kind="show" list={shows} />
    </section>
  )
}

export function AdminMatchPage() {
  const { t } = useTranslation()
  // Nenalezené (žádný zdroj nenašel jistou shodu) zvlášť od těch, na které
  // párování ještě nedošlo. Dělí a počítá je server, prohlížeč drží jen
  // stránky kolem viditelné části.
  const failedMovies = useAdminUnmatched('movie', true)
  const failedShows = useAdminUnmatched('show', true)
  const pendingMovies = useAdminUnmatched('movie', false)
  const pendingShows = useAdminUnmatched('show', false)
  const lists = [failedMovies, failedShows, pendingMovies, pendingShows]

  const failed = lists.find((l) => l.isError && l.total === undefined)
  if (failed) return <ErrorState error={failed.error} onRetry={() => lists.forEach((l) => l.refetch())} />
  if (lists.some((l) => l.total === undefined)) {
    return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  }
  const total = (l: PagedList<unknown>) => l.total ?? 0
  if (lists.every((l) => total(l) === 0)) return <EmptyState title={t('video.admin.match.empty')} />

  return (
    <div className="grid gap-8">
      <TitleGroup
        heading={t('video.admin.match.failed', { count: total(failedMovies) + total(failedShows) })}
        hint={t('video.admin.match.failedHint')}
        movies={failedMovies}
        shows={failedShows}
      />
      <TitleGroup
        heading={t('video.admin.match.pending', { count: total(pendingMovies) + total(pendingShows) })}
        movies={pendingMovies}
        shows={pendingShows}
      />
    </div>
  )
}
