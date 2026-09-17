import { HeadphonesIcon, LibraryIcon, PauseIcon, PlayIcon } from 'lucide-react'
import { useMemo } from 'react'
import { Link } from 'react-router'
import { useBookProgress, useBooks, useSeriesById, useSeriesList, useSessions } from '@/api/hooks'
import type { Book, PlaySession } from '@/api/types'
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

/**
 * První pohled po otevření aplikace: čím se dá pokračovat a co v knihovně
 * stojí za pozornost. Úplný přehled poslechů zůstává na stránce Právě
 * posloucháno, tady je jen ten nejbližší.
 */
export function HomePage() {
  const { user } = useAuth()
  const sessions = useSessions()
  const books = useBooks()
  const seriesList = useSeriesList()
  const { map: seriesById } = useSeriesById()
  const progress = useBookProgress()

  const bookById = useMemo(
    () => new Map((books.data ?? []).map((book) => [book.id, book])),
    [books.data],
  )

  // Rozposlouchané poslechy od naposledy otevřeného; ten první jde do hlavičky.
  const open = useMemo(
    () =>
      (sessions.data ?? [])
        .filter((session) => !session.finished_at)
        .sort((a, b) => b.updated_at.localeCompare(a.updated_at)),
    [sessions.data],
  )

  const newest = useMemo(
    () =>
      [...(books.data ?? [])]
        .sort((a, b) => b.created_at.localeCompare(a.created_at))
        .slice(0, SHELF_SIZE),
    [books.data],
  )

  // Knihy z ostatních rozposlouchaných poslechů – ta z hlavičky se neopakuje.
  const continues = useMemo(() => {
    const seen = new Set<string>()
    const list: Book[] = []
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
    () =>
      [...progress.map.values()]
        .filter((item) => item.finished_at)
        .sort((a, b) => (b.finished_at ?? '').localeCompare(a.finished_at ?? ''))
        .slice(0, SHELF_SIZE)
        .flatMap((item) => {
          const book = bookById.get(item.book_id)
          return book ? [book] : []
        }),
    [progress.map, bookById],
  )

  const seriesBooksById = useMemo(() => {
    const m = new Map<string, Book[]>()
    for (const book of books.data ?? []) {
      if (!book.series_id) continue
      const list = m.get(book.series_id) ?? []
      list.push(book)
      m.set(book.series_id, list)
    }
    // Ve stohu mají stát první díly, ne náhodné.
    for (const list of m.values()) {
      list.sort((a, b) => (a.series_position ?? 0) - (b.series_position ?? 0))
    }
    return m
  }, [books.data])

  // Série bez jediné knihy se neukazují – stejně jako v seznamu sérií. Prázdný
  // stoh obálek by na polici nic neřekl.
  const series = useMemo(
    () => (seriesList.data ?? []).filter((item) => seriesBooksById.has(item.id)).slice(0, SHELF_SIZE),
    [seriesList.data, seriesBooksById],
  )

  if (sessions.isPending || books.isPending) {
    return (
      <>
        <Skeleton className="h-64 w-full rounded-3xl" />
        <div className="mt-10">
          <LoadingGrid count={6} view="small" />
        </div>
      </>
    )
  }

  const greeting = user?.display_name ? `Vítejte zpět, ${user.display_name}` : 'Vítejte zpět'

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
        <StartHero greeting={greeting} book={newest[0]} count={(books.data ?? []).length} />
      )}

      {continues.length > 0 ? (
        <Shelf title="Rozposlouchané" to="/sessions">
          {continues.map((book) => (
            <ShelfItem key={book.id}>
              <BookCard book={book} size="small" status={progress.status(book.id)} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {newest.length > 0 ? (
        <Shelf title="Nově přidané" to="/books">
          {newest.map((book) => (
            <ShelfItem key={book.id}>
              <BookCard book={book} size="small" status={progress.status(book.id)} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {series.length > 0 ? (
        <Shelf title="Série" to="/series">
          {series.map((item) => (
            <div key={item.id} className="w-44 shrink-0 snap-start">
              <Link to={`/series/${item.id}`} className="group block">
                {/* Stoh v „lg“ má šířku police – menší by se vedle obálek knih ztratil. */}
                <SeriesCoverStack books={seriesBooksById.get(item.id) ?? []} variant="lg" />
                <p className="mt-3 line-clamp-2 font-heading text-sm font-semibold tracking-tight transition-colors group-hover:text-primary">
                  {item.title}
                </p>
                <p className="text-xs text-muted-foreground">
                  {bookCount((seriesBooksById.get(item.id) ?? []).length)}
                </p>
              </Link>
            </div>
          ))}
        </Shelf>
      ) : null}

      {finished.length > 0 ? (
        <Shelf title="Doposlechnuté">
          {finished.map((book) => (
            <ShelfItem key={book.id}>
              <BookCard book={book} size="small" status="finished" />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {(books.data ?? []).length === 0 ? (
        <EmptyState
          icon={LibraryIcon}
          title="Knihovna je zatím prázdná"
          description="Jakmile server načte audio soubory, objeví se knihy tady i v seznamu knih."
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
  books: Book[]
  bookById: Map<string, Book>
}) {
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
          src={coverUrl(currentBook)}
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
                className="shadow-glass-lg"
              />
            </Link>
          ) : (
            <div className="aspect-square w-full rounded-2xl bg-foreground/8" />
          )}
        </div>

        <div className="min-w-56 flex-1">
          <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
            Pokračovat v poslechu
          </p>
          <h1 className="font-heading text-3xl font-semibold tracking-tight text-balance md:text-4xl">
            {title}
          </h1>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{sessionKindLabel(session)}</Badge>
            {state.bookCount > 1 ? (
              <Badge variant="outline">
                Kniha {state.bookNumber} z {state.bookCount}
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
              {isPlaying ? 'Pozastavit' : isOpen ? 'Přehrát' : 'Pokračovat'}
            </Button>
            <Button variant="outline" size="lg" asChild>
              <Link to="/sessions">
                <HeadphonesIcon />
                Všechny poslechy
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
  book: Book | undefined
  count: number
}) {
  const player = usePlayer()

  return (
    <section className="glass inset-shadow-glass relative isolate overflow-hidden rounded-3xl bg-brand-glow p-6 shadow-glass ring-1 ring-glass-edge md:p-8">
      {book?.cover_path ? (
        <img
          src={coverUrl(book)}
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
              <BookCover book={book} key={book.id} lift={false} className="shadow-glass-lg" />
            </Link>
          </div>
        ) : null}

        <div className="min-w-56 flex-1">
          <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
            {greeting}
          </p>
          <h1 className="font-heading text-3xl font-semibold tracking-tight text-balance md:text-4xl">
            Začněte poslouchat
          </h1>
          <p className="mt-3 text-muted-foreground">
            {count > 0
              ? `V knihovně čeká ${bookCount(count)}. Vyberte si, nebo rovnou pusťte poslední přírůstek.`
              : 'Jakmile server načte audio soubory, objeví se knihy tady i v seznamu knih.'}
          </p>

          <div className="mt-5 flex flex-wrap items-center gap-2">
            {book ? (
              <Button size="lg" onClick={() => player.playBook(book.id)} disabled={player.loading}>
                <PlayIcon />
                Přehrát {book.title}
              </Button>
            ) : null}
            <Button variant="outline" size="lg" asChild>
              <Link to="/books">
                <LibraryIcon />
                Do knihovny
              </Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}
