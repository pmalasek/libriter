import { ArrowLeftIcon, CheckCircle2Icon, HeadphonesIcon } from 'lucide-react'
import { useMemo } from 'react'
import { Link, useParams } from 'react-router'
import { useListeningUser } from '@/api/adminHooks'
import { useBooks, useSeriesById } from '@/api/hooks'
import {
  ROLE_LABELS,
  type Book,
  type ListeningDay,
  type PlaySession,
  type Series,
} from '@/api/types'
import { ListeningSessionRow } from '@/components/admin/ListeningSessionRow'
import { BookCover } from '@/components/BookCover'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatDate, formatDateTime, formatDuration } from '@/lib/format'

/** Den deníku s celkovým časem a knihami, které v něm hrály. */
interface DaySummary {
  day: string
  total: number
  books: { bookId: string; seconds: number }[]
}

/** Deník po dnech: řádky přijdou po knihách, tabulka je chce po dnech. */
function groupByDay(days: ListeningDay[]): DaySummary[] {
  const byDay = new Map<string, DaySummary>()
  for (const entry of days) {
    const summary = byDay.get(entry.day) ?? { day: entry.day, total: 0, books: [] }
    summary.total += entry.seconds_listened
    summary.books.push({ bookId: entry.book_id, seconds: entry.seconds_listened })
    byDay.set(entry.day, summary)
  }
  return [...byDay.values()]
}

/** Doba poslechu; nula by se jinak vypsala jako „neznámá délka“. */
function duration(seconds: number): string {
  return seconds > 0 ? formatDuration(seconds) : '–'
}

/**
 * Poslechy jednoho uživatele: co má rozposlouchané, které knihy už slyšel a
 * kolik času u nich strávil. Čte se jen – admin cizí poslech nemění.
 */
export function AdminListeningUserPage() {
  const { userId = '' } = useParams()
  const detail = useListeningUser(userId)
  const books = useBooks()
  const { map: seriesById } = useSeriesById()

  const bookById = useMemo(
    () => new Map((books.data ?? []).map((book) => [book.id, book])),
    [books.data],
  )

  const sessions = detail.data?.sessions ?? []
  const open = sessions.filter((session) => !session.finished_at)
  const finished = sessions.filter((session) => session.finished_at)

  // Součty deníku po knihách se spojí se stavem knihy, ať je v jedné tabulce
  // vidět „slyšel“ i „kolik toho odposlouchal“.
  const bookRows = useMemo(() => {
    if (!detail.data) return []
    const totals = new Map(detail.data.books.map((total) => [total.book_id, total]))
    const ids = new Set([
      ...detail.data.books.map((total) => total.book_id),
      ...detail.data.progress.map((p) => p.book_id),
    ])
    return [...ids]
      .map((bookId) => ({
        bookId,
        book: bookById.get(bookId),
        seconds: totals.get(bookId)?.seconds_listened ?? 0,
        lastAt: totals.get(bookId)?.last_at,
        finishedAt: detail.data.progress.find((p) => p.book_id === bookId)?.finished_at,
        tracked: detail.data.progress.some((p) => p.book_id === bookId),
      }))
      .sort((a, b) => (b.lastAt ?? '').localeCompare(a.lastAt ?? ''))
  }, [bookById, detail.data])

  const dayRows = useMemo(() => groupByDay(detail.data?.days ?? []), [detail.data])

  const backLink = (
    <Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
      <Link to="/admin/listening">
        <ArrowLeftIcon />
        Zpět na poslechy
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
        <Badge variant="secondary">{ROLE_LABELS[user.role]}</Badge>
      </div>

      <SessionSection
        title="Rozposlouchané"
        empty="Nic rozposlouchaného."
        sessions={open}
        bookById={bookById}
        seriesById={seriesById}
      />
      <SessionSection
        title="Doposlechnuté poslechy"
        empty="Zatím nic doposlechnutého."
        sessions={finished}
        bookById={bookById}
        seriesById={seriesById}
      />

      <h3 className="font-heading mt-8 mb-3 text-lg font-semibold">Knihy</h3>
      {bookRows.length === 0 ? (
        <p className="text-sm text-muted-foreground">Tenhle účet zatím nic neposlouchal.</p>
      ) : (
        <div className="rounded-xl border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Kniha</TableHead>
                <TableHead className="w-40">Stav</TableHead>
                <TableHead className="w-32 text-right">Odposloucháno</TableHead>
                <TableHead className="w-40">Naposledy</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {bookRows.map((row) => (
                <TableRow key={row.bookId}>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      {row.book ? (
                        <BookCover
                          book={row.book}
                          key={row.book.id}
                          lift={false}
                          className="size-10 shrink-0 rounded-lg"
                        />
                      ) : (
                        <div className="size-10 shrink-0 rounded-lg bg-muted" />
                      )}
                      <BookTitle book={row.book} />
                    </div>
                  </TableCell>
                  <TableCell>
                    {row.finishedAt ? (
                      <Badge variant="highlight">
                        <CheckCircle2Icon />
                        Doposlechnuto
                      </Badge>
                    ) : row.tracked ? (
                      <Badge variant="secondary">
                        <HeadphonesIcon />
                        Rozposlouchané
                      </Badge>
                    ) : (
                      <span className="text-sm text-muted-foreground">Jen v deníku</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{duration(row.seconds)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatDateTime(row.lastAt)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <h3 className="font-heading mt-8 mb-3 text-lg font-semibold">Poslední dny</h3>
      {dayRows.length === 0 ? (
        <p className="text-sm text-muted-foreground">Za posledních 90 dní žádný poslech.</p>
      ) : (
        <div className="rounded-xl border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-44">Den</TableHead>
                <TableHead className="w-32 text-right">Celkem</TableHead>
                <TableHead>Knihy</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {dayRows.map((row) => (
                <TableRow key={row.day}>
                  <TableCell className="text-sm">{formatDate(row.day)}</TableCell>
                  <TableCell className="text-right tabular-nums">{duration(row.total)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {row.books
                      .map((entry) => {
                        const title = bookById.get(entry.bookId)?.title ?? 'Smazaná kniha'
                        return `${title} (${duration(entry.seconds)})`
                      })
                      .join(', ')}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

function BookTitle({ book }: { book: Book | undefined }) {
  if (!book) {
    return <span className="text-sm text-muted-foreground">Kniha už není v knihovně</span>
  }
  return (
    <Link
      to={`/books/${book.id}`}
      className="min-w-0 truncate text-sm font-medium text-primary underline-offset-4 hover:underline"
    >
      {book.title}
    </Link>
  )
}

function SessionSection({
  title,
  empty,
  sessions,
  bookById,
  seriesById,
}: {
  title: string
  empty: string
  sessions: PlaySession[]
  bookById: Map<string, Book>
  seriesById: Map<string, Series>
}) {
  return (
    <>
      <h3 className="font-heading mt-6 mb-3 text-lg font-semibold">{title}</h3>
      {sessions.length === 0 ? (
        <p className="text-sm text-muted-foreground">{empty}</p>
      ) : (
        <div className="space-y-3">
          {sessions.map((session) => (
            <ListeningSessionRow
              key={session.id}
              session={session}
              bookById={bookById}
              seriesById={seriesById}
            />
          ))}
        </div>
      )}
    </>
  )
}
