import {
  ExternalLinkIcon,
  HeadphonesIcon,
  LibraryIcon,
  PauseIcon,
  PlayIcon,
  SmartphoneIcon,
} from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import {
  useBookPage,
  useBookProgress,
  useBooksByIds,
  useSeriesByIds,
  useSeriesPage,
  useSessions,
} from '@/api/hooks'
import type { BookSummary, PlaySession } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { BookCard } from '@/components/BookCard'
import { BookCover, coverUrl } from '@/components/BookCover'
import { EmptyState } from '@/components/EmptyState'
import { LoadingGrid } from '@/components/LoadingGrid'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Shelf, ShelfItem } from '@/components/Shelf'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { authorsLabel, bookCount, formatClock } from '@/lib/format'
import { usePlayer } from '@/player/playerContext'
import { sessionBooks, sessionKindLabel, sessionProgress, sessionTitle } from '@/player/sessionLabels'

/** Kolik položek se vejde do police, než začne být rolování únavné. */
const SHELF_SIZE = 12

/** Uzavřené testování mobilní aplikace na Google Play. */
const ANDROID_TESTING_URL = 'https://play.google.com/apps/testing/com.libriter.app'

/**
 * První pohled po otevření aplikace: čím se dá pokračovat a co v knihovně
 * stojí za pozornost. Úplný přehled poslechů zůstává na stránce Právě
 * posloucháno, tady je jen ten nejbližší.
 */
