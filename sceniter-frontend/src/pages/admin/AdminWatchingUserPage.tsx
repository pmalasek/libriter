import { ArrowLeftIcon } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams, useSearchParams } from 'react-router'
import {
  currentLanguage,
  episodeLabel,
  formatDate,
  formatDuration,
  roleLabel,
  type WatchHistoryEntry,
} from 'sceniter-shared'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { LoadingList } from 'core-web/src/components/LoadingList'
import { Badge } from 'core-web/src/components/ui/badge'
import { Button } from 'core-web/src/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from 'core-web/src/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from 'core-web/src/components/ui/table'
import { useAdminWatchingUser } from '@/api/hooks'
import { WatchingSinceNote } from './AdminWatchingPage'

/** Nabízená období; server přijme 1–365 dní. */
const PERIODS = [30, 90, 365] as const
type Period = (typeof PERIODS)[number]
const DEFAULT_PERIOD: Period = 90

function parsePeriod(raw: string | null): Period {
  const n = Number(raw)
  return (PERIODS as readonly number[]).includes(n) ? (n as Period) : DEFAULT_PERIOD
}

/** Den historie s celkovým časem a tituly, které v něm běžely. */
interface DaySummary {
  day: string
  total: number
  entries: WatchHistoryEntry[]
}

/** Řádky přijdou po titulech a dnech (od nejnovějšího); tabulka je chce po dnech. */
function groupByDay(entries: WatchHistoryEntry[]): DaySummary[] {
  const byDay = new Map<string, DaySummary>()
  for (const entry of entries) {
    const summary = byDay.get(entry.day) ?? { day: entry.day, total: 0, entries: [] }
    summary.total += entry.seconds
    summary.entries.push(entry)
    byDay.set(entry.day, summary)
  }
  return [...byDay.values()]
}

/** Doba sledování; nula by se jinak vypsala jako „neznámá délka“. */
function duration(seconds: number): string {
  return seconds > 0 ? formatDuration(seconds) : '–'
}

/**
 * Den je datum v UTC („2026-11-27“). Jako půlnoc UTC by se v časových
 * pásmech západně od Greenwiche ukázal předchozí den, proto poledne.
 */
function formatDay(day: string): string {
  return formatDate(`${day}T12:00:00`)
}

let timeFormatter: Intl.DateTimeFormat | null = null
let timeFormatterLocale = ''

/** Čas v rámci dne (14:05) v jazyce rozhraní. */
function formatTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '–'
  const locale = currentLanguage()
  if (!timeFormatter || timeFormatterLocale !== locale) {
    timeFormatter = new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit' })
    timeFormatterLocale = locale
  }
  return timeFormatter.format(date)
}

/**
 * Historie sledování jednoho účtu po dnech: co, kdy a jak dlouho. Jen čtení –
 * admin cizí rozkoukanost nemění.
 */
export function AdminWatchingUserPage() {
  const { t } = useTranslation()
  const { userId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const period = parsePeriod(searchParams.get('days'))
  const detail = useAdminWatchingUser(userId, period)

  const days = useMemo(() => groupByDay(detail.data?.entries ?? []), [detail.data])
  const periodTotal = days.reduce((sum, day) => sum + day.total, 0)

  const backLink = (
    <Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
      <Link to="/admin/watching">
        <ArrowLeftIcon />
        {t('video.admin.watching.back')}
      </Link>
    </Button>
  )

  if (detail.isPending) {
    return (
      <div>
        {backLink}
        <LoadingList count={4} />
      </div>
    )
  }
  if (detail.error) {
    return (
      <div>
        {backLink}
        <ErrorState error={detail.error} onRetry={() => void detail.refetch()} />
      </div>
    )
  }

  const { user } = detail.data

  return (
    <div>
      {backLink}

      <div className="mb-6 flex flex-wrap items-center gap-3">
        <div className="min-w-0">
          <h2 className="font-heading text-xl font-bold">{user.display_name}</h2>
          <p className="break-all text-sm text-muted-foreground">{user.email}</p>
        </div>
        <Badge variant="secondary">{roleLabel(user.role)}</Badge>
        <div className="ml-auto">
          <Select
            value={String(period)}
            onValueChange={(value) =>
              setSearchParams(
                (prev) => {
                  const next = new URLSearchParams(prev)
                  if (Number(value) === DEFAULT_PERIOD) next.delete('days')
                  else next.set('days', value)
                  return next
                },
                { replace: true },
              )
            }
          >
            <SelectTrigger className="w-48" aria-label={t('video.admin.watching.period')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PERIODS.map((p) => (
                <SelectItem key={p} value={String(p)}>
                  {t(`video.admin.watching.periods.d${p}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {days.length === 0 ? (
        <div className="space-y-2">
          <p className="text-sm">{t('video.admin.watching.noHistory')}</p>
          <WatchingSinceNote />
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground tabular-nums">
            {t('video.admin.watching.periodTotal', { duration: duration(periodTotal) })}
          </p>
          <div className="rounded-xl border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-44">{t('video.admin.watching.day')}</TableHead>
                  <TableHead className="w-28 text-right">{t('video.admin.watching.total')}</TableHead>
                  <TableHead>{t('video.admin.watching.titles')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {days.map((day) => (
                  <TableRow key={day.day} className="align-top">
                    <TableCell className="text-sm">{formatDay(day.day)}</TableCell>
                    <TableCell className="text-right tabular-nums">{duration(day.total)}</TableCell>
                    <TableCell>
                      <ul className="space-y-1.5">
                        {day.entries.map((entry) => (
                          <EntryLine key={entry.movie_id ?? entry.episode_id} entry={entry} />
                        ))}
                      </ul>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <WatchingSinceNote />
        </div>
      )}
    </div>
  )
}

/** Jeden titul v řádku dne: odkaz na detail, odkoukaný čas a rozsah hodin. */
function EntryLine({ entry }: { entry: WatchHistoryEntry }) {
  const { t } = useTranslation()
  const link = titleLink(entry)
  const label = titleLabel(entry) ?? t('video.admin.watching.unknownTitle')

  return (
    <li className="flex flex-wrap items-baseline gap-x-2 text-sm">
      {link ? (
        <Link to={link} className="min-w-0 font-medium text-primary underline-offset-4 hover:underline">
          {label}
        </Link>
      ) : (
        <span className="min-w-0 font-medium">{label}</span>
      )}
      {entry.kind === 'episode' && entry.season != null && entry.episode != null ? (
        <span className="text-muted-foreground">
          {episodeLabel({ season: entry.season, number: entry.episode, title: entry.episode_title })}
        </span>
      ) : null}
      <span className="text-muted-foreground tabular-nums">
        {duration(entry.seconds)} ·{' '}
        {t('video.admin.watching.timeRange', { from: formatTime(entry.first_at), to: formatTime(entry.last_at) })}
      </span>
    </li>
  )
}

/** Film „Název (rok)“, u dílu název seriálu. */
function titleLabel(entry: WatchHistoryEntry): string | undefined {
  if (entry.kind === 'episode') return entry.show_title
  if (!entry.movie_title) return undefined
  return entry.movie_year ? `${entry.movie_title} (${entry.movie_year})` : entry.movie_title
}

function titleLink(entry: WatchHistoryEntry): string | undefined {
  if (entry.kind === 'movie') return entry.movie_id ? `/movies/${entry.movie_id}` : undefined
  return entry.show_id ? `/shows/${entry.show_id}` : undefined
}
