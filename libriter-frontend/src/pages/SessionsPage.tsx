import { CheckCircle2Icon, HeadphonesIcon, PauseIcon, PlayIcon, Trash2Icon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { useBooks, useSeriesById, useSessions } from '@/api/hooks'
import type { Book, PlaySession } from '@/api/types'
import { BookCover } from '@/components/BookCover'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { formatClock, formatDateTime } from '@/lib/format'
import { usePlayer } from '@/player/playerContext'
import {
  sessionBooks,
  sessionKindLabel,
  sessionProgress,
  sessionTitle,
} from '@/player/sessionLabels'

/**
 * Přehled rozposlouchaných poslechů. Lišta přehrávače umí přepínat mezi nimi
 * taky, ale ta ukazuje jen jednu řádku na poslech – tady je vidět, co je kde
 * rozečtené, a dá se to uklidit.
 */
export function SessionsPage() {
  const sessions = useSessions()
  const books = useBooks()
  const { map: seriesById } = useSeriesById()
  const [toRemove, setToRemove] = useState<PlaySession | null>(null)
  const player = usePlayer()

  const bookById = useMemo(
    () => new Map((books.data ?? []).map((book) => [book.id, book])),
    [books.data],
  )

  const { open, finished } = useMemo(() => {
    const all = sessions.data ?? []
    return {
      open: all.filter((session) => !session.finished_at),
      finished: all.filter((session) => session.finished_at),
    }
  }, [sessions.data])

  if (sessions.isPending) {
    return (
      <>
        <PageHeader title="Poslechy" />
        <LoadingGrid count={3} view="list" />
      </>
    )
  }

  if (sessions.isError) {
    return (
      <>
        <PageHeader title="Poslechy" />
        <ErrorState error={sessions.error} onRetry={() => void sessions.refetch()} />
      </>
    )
  }

  const title = (session: PlaySession) => sessionTitle(session, bookById, seriesById)

  return (
    <>
      <PageHeader
        title="Poslechy"
        description={
          open.length > 0
            ? `${open.length} rozposlouchaných · pokračujte tam, kde jste skončili`
            : 'Zatím nic rozposlouchaného'
        }
      />

      {open.length === 0 && finished.length === 0 ? (
        <EmptyState
          icon={HeadphonesIcon}
          title="Zatím nic neposloucháte"
          description="Spusťte knihu tlačítkem Přehrát v jejím detailu, celou sérii u série, nebo si vyberte víc knih naráz v seznamu knih."
        />
      ) : null}

      <div className="space-y-3">
        {open.map((session) => (
          <SessionCard
            key={session.id}
            session={session}
            title={title(session)}
            books={sessionBooks(session, bookById)}
            bookById={bookById}
            onRemove={() => setToRemove(session)}
          />
        ))}
      </div>

      {finished.length > 0 ? (
        <>
          <h2 className="mt-10 mb-3 font-heading text-lg font-semibold">Doposlechnuté</h2>
          <div className="space-y-3">
            {finished.map((session) => (
              <SessionCard
                key={session.id}
                session={session}
                title={title(session)}
                books={sessionBooks(session, bookById)}
                bookById={bookById}
                onRemove={() => setToRemove(session)}
              />
            ))}
          </div>
        </>
      ) : null}

      <AlertDialog open={toRemove !== null} onOpenChange={(next) => !next && setToRemove(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Odebrat poslech?</AlertDialogTitle>
            <AlertDialogDescription>
              {toRemove
                ? `„${title(toRemove)}“ zmizí ze seznamu včetně uložené pozice. Knihy v knihovně zůstanou.`
                : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Zrušit</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (toRemove) player.removeSession(toRemove.id)
                setToRemove(null)
              }}
            >
              Odebrat
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function SessionCard({
  session,
  title,
  books,
  bookById,
  onRemove,
}: {
  session: PlaySession
  title: string
  books: Book[]
  bookById: Map<string, Book>
  onRemove: () => void
}) {
  const player = usePlayer()
  const progress = sessionProgress(session)
  const currentBook = progress.bookId ? bookById.get(progress.bookId) : undefined

  // Otevřený poslech se odtud jen pozastaví a rozjede; načítat ho znovu ze
  // serveru by zahodilo pozici, kterou přehrávač právě drží.
  const isOpen = player.session?.id === session.id
  const isPlaying = isOpen && player.playing

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

        <div className="flex shrink-0 items-center gap-2">
          <Button
            onClick={() => (isOpen ? player.toggle() : player.switchSession(session.id))}
            disabled={player.loading}
          >
            {isPlaying ? <PauseIcon /> : <PlayIcon />}
            {isPlaying ? 'Pozastavit' : isOpen ? 'Přehrát' : 'Pokračovat'}
          </Button>
          <Button variant="ghost" size="icon" onClick={onRemove} aria-label={`Odebrat poslech ${title}`}>
            <Trash2Icon />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
