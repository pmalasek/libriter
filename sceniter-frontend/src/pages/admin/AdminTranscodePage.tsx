import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatDateTime } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from 'core-web/src/components/ui/table'
import { useAdminAction, useAdminTranscodes } from '@/api/hooks'

export function AdminTranscodePage() {
  const { t } = useTranslation()
  const sessions = useAdminTranscodes()
  const action = useAdminAction()
  if (sessions.isError) return <ErrorState error={sessions.error} onRetry={() => void sessions.refetch()} />
  if (!sessions.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  if (sessions.data.length === 0) return <EmptyState title={t('video.admin.transcode.empty')} />

  return (
    <div className="glass inset-shadow-glass overflow-x-auto rounded-3xl p-2 ring-1 ring-glass-edge">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('video.admin.transcode.mode')}</TableHead>
            <TableHead>{t('video.admin.transcode.started')}</TableHead>
            <TableHead>{t('video.admin.transcode.lastAccess')}</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          {sessions.data.map((s) => (
            <TableRow key={s.id}>
              <TableCell>
                {t(`video.player.mode.${s.mode}`)}
                {s.error ? <p className="text-xs text-destructive">{s.error}</p> : null}
              </TableCell>
              <TableCell>{formatDateTime(s.started_at)}</TableCell>
              <TableCell>{formatDateTime(s.last_access)}</TableCell>
              <TableCell className="text-right">
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() =>
                    action.mutate(
                      { path: `/admin/transcode/${s.id}`, method: 'DELETE' },
                      { onSuccess: () => toast.success(t('video.admin.transcode.stopped')), onError: (e) => toast.error(e.message) },
                    )
                  }
                >
                  {t('video.admin.transcode.stop')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
