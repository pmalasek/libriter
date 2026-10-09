import { RefreshCwIcon, SparklesIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatBytes, formatDateTime } from 'sceniter-shared'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from 'core-web/src/components/ui/card'
import { useAdminAction, useAdminStats } from '@/api/hooks'

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

  if (stats.isError) return <ErrorState error={stats.error} onRetry={() => void stats.refetch()} />
  const s = stats.data
  if (!s) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  const sc = s.scanner
  const lib = s.library

  return (
    <div className="grid gap-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <Stat label={t('video.admin.stats.movies')} value={lib.movies} />
        <Stat label={t('video.admin.stats.shows')} value={lib.shows} />
        <Stat label={t('video.admin.stats.episodes')} value={lib.episodes} />
        <Stat label={t('video.admin.stats.files')} value={lib.files} />
        <Stat label={t('video.admin.stats.size')} value={formatBytes(lib.total_bytes)} />
        <Stat label={t('video.admin.stats.unmatched')} value={lib.unmatched_movies + lib.unmatched_shows} />
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
            <CardDescription>{s.tmdb ? t('video.admin.metadata.enabled') : t('video.admin.metadata.disabled')}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button
              variant="outline"
              disabled={!s.tmdb || action.isPending}
              onClick={() =>
                action.mutate(
                  { path: '/admin/metadata/match-all' },
                  { onSuccess: () => toast.success(t('video.admin.metadata.started')), onError: (e) => toast.error(e.message) },
                )
              }
            >
              <SparklesIcon />
              {t('video.admin.metadata.matchAll')}
            </Button>
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
