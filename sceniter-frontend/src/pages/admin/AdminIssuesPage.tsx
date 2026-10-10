import { ChevronDownIcon, FolderPenIcon, UnlinkIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { episodeCode, posterUrl, type MovieIssue, type ShowIssue, type ShowIssueItem } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from 'core-web/src/components/ui/collapsible'
import { useAdminMovieIssues, useAdminShowIssues, useDetachFile } from '@/api/hooks'
import { Poster } from '@/components/Poster'

/** Odkaz do editoru složky seriálu – tam se číslování opraví. */
function FixLink({ showId, dir }: { showId: string; dir?: string }) {
  const { t } = useTranslation()
  const to = `/shows/${showId}/files${dir ? `?dir=${encodeURIComponent(dir)}` : ''}`
  return (
    <Link to={to} className="inline-flex items-center gap-1 text-xs text-primary hover:underline">
      <FolderPenIcon className="size-3.5" />
      {t('video.admin.showIssues.fix')}
    </Link>
  )
}

function IssueLine({ showId, issue }: { showId: string; issue: ShowIssueItem }) {
  const { t } = useTranslation()
  const season = `S${String(issue.season).padStart(2, '0')}`
  const files = issue.files ?? []
  switch (issue.kind) {
    case 'duplicate_files':
      return (
        <li>
          <p className="flex flex-wrap items-center gap-x-3">
            {t('video.admin.showIssues.duplicate', { episode: episodeCode(issue.season, issue.episode ?? 0), count: files.length })}
            <FixLink showId={showId} dir={issue.dir} />
          </p>
          <FileList files={files} />
        </li>
      )
    case 'gap':
      return (
        <li className="flex flex-wrap items-center gap-x-3">
          {t('video.admin.showIssues.gap', { season, episodes: (issue.missing ?? []).join(', ') })}
          <FixLink showId={showId} dir={issue.dir} />
        </li>
      )
    case 'positional':
      return (
        <li>
          <p className="flex flex-wrap items-center gap-x-3">
            {t('video.admin.showIssues.positional', { season, count: files.length })}
            <FixLink showId={showId} dir={issue.dir} />
          </p>
          <FileList files={files} />
        </li>
      )
  }
}

function FileList({ files }: { files: string[] }) {
  return (
    <ul className="mt-1 mb-2 grid gap-0.5 pl-4">
      {files.map((f) => (
        <li key={f} className="font-mono text-xs break-all text-muted-foreground">
          {f}
        </li>
      ))}
    </ul>
  )
}

function IssueCard({
  poster,
  title,
  to,
  sourceKey,
  summary,
  children,
}: {
  poster: string | undefined
  title: string
  to: string
  sourceKey: string
  summary: string
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  return (
    <Collapsible asChild>
      <li className="glass inset-shadow-glass rounded-2xl p-3 ring-1 ring-glass-edge">
        <div className="flex items-center gap-3">
          <div className="w-12 shrink-0">
            <Poster src={poster} title={title} className="rounded-lg" />
          </div>
          <div className="min-w-0 flex-1">
            <Link to={to} className="font-medium hover:underline">
              {title}
            </Link>
            <p className="truncate font-mono text-xs text-muted-foreground">{sourceKey}</p>
            <p className="text-xs text-muted-foreground">{summary}</p>
          </div>
          <CollapsibleTrigger
            className="group rounded-full p-2 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
            aria-label={t('video.admin.showIssues.details')}
          >
            <ChevronDownIcon className="size-4 transition-transform group-data-[state=open]:rotate-180" />
          </CollapsibleTrigger>
        </div>
        <CollapsibleContent>{children}</CollapsibleContent>
      </li>
    </Collapsible>
  )
}

function ShowIssueRow({ show }: { show: ShowIssue }) {
  const { t } = useTranslation()
  const count = (kind: ShowIssueItem['kind']) => show.issues.filter((i) => i.kind === kind).length
  const summary = [
    count('duplicate_files') ? t('video.admin.showIssues.summaryDuplicate', { count: count('duplicate_files') }) : null,
    count('gap') ? t('video.admin.showIssues.summaryGap', { count: count('gap') }) : null,
    count('positional') ? t('video.admin.showIssues.summaryPositional') : null,
  ].filter(Boolean)
  return (
    <IssueCard poster={posterUrl(show, 48)} title={show.title} to={`/shows/${show.id}`} sourceKey={show.source_key} summary={summary.join(' · ')}>
      <ul className="mt-3 grid gap-1 text-sm">
        {show.issues.map((issue, i) => (
          <IssueLine key={i} showId={show.id} issue={issue} />
        ))}
      </ul>
    </IssueCard>
  )
}

function MovieIssueRow({ movie }: { movie: MovieIssue }) {
  const { t } = useTranslation()
  const detach = useDetachFile()
  return (
    <IssueCard
      poster={posterUrl(movie, 48)}
      title={movie.title}
      to={`/movies/${movie.id}`}
      sourceKey={movie.source_key}
      summary={t('video.admin.movieIssues.summary', { count: movie.files.length })}
    >
      <ul className="mt-3 grid gap-2 text-sm">
        {movie.files.map((f) => (
          <li key={f.id} className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="font-mono text-xs break-all text-muted-foreground">{f.file_path}</span>
            {f.detachable ? (
              <Button
                size="sm"
                variant="outline"
                disabled={detach.isPending}
                onClick={() =>
                  detach.mutate(f.id, {
                    onSuccess: () => toast.success(t('video.detail.detached')),
                    onError: (err) => toast.error(err.message),
                  })
                }
              >
                <UnlinkIcon />
                {t('video.detail.detachFile')}
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
    </IssueCard>
  )
}

/** Kontrola knihovny: seriály s nesedícím číslováním dílů a filmy s přisloučeným cizím souborem. */
export function AdminIssuesPage() {
  const { t } = useTranslation()
  const shows = useAdminShowIssues()
  const movies = useAdminMovieIssues()
  if (shows.isError) return <ErrorState error={shows.error} onRetry={() => void shows.refetch()} />
  if (movies.isError) return <ErrorState error={movies.error} onRetry={() => void movies.refetch()} />
  if (!shows.data || !movies.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  if (shows.data.length === 0 && movies.data.length === 0) return <EmptyState title={t('video.admin.issues.empty')} />
  return (
    <div className="grid gap-8">
      {shows.data.length > 0 ? (
        <section className="grid gap-4">
          <div>
            <h2 className="font-heading text-lg font-semibold">{t('video.admin.showIssues.title', { count: shows.data.length })}</h2>
            <p className="text-sm text-muted-foreground">{t('video.admin.showIssues.hint')}</p>
          </div>
          <ul className="grid gap-2">
            {shows.data.map((s) => (
              <ShowIssueRow key={s.id} show={s} />
            ))}
          </ul>
        </section>
      ) : null}
      {movies.data.length > 0 ? (
        <section className="grid gap-4">
          <div>
            <h2 className="font-heading text-lg font-semibold">{t('video.admin.movieIssues.title', { count: movies.data.length })}</h2>
            <p className="text-sm text-muted-foreground">{t('video.admin.movieIssues.hint')}</p>
          </div>
          <ul className="grid gap-2">
            {movies.data.map((m) => (
              <MovieIssueRow key={m.id} movie={m} />
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}