export function HomePage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const sessions = useSessions()
  const progress = useBookProgress()
  // Police jsou první stránky výpisů – celá knihovna se kvůli nim nestahuje.
  const newestPage = useBookPage({ sort: 'added', dir: 'desc' }, SHELF_SIZE)
  const seriesPage = useSeriesPage({}, SHELF_SIZE)

  // Rozposlouchané poslechy od naposledy otevřeného; ten první jde do hlavičky.
  const open = useMemo(
    () =>
      (sessions.data ?? [])
        .filter((session) => !session.finished_at)
        .sort((a, b) => b.updated_at.localeCompare(a.updated_at)),
    [sessions.data],
  )

  const finishedIds = useMemo(
    () =>
      [...progress.map.values()]
        .filter((item) => item.finished_at)
        .sort((a, b) => (b.finished_at ?? '').localeCompare(a.finished_at ?? ''))
        .slice(0, SHELF_SIZE)
        .map((item) => item.book_id),
    [progress.map],
  )

  // Knihy, které stránka ukazuje mimo police: poslech v hlavičce, aktuální
  // knihy ostatních poslechů a doposlechnuté. Stahují se jen tyhle.
  const neededIds = useMemo(
    () => [
      ...(open[0]?.items.map((item) => item.book_id) ?? []),
      ...open.slice(1).map((session) => sessionProgress(session).bookId),
      ...finishedIds,
    ],
    [open, finishedIds],
  )
  const { map: bookById } = useBooksByIds(neededIds)
  const seriesIds = useMemo(
    () => open.filter((session) => session.kind === 'series').map((session) => session.source_id),
    [open],
  )
  const { map: seriesById } = useSeriesByIds(seriesIds)

  const newest = newestPage.data?.items ?? []
  const libraryCount = newestPage.data?.total ?? 0

  // Knihy z ostatních rozposlouchaných poslechů – ta z hlavičky se neopakuje.
  const continues = useMemo(() => {
    const seen = new Set<string>()
    const list: BookSummary[] = []
    for (const session of open.slice(1)) {
      const current = sessionProgress(session).bookId
      const book = current ? bookById.get(current) : undefined
      if (!book || seen.has(book.id)) continue
      seen.add(book.id)
      list.push(book)
    }
    return list
  }, [open, bookById])

  const finished = useMemo(
    () => finishedIds.flatMap((id) => bookById.get(id) ?? []),
    [finishedIds, bookById],
  )

  const series = seriesPage.data?.items ?? []

  if (sessions.isPending || newestPage.isPending) {
    return (
      <>
        <Skeleton className="h-64 w-full rounded-3xl" />
        <div className="mt-10">
          <LoadingGrid count={6} view="small" />
        </div>
      </>
    )
  }

  const greeting = user?.display_name
    ? t('layout.home.greeting', { name: user.display_name })
    : t('layout.home.greetingAnonymous')

  return (
    <>
      {open.length > 0 ? (
        <ContinueHero
          session={open[0]}
          title={sessionTitle(open[0], bookById, seriesById)}
          books={sessionBooks(open[0], bookById)}
          bookById={bookById}
        />
      ) : (
        <StartHero greeting={greeting} book={newest[0]} count={libraryCount} />
      )}

      <AndroidTestingLink />

      {continues.length > 0 ? (
        <Shelf title={t('layout.home.shelves.continuing')} to="/sessions">
          {continues.map((book) => (
            <ShelfItem key={book.id}>
              <BookCard book={book} size="small" status={progress.status(book.id)} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {newest.length > 0 ? (
        <Shelf title={t('layout.home.shelves.newest')} to="/books">
          {newest.map((book) => (
            <ShelfItem key={book.id}>
              <BookCard book={book} size="small" status={progress.status(book.id)} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {series.length > 0 ? (
        <Shelf title={t('layout.home.shelves.series')} to="/series">
          {series.map((item) => (
            <div key={item.id} className="w-44 shrink-0 snap-start">
              <Link to={`/series/${item.id}`} className="group block">
                {/* Stoh v „lg“ má šířku police – menší by se vedle obálek knih ztratil. */}
                <SeriesCoverStack books={item.covers} variant="lg" />
                <p className="mt-3 line-clamp-2 font-heading text-sm font-semibold tracking-tight transition-colors group-hover:text-primary">
                  {item.title}
                </p>
                <p className="text-xs text-muted-foreground">
                  {bookCount(item.book_count)}
                </p>
              </Link>
            </div>
          ))}
        </Shelf>
      ) : null}

      {finished.length > 0 ? (
        <Shelf title={t('layout.home.shelves.finished')}>
          {finished.map((book) => (
            <ShelfItem key={book.id}>
              <BookCard book={book} size="small" status="finished" />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {libraryCount === 0 ? (
        <EmptyState
          icon={LibraryIcon}
          title={t('layout.home.emptyTitle')}
          description={t('layout.home.emptyDescription')}
        />
      ) : null}
    </>
  )
}

/** Hlavička s nejbližším rozposlouchaným poslechem. */
function ContinueHero({
  session,
  title,
  books,
  bookById,
}: {
  session: PlaySession
  title: string
  books: BookSummary[]
  bookById: ReadonlyMap<string, BookSummary>
}) {
  const { t } = useTranslation()
  const player = usePlayer()
  const state = sessionProgress(session)
  const currentBook = state.bookId ? bookById.get(state.bookId) : undefined

  // Otevřený poslech se odtud jen pozastaví a rozjede; načítat ho znovu ze
  // serveru by zahodilo pozici, kterou přehrávač právě drží.
  const isOpen = player.session?.id === session.id
  const isPlaying = isOpen && player.playing

  return (
    <section className="glass inset-shadow-glass relative isolate overflow-hidden rounded-3xl bg-brand-glow p-6 shadow-glass ring-1 ring-glass-edge md:p-8">
      {currentBook?.cover_path ? (
        <img
          src={coverUrl(currentBook, 160)}
          alt=""
          aria-hidden
          decoding="async"
          className="absolute inset-0 -z-10 size-full scale-125 object-cover opacity-25 blur-3xl dark:opacity-20"
        />
      ) : null}

      <div className="flex flex-wrap items-center gap-6 md:gap-8">
        <div className="w-32 shrink-0 sm:w-40 md:w-48">
          {books.length > 1 ? (
            <SeriesCoverStack books={books} variant="lg" />
          ) : currentBook ? (
            <Link to={`/books/${currentBook.id}`}>
              <BookCover
                book={currentBook}
                key={currentBook.id}
                lift={false}
                width={192}
                className="shadow-glass-lg"
              />
            </Link>
          ) : (
            <div className="aspect-square w-full rounded-2xl bg-foreground/8" />
          )}
        </div>

        <div className="min-w-56 flex-1">
          <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
            {t('layout.home.continueEyebrow')}
          </p>
          <h1 className="font-heading text-3xl font-semibold tracking-tight text-balance md:text-4xl">
            {title}
          </h1>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{sessionKindLabel(session)}</Badge>
            {state.bookCount > 1 ? (
              <Badge variant="outline">
                {t('sessions.bookOf', { number: state.bookNumber, count: state.bookCount })}
              </Badge>
            ) : null}
            {state.positionSeconds > 0 ? (
              <Badge variant="highlight">{formatClock(state.positionSeconds)}</Badge>
            ) : null}
          </div>

          {currentBook ? (
            <p className="mt-3 text-muted-foreground">
              <Link to={`/books/${currentBook.id}`} className="hover:underline">
                {currentBook.title}
              </Link>
              {currentBook.authors?.length ? ` · ${authorsLabel(currentBook.authors)}` : null}
            </p>
          ) : null}

          <div className="mt-5 flex flex-wrap items-center gap-2">
            <Button
              size="lg"
              onClick={() => (isOpen ? player.toggle() : player.switchSession(session.id))}
              disabled={player.loading}
            >
              {isPlaying ? <PauseIcon /> : <PlayIcon />}
              {isPlaying ? t('player.pause') : isOpen ? t('player.play') : t('player.resume')}
            </Button>
            <Button variant="outline" size="lg" asChild>
              <Link to="/sessions">
                <HeadphonesIcon />
                {t('layout.home.allSessions')}
              </Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}

/** Hlavička pro účet, který zatím nic neposlouchá. */
function StartHero({
  greeting,
  book,
  count,
}: {
  greeting: string
  book: BookSummary | undefined
  count: number
}) {
  const { t } = useTranslation()
  const player = usePlayer()

  return (
    <section className="glass inset-shadow-glass relative isolate overflow-hidden rounded-3xl bg-brand-glow p-6 shadow-glass ring-1 ring-glass-edge md:p-8">
      {book?.cover_path ? (
        <img
          src={coverUrl(book, 160)}
          alt=""
          aria-hidden
          decoding="async"
          className="absolute inset-0 -z-10 size-full scale-125 object-cover opacity-25 blur-3xl dark:opacity-20"
        />
      ) : null}

      <div className="flex flex-wrap items-center gap-6 md:gap-8">
        {book ? (
          <div className="w-32 shrink-0 sm:w-40 md:w-48">
            <Link to={`/books/${book.id}`}>
              <BookCover book={book} key={book.id} lift={false} width={192} className="shadow-glass-lg" />
            </Link>
          </div>
        ) : null}

        <div className="min-w-56 flex-1">
          <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
            {greeting}
          </p>
          <h1 className="font-heading text-3xl font-semibold tracking-tight text-balance md:text-4xl">
            {t('layout.home.startTitle')}
          </h1>
          <p className="mt-3 text-muted-foreground">
            {count > 0
              ? t('layout.home.startDescription', { books: bookCount(count) })
              : t('layout.home.emptyDescription')}
          </p>

          <div className="mt-5 flex flex-wrap items-center gap-2">
            {book ? (
              <Button size="lg" onClick={() => player.playBook(book.id)} disabled={player.loading}>
                <PlayIcon />
                {t('layout.home.playBook', { title: book.title })}
              </Button>
            ) : null}
            <Button variant="outline" size="lg" asChild>
              <Link to="/books">
                <LibraryIcon />
                {t('layout.home.toLibrary')}
              </Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}

/** Pozvánka do testování aplikace pro Android. */
function AndroidTestingLink() {
  const { t } = useTranslation()

  return (
    <a
      href={ANDROID_TESTING_URL}
      target="_blank"
      rel="noopener noreferrer"
      className="glass group mt-4 flex items-center gap-4 rounded-2xl p-4 shadow-glass ring-1 ring-glass-edge transition-colors hover:ring-primary/50"
    >
      <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        <SmartphoneIcon className="size-5" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block font-heading font-semibold tracking-tight transition-colors group-hover:text-primary">
          {t('layout.home.androidTesting')}
        </span>
        <span className="block text-sm text-muted-foreground">
          {t('layout.home.androidTestingDescription')}
        </span>
      </span>
      <ExternalLinkIcon className="size-4 shrink-0 text-muted-foreground" />
    </a>
  )
}
