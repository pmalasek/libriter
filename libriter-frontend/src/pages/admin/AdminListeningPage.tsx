import { HeadphonesIcon } from 'lucide-react'
import { Link } from 'react-router'
import { useListeningOverview } from '@/api/adminHooks'
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
import { formatDateTime, formatDuration } from '@/lib/format'

/**
 * Přehled poslechu všech účtů. Odposlouchaný čas se počítá ze skutečně
 * přehraného obsahu, takže při zrychleném poslechu je vyšší než čas strávený
 * u sluchátek.
 */
export function AdminListeningPage() {
  const overview = useListeningOverview()

  if (overview.isPending) return <LoadingList count={4} />
  if (overview.error) {
    return <ErrorState error={overview.error} onRetry={() => void overview.refetch()} />
  }
  if (overview.data.length === 0) {
    return <EmptyState title="Žádní uživatelé" icon={HeadphonesIcon} />
  }

  return (
    <div className="rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Uživatel</TableHead>
            <TableHead className="w-32 text-right">Rozposlouchané</TableHead>
            <TableHead className="w-32 text-right">Doposlechnuté</TableHead>
            <TableHead className="w-32 text-right">Slyšené knihy</TableHead>
            <TableHead className="w-32 text-right">Odposloucháno</TableHead>
            <TableHead className="w-40">Naposledy</TableHead>
            <TableHead className="w-16 text-right">Detail</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {overview.data.map((row) => (
            <TableRow key={row.user_id}>
              <TableCell>
                <Link
                  to={`/admin/listening/${row.user_id}`}
                  className="font-medium text-primary underline-offset-4 hover:underline"
                >
                  {row.display_name}
                </Link>
                <p className="break-all text-xs text-muted-foreground">{row.email}</p>
              </TableCell>
              <TableCell className="text-right tabular-nums">{row.open_sessions}</TableCell>
              <TableCell className="text-right tabular-nums">{row.finished_sessions}</TableCell>
              <TableCell className="text-right tabular-nums">{row.finished_books}</TableCell>
              {/* formatDuration hlásí u nuly „neznámá délka“ – nikdy
                  neposlouchaný účet má mít pomlčku. */}
              <TableCell className="text-right tabular-nums">
                {row.seconds_listened > 0 ? formatDuration(row.seconds_listened) : '–'}
              </TableCell>
              <TableCell className="text-sm text-muted-foreground">
                {formatDateTime(row.last_listened_at)}
              </TableCell>
              <TableCell className="text-right">
                <Button asChild variant="ghost" size="icon">
                  <Link
                    to={`/admin/listening/${row.user_id}`}
                    aria-label={`Poslechy uživatele ${row.email}`}
                  >
                    <HeadphonesIcon />
                  </Link>
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
