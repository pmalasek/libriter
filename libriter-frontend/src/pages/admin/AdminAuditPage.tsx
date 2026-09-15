import { useAuditLog } from '@/api/adminHooks'
import { AUDIT_ACTION_LABELS, type AuditEntry } from '@/api/types'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatDateTime } from '@/lib/format'

export function AdminAuditPage() {
  const audit = useAuditLog()

  if (audit.isPending) return <LoadingList count={5} />
  if (audit.error) return <ErrorState error={audit.error} onRetry={() => void audit.refetch()} />

  const entries = audit.data.pages.flatMap((page) => page.items)
  if (entries.length === 0) {
    return (
      <EmptyState
        title="Zatím žádné záznamy"
        description="Sem se zapisují zásahy administrátora – změny rolí, mazání a úpravy nastavení."
      />
    )
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-40">Čas</TableHead>
              <TableHead className="w-52">Uživatel</TableHead>
              <TableHead className="w-52">Akce</TableHead>
              <TableHead>Cíl</TableHead>
              <TableHead>Detaily</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => (
              <TableRow key={entry.id}>
                <TableCell className="text-sm text-muted-foreground">
                  {formatDateTime(entry.created_at)}
                </TableCell>
                <TableCell className="break-all text-sm">{entry.actor_email || '–'}</TableCell>
                <TableCell className="text-sm">
                  {AUDIT_ACTION_LABELS[entry.action] ?? entry.action}
                </TableCell>
                <TableCell className="break-all text-sm">{targetLabel(entry)}</TableCell>
                <TableCell>
                  <Details details={entry.details} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {audit.hasNextPage ? (
        <div className="flex justify-center">
          <Button
            variant="outline"
            onClick={() => void audit.fetchNextPage()}
            disabled={audit.isFetchingNextPage}
          >
            {audit.isFetchingNextPage ? 'Načítám…' : 'Načíst další'}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

/** Popisek cíle; u starších záznamů může chybět, pak padne zpět na ID. */
function targetLabel(entry: AuditEntry): string {
  return entry.target_label || entry.target_id || '–'
}

function Details({ details }: { details: Record<string, unknown> }) {
  const pairs = Object.entries(details ?? {})
  if (pairs.length === 0) return <span className="text-sm text-muted-foreground">–</span>

  return (
    <ul className="space-y-0.5 text-xs text-muted-foreground">
      {pairs.map(([key, value]) => (
        <li key={key} className="break-all">
          <span className="font-mono">{key}</span>: {formatValue(value)}
        </li>
      ))}
    </ul>
  )
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return '–'
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'boolean') return value ? 'ano' : 'ne'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}
