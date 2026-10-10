import { DatabaseZapIcon, RefreshCwIcon, SparklesIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { formatBytes, formatDateTime } from 'sceniter-shared'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from 'core-web/src/components/ui/card'
import { useAdminAction, useAdminStats, useMatchStatus, useRefreshStatus } from '@/api/hooks'

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="glass inset-shadow-glass rounded-2xl p-4 ring-1 ring-glass-edge">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="font-heading mt-1 text-2xl font-semibold tabular-nums">{value}</p>
    </div>
  )
}

export function AdminOverviewPage() {
  const { t } = useTranslation()
  const stats = useAdminStats()
  const action = useAdminAction()
  const refresh = useRefreshStatus()
  const rs = refresh.data
  const matching = useMatchStatus()
  const ms = matching.data

  if (stats.isError) return <ErrorState error={stats.error} onRetry={() => void stats.refetch()} />
  const s = stats.data
  if (!s) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  const sc = s.scanner
  const lib = s.library
  const unmatched = lib.unmatched_movies + lib.unmatched_shows
  const failed = lib.failed_movies + lib.failed_shows

  return (
    <div className="grid gap-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <Stat label={t('video.admin.stats.movies')} value={lib.movies} />
        <Stat label={t('video.admin.stats.shows')} value={lib.shows} />
        <Stat label={t('video.admin.stats.episodes')} value={lib.episodes} />
        <Stat label={t('video.admin.stats.files')} value={lib.files} />
        <Stat label={t('video.admin.stats.size')} value={formatBytes(lib.total_bytes)} />
        <Stat label={t('video.admin.stats.unmatched')} value={unmatched} />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t('video.admin.scanner.title')}</CardTitle>
            <CardDescription>
              {sc.running ? t('video.admin.scanner.running') : t('video.admin.scanner.idle')}
              {sc.finished_at ? ` · ${t('video.admin.scanner.lastRun', { date: formatDateTime(sc.finished_at) })}` : ''}
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 text-sm">
            <p>{t('video.admin.scanner.processed', { count: sc.processed, added: sc.added, removed: sc.removed })}</p>
            <p className="text-muted-foreground">{sc.watcher_active ? t('video.admin.scanner.watcher') : t('video.admin.scanner.watcherOff')}</p>
            {!sc.ffprobe ? <p className="text-destructive">{t('video.admin.scanner.noFfprobe')}</p> : null}
            {sc.errors > 0 ? <p className="text-destructive">{t('video.admin.scanner.errors', { count: sc.errors, error: sc.last_error })}</p> : null}
            <Button
              className="w-fit"
              disabled={sc.running || action.isPending}
              onClick={() =>
                action.mutate(
                  { path: '/admin/scanner/rescan' },
                  { onSuccess: () => toast.success(t('video.admin.scanner.started')), onError: (e) => toast.error(e.message) },
                )
              }
            >
              <RefreshCwIcon className={sc.running ? 'animate-spin' : undefined} />
              {t('video.admin.scanner.rescan')}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('video.admin.metadata.title')}</CardTitle>
            <CardDescription>{s.metadata ? t('video.admin.metadata.enabled') : t('video.admin.metadata.disabled')}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 text-sm">
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                disabled={!s.metadata || action.isPending || ms?.running}
                onClick={() =>
                  action.mutate(
                    { path: '/admin/metadata/match-all' },
                    {
                      onSuccess: () => {
                        toast.success(t('video.admin.metadata.started'))
                        void matching.refetch()
                      },
                      onError: (e) => toast.error(e.message),
                    },
                  )
                }
              >
                <SparklesIcon className={ms?.running ? 'animate-pulse' : undefined} />
                {t('video.admin.metadata.matchAll')}
              </Button>
              <Button
                variant="outline"
                disabled={!s.metadata || action.isPending || rs?.running}
                onClick={() =>
                  action.mutate(
                    { path: '/admin/metadata/refresh-all' },
                    { onSuccess: () => toast.success(t('video.admin.metadata.refreshStarted')), onError: (e) => toast.error(e.message) },
                  )
                }
              >
                <DatabaseZapIcon className={rs?.running ? 'animate-pulse' : undefined} />
                {t('video.admin.metadata.refreshAll')}
              </Button>
            </div>
            {ms?.running ? (
              <div className="grid gap-1">
                <p className="tabular-nums">
                  {t('video.admin.metadata.matchRunning', { done: ms.done, total: ms.total, matched: ms.matched, failed: ms.failed })}
                </p>
                <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full bg-primary transition-[width]"
                    style={{ width: `${ms.total > 0 ? (ms.done / ms.total) * 100 : 0}%` }}
                  />
                </div>
                {ms.current ? <p className="truncate text-muted-foreground">{t('video.admin.metadata.matchCurrent', { title: ms.current })}</p> : null}
              </div>
            ) : ms && ms.total > 0 ? (
              <p className="text-muted-foreground tabular-nums">
                {t('video.admin.metadata.matchDone', { matched: ms.matched, failed: ms.failed, deferred: ms.deferred })}
              </p>
            ) : null}
            {ms?.paused_until ? (
              <p className="text-muted-foreground">{t('video.admin.metadata.matchPaused', { time: formatDateTime(ms.paused_until) })}</p>
            ) : null}
            {unmatched > 0 ? (
              <p className="tabular-nums">
                {t('video.admin.metadata.unmatchedSummary', { total: unmatched, pending: unmatched - failed, failed })}{' '}
                <Link to="/admin/match" className="text-primary hover:underline">
                  {t('video.admin.metadata.showUnmatched')}
                </Link>
              </p>
            ) : null}
            <p className="text-muted-foreground">{t('video.admin.metadata.refreshHint')}</p>
            {rs?.running ? (
              <p className="tabular-nums">{t('video.admin.metadata.refreshRunning', { done: rs.done, total: rs.total })}</p>
            ) : rs && rs.total > 0 ? (
              <p className="text-muted-foreground tabular-nums">
                {t('video.admin.metadata.refreshDone', { done: rs.done, failed: rs.failed })}
              </p>
            ) : null}
          </CardContent>
        </Card>

        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>{t('video.admin.users.title')}</CardTitle>
            <CardDescription>{t('video.admin.users.description')}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    </div>
  )
}
