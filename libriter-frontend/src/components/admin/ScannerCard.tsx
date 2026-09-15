import { RefreshCwIcon } from 'lucide-react'
import { toast } from 'sonner'
import { useScannerStatus, useTriggerRescan } from '@/api/adminHooks'
import type { ScannerStatus } from '@/api/types'
import { ErrorState } from '@/components/ErrorState'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/format'

const TRIGGER_LABELS: Record<string, string> = {
  startup: 'start serveru',
  manual: 'ručně z administrace',
}

export function ScannerCard() {
  const status = useScannerStatus()
  const rescan = useTriggerRescan()

  function handleRescan() {
    rescan.mutate(undefined, {
      onSuccess: () => toast.success('Kontrola knihovny běží na pozadí.'),
      onError: (error) => toast.error(error.message),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Scanner</CardTitle>
        <CardDescription>
          Projde AUDIO_ROOT a doplní soubory, které v knihovně chybí. Nové soubory jinak zachytí
          sledování složky samo.
        </CardDescription>
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
          {status.data?.running ? 'Kontrola běží…' : 'Spustit kontrolu knihovny'}
        </Button>
      </CardContent>
    </Card>
  )
}

function ScannerDetails({ status }: { status: ScannerStatus }) {
  const rows: { label: string; value: React.ReactNode }[] = [
    {
      label: 'Stav',
      value: status.running ? (
        <Badge>Běží</Badge>
      ) : (
        <Badge variant="secondary">Nečinný</Badge>
      ),
    },
    { label: 'Spuštěno', value: TRIGGER_LABELS[status.trigger] ?? '–' },
    { label: 'Začátek', value: formatDateTime(status.started_at) },
    { label: 'Konec', value: status.running ? '–' : formatDateTime(status.finished_at) },
    {
      label: 'Soubory',
      value: `${status.processed} zpracováno, ${status.ingested} nově načteno`,
    },
    {
      label: 'Sledování složky',
      value: status.watcher_active ? 'aktivní' : 'neběží',
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
          Chyb při posledním průchodu: {status.errors}
          {status.last_error ? ` – ${status.last_error}` : ''}
        </p>
      ) : null}
    </div>
  )
}
