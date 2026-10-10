import { ChevronDownIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { posterUrl, type ShowIssue, type ShowIssueItem } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from 'core-web/src/components/ui/collapsible'
import { useAdminShowIssues } from '@/api/hooks'
import { Poster } from '@/components/Poster'

const pad = (n: number) => String(n).padStart(2, '0')

function IssueLine({ issue }: { issue: ShowIssueItem }) {
  const { t } = useTranslation()
  const season = `S${pad(issue.season)}`
  const files = issue.files ?? []
  switch (issue.kind) {
    case 'duplicate_files':
      return (
        <li>
          <p>{t('video.admin.showIssues.duplicate', { episode: `${season}E${pad(issue.episode ?? 0)}`, count: files.length })}</p>
          <FileList files={files} />
        </li>
      )
    case 'gap':
      return <li>{t('video.admin.showIssues.gap', { season, episodes: (issue.missing ?? []).join(', ') })}</li>
    case 'positional':
      return (
        <li>
          <p>{t('video.admin.showIssues.positional', { season, count: files.length })}</p>
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

function ShowIssueRow({ show }: { show: ShowIssue }) {
  const { t } = useTranslation()
  const count = (kind: ShowIssueItem['kind']) => show.issues.filter((i) => i.kind === kind).length
  const summary = [
    count('duplicate_files') ? t('video.admin.showIssues.summaryDuplicate', { count: count('duplicate_files') }) : null,
    count('gap') ? t('video.admin.showIssues.summaryGap', { count: count('gap') }) : null,
    count('positional') ? t('video.admin.showIssues.summaryPositional') : null,
  ].filter(Boolean)
  return (
    <Collapsible asChild>
      <li className="glass inset-shadow-glass rounded-2xl p-3 ring-1 ring-glass-edge">
        <div className="flex items-center gap-3">
          <div className="w-12 shrink-0">
            <Poster src={posterUrl(show, 48)} title={show.title} className="rounded-lg" />
          </div>
          <div className="min-w-0 flex-1">
            <Link to={`/shows/${show.id}`} className="font-medium hover:underline">
              {show.title}
            </Link>
            <p className="truncate font-mono text-xs text-muted-foreground">{show.source_key}</p>
            <p className="text-xs text-muted-foreground">{summary.join(' · ')}</p>
          </div>
          <CollapsibleTrigger
            className="group rounded-full p-2 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
            aria-label={t('video.admin.showIssues.details')}
          >
            <ChevronDownIcon className="size-4 transition-transform group-data-[state=open]:rotate-180" />
          </CollapsibleTrigger>
        </div>
        <CollapsibleContent>
          <ul className="mt-3 grid gap-1 text-sm">
            {show.issues.map((issue, i) => (
              <IssueLine key={i} issue={issue} />
            ))}
          </ul>
        </CollapsibleContent>
      </li>
    </Collapsible>
  )
}

/** Kontrola číslování dílů: seriály, u kterých soubory nesedí na díly. */
export function AdminShowIssuesPage() {
  const { t } = useTranslation()
  const issues = useAdminShowIssues()
  if (issues.isError) return <ErrorState error={issues.error} onRetry={() => void issues.refetch()} />
  if (!issues.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  if (issues.data.length === 0) return <EmptyState title={t('video.admin.showIssues.empty')} />
  return (
    <section className="grid gap-4">
      <div>
        <h2 className="font-heading text-lg font-semibold">{t('video.admin.showIssues.title', { count: issues.data.length })}</h2>
        <p className="text-sm text-muted-foreground">{t('video.admin.showIssues.hint')}</p>
      </div>
      <ul className="grid gap-2">
        {issues.data.map((s) => (
          <ShowIssueRow key={s.id} show={s} />
        ))}
      </ul>
    </section>
  )
}
