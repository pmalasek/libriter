import { CopyIcon, RefreshCwIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { useScannerStatus, useTriggerRescan } from '@/api/adminHooks'
import type { ScannerStatus, ScannerSuspect } from '@/api/types'
import { ErrorState } from '@/components/ErrorState'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/format'

const TRIGGER_LABELS: Partial<
  Record<string, 'admin.scanner.trigger.startup' | 'admin.scanner.trigger.manual'>
> = {
  startup: 'admin.scanner.trigger.startup',
  manual: 'admin.scanner.trigger.manual',
}

export function ScannerCard() {
  const { t } = useTranslation()
  const status = useScannerStatus()
  const rescan = useTriggerRescan()

  function handleRescan() {
    rescan.mutate(undefined, {
      onSuccess: () => toast.success(t('admin.scanner.started')),
      onError: (error) => toast.error(error.message),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.scanner.title')}</CardTitle>
        <CardDescription>{t('admin.scanner.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {status.isPending ? <Skeleton className="h-24 w-full rounded-lg" /> : null}
        {status.error ? (
          <ErrorState error={status.error} onRetry={() => void status.refetch()} />
        ) : null}
        {status.data ? <ScannerDetails status={status.data} /> : null}

        <Button
          onClick={handleRescan}
          disabled={rescan.isPending || status.data?.running === true}
        >
          <RefreshCwIcon />
          {status.data?.running ? t('admin.scanner.running') : t('admin.scanner.start')}
        </Button>
      </CardContent>
    </Card>
  )
}

function ScannerDetails({ status }: { status: ScannerStatus }) {
  const { t } = useTranslation()
  const trigger = TRIGGER_LABELS[status.trigger]
  const rows: { label: string; value: React.ReactNode }[] = [
    {
      label: t('admin.scanner.status'),
      value: status.running ? (
        <Badge>{t('admin.scanner.statusRunning')}</Badge>
      ) : (
        <Badge variant="secondary">{t('admin.scanner.statusIdle')}</Badge>
      ),
    },
    { label: t('admin.scanner.trigger.label'), value: trigger ? t(trigger) : '–' },
    { label: t('admin.scanner.startedAt'), value: formatDateTime(status.started_at) },
    {
      label: t('admin.scanner.finishedAt'),
      value: status.running ? '–' : formatDateTime(status.finished_at),
    },
    {
      label: t('admin.scanner.files'),
      value: t('admin.scanner.filesValue', {
        processed: status.processed,
        ingested: status.ingested,
      }),
    },
    {
      label: t('admin.scanner.watcher'),
      value: status.watcher_active
        ? t('admin.scanner.watcherActive')
        : t('admin.scanner.watcherInactive'),
    },
  ]

  return (
    <div className="space-y-3">
      <dl className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-[10rem_1fr]">
        {rows.map((row) => (
          <div key={row.label} className="sm:contents">
            <dt className="text-sm text-muted-foreground">{row.label}</dt>
            <dd className="text-sm">{row.value}</dd>
          </div>
        ))}
      </dl>

      {status.errors > 0 ? (
        <p className="text-sm text-destructive">
          {t('admin.scanner.errors', { n: status.errors })}
          {status.last_error ? ` – ${status.last_error}` : ''}
        </p>
      ) : null}

      {status.suspects?.length ? <SuspectList suspects={status.suspects} /> : null}
    </div>
  )
}

/**
 * Knihy, které scanner založil, přestože v knihovně nejspíš už jsou. Zabránit
 * tomu nemůže – běží bez obsluhy a nemá se koho zeptat –, tak aspoň upozorní.
 */
function SuspectList({ suspects }: { suspects: ScannerSuspect[] }) {
  const { t } = useTranslation()

  return (
    <div className="rounded-lg border border-highlight/50 bg-highlight/10 p-3">
      <p className="flex items-start gap-2 text-sm font-medium">
        <CopyIcon className="mt-0.5 size-4 shrink-0" />
        <span>{t('admin.scanner.suspectsTitle')}</span>
      </p>
      <ul className="mt-2 space-y-2 pl-6">
        {suspects.map((suspect) => (
          <li key={suspect.book_id} className="text-sm">
            <Link to={`/books/${suspect.book_id}`} className="underline underline-offset-2">
              {suspect.title}
            </Link>
            <p className="font-mono text-xs text-muted-foreground">
              {t('admin.scanner.suspectNew', { path: suspect.file_path })}
            </p>
            <p className="font-mono text-xs text-muted-foreground">
              {t('admin.scanner.suspectExisting', { path: suspect.existing_path })}
            </p>
          </li>
        ))}
      </ul>
    </div>
  )
}
