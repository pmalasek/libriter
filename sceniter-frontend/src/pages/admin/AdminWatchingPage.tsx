import { HistoryIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { formatDateTime, formatDuration } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { LoadingList } from 'core-web/src/components/LoadingList'
import { Button } from 'core-web/src/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from 'core-web/src/components/ui/table'
import { useAdminWatching } from '@/api/hooks'

/** Doba sledování; nula by se jinak vypsala jako „neznámá délka“. */
function watchedDuration(seconds: number): string {
  return seconds > 0 ? formatDuration(seconds) : '–'
}

/** Poznámka, že historie začíná až nasazením této verze serveru. */
export function WatchingSinceNote() {
  const { t } = useTranslation()
  return <p className="text-sm text-muted-foreground">{t('video.admin.watching.sinceNote')}</p>
}

/**
 * Přehled sledování všech účtů: kolik toho kdo viděl a kdy naposledy.
 * Počítají se sekundy skutečně přehraného obsahu, ne čas strávený na stránce.
 */
export function AdminWatchingPage() {
  const { t } = useTranslation()
  const overview = useAdminWatching()

  if (overview.isPending) return <LoadingList count={4} />
  if (overview.error) return <ErrorState error={overview.error} onRetry={() => void overview.refetch()} />
  if (overview.data.length === 0) return <EmptyState title={t('video.admin.watching.empty')} icon={HistoryIcon} />

  return (
    <div className="space-y-4">
      <WatchingSinceNote />
      <div className="rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('video.admin.watching.user')}</TableHead>
              <TableHead className="w-24 text-right">{t('video.admin.watching.movies')}</TableHead>
              <TableHead className="w-24 text-right">{t('video.admin.watching.shows')}</TableHead>
              <TableHead className="w-24 text-right">{t('video.admin.watching.episodes')}</TableHead>
              <TableHead className="w-32 text-right">{t('video.admin.watching.watched')}</TableHead>
              <TableHead className="w-40">{t('video.admin.watching.lastWatched')}</TableHead>
              <TableHead className="w-16 text-right">{t('video.admin.watching.detail')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {overview.data.map((row) => (
              <TableRow key={row.user_id}>
                <TableCell>
                  <Link
                    to={`/admin/watching/${row.user_id}`}
                    className="font-medium text-primary underline-offset-4 hover:underline"
                  >
                    {row.display_name}
                  </Link>
                  <p className="break-all text-xs text-muted-foreground">{row.email}</p>
                </TableCell>
                <TableCell className="text-right tabular-nums">{row.movies}</TableCell>
                <TableCell className="text-right tabular-nums">{row.shows}</TableCell>
                <TableCell className="text-right tabular-nums">{row.episodes}</TableCell>
                <TableCell className="text-right tabular-nums">{watchedDuration(row.seconds_watched)}</TableCell>
                <TableCell className="text-sm text-muted-foreground">{formatDateTime(row.last_watched_at)}</TableCell>
                <TableCell className="text-right">
                  <Button asChild variant="ghost" size="icon">
                    <Link
                      to={`/admin/watching/${row.user_id}`}
                      aria-label={t('video.admin.watching.historyOf', { email: row.email })}
                    >
                      <HistoryIcon />
                    </Link>
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
