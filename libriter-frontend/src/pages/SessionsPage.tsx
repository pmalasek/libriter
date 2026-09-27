import { CheckCircle2Icon, HeadphonesIcon, PauseIcon, PlayIcon, RotateCcwIcon, Trash2Icon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
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
 * Úplný přehled rozposlouchaných poslechů. Úvodní stránka nabídne jen ten
 * nejbližší a nabídka v přehrávači jen řádku na poslech – tady je vidět, co je
 * kde rozečtené, a dá se to uklidit.
 */
export function SessionsPage() {
  const { t } = useTranslation()
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
        <PageHeader title={t('sessions.title')} />
        <LoadingGrid count={3} view="list" />
      </>
    )
  }

  if (sessions.isError) {
    return (
      <>
        <PageHeader title={t('sessions.title')} />
        <ErrorState error={sessions.error} onRetry={() => void sessions.refetch()} />
      </>
    )
  }

  const title = (session: PlaySession) => sessionTitle(session, bookById, seriesById)

  return (
    <>
      <PageHeader
        title={t('sessions.title')}
        description={
          open.length > 0
            ? t('sessions.inProgress', { count: open.length })
            : t('sessions.nothingInProgress')
        }
      />

      {open.length === 0 && finished.length === 0 ? (
        <EmptyState
          icon={HeadphonesIcon}
          title={t('sessions.emptyTitle')}
          description={t('sessions.emptyDescription')}
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
          <h2 className="mt-10 mb-3 font-heading text-lg font-semibold">{t('sessions.finished')}</h2>
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
            <AlertDialogTitle>{t('sessions.remove.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {toRemove
                ? t('sessions.remove.description', { title: title(toRemove) })
                : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (toRemove) player.removeSession(toRemove.id)
                setToRemove(null)
              }}
            >
              {t('sessions.remove.action')}
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
  const { t } = useTranslation()
  const player = usePlayer()
  const progress = sessionProgress(session)
  const currentBook = progress.bookId ? bookById.get(progress.bookId) : undefined

  // Otevřený poslech se odtud jen pozastaví a rozjede; načítat ho znovu ze
  // serveru by zahodilo pozici, kterou přehrávač právě drží.
  const isOpen = player.session?.id === session.id
  const isPlaying = isOpen && player.playing
  // Doposlechnutý poslech stojí na konci poslední kapitoly – „pokračovat“ by
  // přehrálo pár posledních sekund. Nabídne se proto poslech od začátku.
  const restart = Boolean(session.finished_at) && !isPlaying
  const label = isPlaying
    ? t('player.pause')
    : restart
      ? t('player.restart')
      : isOpen
        ? t('player.play')
        : t('player.resume')

  return (
    <Card className="@container">
      <CardContent className="flex flex-wrap items-center gap-x-4 gap-y-3">
        {books.length > 1 ? (
          <SeriesCoverStack books={books} />
        ) : currentBook ? (
          <Link to={`/books/${currentBook.id}`} className="shrink-0">
            <BookCover book={currentBook} key={currentBook.id} lift={false} className="size-14" />
          </Link>
        ) : (
          <div className="size-14 shrink-0 rounded-2xl bg-foreground/8" />
        )}

        {/* Text má přednost před tlačítky: když by mu zbylo míň než 13 rem,
            zalomí se tlačítka na vlastní řádek vpravo dole. Míň už být nesmí,
            jinak by se na úzkou kartu nevešel vedle obálky ani on. */}
        <div className="min-w-52 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate font-medium">{title}</p>
            <Badge variant="secondary">{sessionKindLabel(session)}</Badge>
            {session.finished_at ? (
              <Badge variant="outline">
                <CheckCircle2Icon />
                {t('sessions.finishedBadge')}
              </Badge>
            ) : null}
          </div>

          <p className="mt-1 truncate text-sm text-muted-foreground">
            {progress.bookCount > 1
              ? t('sessions.bookOf', { number: progress.bookNumber, count: progress.bookCount })
              : null}
            {progress.bookCount > 1 && currentBook ? ' · ' : null}
            {currentBook ? (
              <Link to={`/books/${currentBook.id}`} className="hover:underline">
                {currentBook.title}
              </Link>
            ) : (
              t('sessions.bookMissing')
            )}
            {progress.positionSeconds > 0 ? ` · ${formatClock(progress.positionSeconds)}` : null}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t('sessions.lastPlayed', { date: formatDateTime(session.updated_at) })}
          </p>
        </div>

        <div className="ml-auto flex shrink-0 items-center gap-2">
          <Button
            onClick={() =>
              restart
                ? player.switchSession(session.id, { fromStart: true })
                : isOpen
                  ? player.toggle()
                  : player.switchSession(session.id)
            }
            disabled={player.loading}
            aria-label={label}
          >
            {isPlaying ? <PauseIcon /> : restart ? <RotateCcwIcon /> : <PlayIcon />}
            {/* Na úzké kartě mluví ikona sama za sebe. */}
            <span className="hidden @sm:inline">{label}</span>
          </Button>
          <Button variant="ghost" size="icon" onClick={onRemove} aria-label={t('sessions.remove.labelNamed', { title })}>
            <Trash2Icon />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
