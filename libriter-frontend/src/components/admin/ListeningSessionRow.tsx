import { CheckCircle2Icon } from 'lucide-react'
import { Link } from 'react-router'
import type { Book, PlaySession, Series } from '@/api/types'
import { BookCover } from '@/components/BookCover'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { formatClock, formatDateTime } from '@/lib/format'
import {
  sessionBooks,
  sessionKindLabel,
  sessionProgress,
  sessionTitle,
} from '@/player/sessionLabels'

/**
 * Poslech cizího uživatele v administraci – stejné informace jako na stránce
 * Právě posloucháno, ale bez ovládání. Admin cizí poslech nepřehrává ani
 * nemaže, jen se dívá.
 */
export function ListeningSessionRow({
  session,
  bookById,
  seriesById,
}: {
  session: PlaySession
  bookById: Map<string, Book>
  seriesById: Map<string, Series>
}) {
  const title = sessionTitle(session, bookById, seriesById)
  const books = sessionBooks(session, bookById)
  const progress = sessionProgress(session)
  const currentBook = progress.bookId ? bookById.get(progress.bookId) : undefined

  return (
    <Card>
      <CardContent className="flex flex-wrap items-center gap-4">
        {books.length > 1 ? (
          <SeriesCoverStack books={books} />
        ) : currentBook ? (
          <Link to={`/books/${currentBook.id}`} className="shrink-0">
            <BookCover book={currentBook} key={currentBook.id} lift={false} className="size-14" />
          </Link>
        ) : (
          <div className="size-14 shrink-0 rounded-2xl bg-muted" />
        )}

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate font-medium">{title}</p>
            <Badge variant="secondary">{sessionKindLabel(session)}</Badge>
            {session.finished_at ? (
              <Badge variant="outline">
                <CheckCircle2Icon />
                Doposlechnuto
              </Badge>
            ) : null}
          </div>

          <p className="mt-1 truncate text-sm text-muted-foreground">
            {progress.bookCount > 1 ? `Kniha ${progress.bookNumber} z ${progress.bookCount}` : null}
            {progress.bookCount > 1 && currentBook ? ' · ' : null}
            {currentBook ? (
              <Link to={`/books/${currentBook.id}`} className="hover:underline">
                {currentBook.title}
              </Link>
            ) : (
              'Kniha už není v knihovně'
            )}
            {progress.positionSeconds > 0 ? ` · ${formatClock(progress.positionSeconds)}` : null}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Naposledy {formatDateTime(session.updated_at)}
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
