import { useMemo, useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Headphones, Library, Pause, Play } from 'lucide-react-native'
import {
  authorsLabel,
  bookCount,
  formatClock,
  sessionBooks,
  sessionKindLabel,
  sessionProgress,
  sessionTitle,
  type Book,
  type PlaySession,
} from 'libriter-shared'

import { useAuth } from '@/auth/AuthProvider'
import { BookCard } from '@/components/BookCard'
import { BookCover, coverUrl } from '@/components/BookCover'
import { EmptyState } from '@/components/EmptyState'
import { Screen, ActionRow } from '@/components/Screen'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Shelf, ShelfItem } from '@/components/Shelf'
import { SyncBadge } from '@/components/SyncBadge'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { GlassCard } from '@/components/ui/GlassCard'
import { Body, Eyebrow, Heading, Muted, Title } from '@/components/ui/Text'
import { useBookProgress, useBooks, useSeriesById, useSeriesList, useSessions } from '@/data/hooks'
import { usePlayer } from '@/player/PlayerProvider'
import { syncEngine } from '@/sync/syncEngine'
import { spacing, useTheme } from '@/theme'

/** Kolik položek se vejde do police, než začne být rolování únavné. */
const SHELF_SIZE = 12

/**
 * První pohled po otevření aplikace – totéž co HomePage na webu: čím se dá
 * pokračovat a co v knihovně stojí za pozornost.
 */
export default function HomeScreen() {
  const { user } = useAuth()
  const sessions = useSessions()
  const books = useBooks()
  const seriesList = useSeriesList()
  const { map: seriesById } = useSeriesById()
  const progress = useBookProgress()
  const [refreshing, setRefreshing] = useState(false)

  const bookById = useMemo(() => new Map((books.data ?? []).map((book) => [book.id, book])), [books.data])

  // Rozposlouchané poslechy od naposledy otevřeného; ten první jde do hlavičky.
  const open = useMemo(
    () =>
      (sessions.data ?? [])
        .filter((session) => !session.finished_at)
        .sort((a, b) => b.updated_at.localeCompare(a.updated_at)),
    [sessions.data],
  )

  const newest = useMemo(
    () => [...(books.data ?? [])].sort((a, b) => b.created_at.localeCompare(a.created_at)).slice(0, SHELF_SIZE),
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
    for (const list of m.values()) list.sort((a, b) => (a.series_position ?? 0) - (b.series_position ?? 0))
    return m
  }, [books.data])

  const series = useMemo(
    () => (seriesList.data ?? []).filter((item) => seriesBooksById.has(item.id)).slice(0, SHELF_SIZE),
    [seriesList.data, seriesBooksById],
  )

  const refresh = async () => {
    setRefreshing(true)
    await Promise.all([syncEngine.syncNow(), books.refetch(), sessions.refetch(), progress.refetch()])
    setRefreshing(false)
  }

  const greeting = user?.display_name ? `Vítejte zpět, ${user.display_name}` : 'Vítejte zpět'

  return (
    <Screen refreshing={refreshing} onRefresh={() => void refresh()}>
      <SyncBadge />

      {open.length > 0 ? (
        <ContinueHero
          session={open[0]}
          title={sessionTitle(open[0], bookById, seriesById)}
          books={sessionBooks(open[0], bookById)}
          bookById={bookById}
        />
      ) : (
        <StartHero greeting={greeting} book={newest[0]} count={(books.data ?? []).length} loading={books.isPending} />
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
            <ShelfItem key={item.id} width={176}>
              <SeriesShelfItem
                id={item.id}
                title={item.title}
                books={seriesBooksById.get(item.id) ?? []}
              />
            </ShelfItem>
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

      {!books.isPending && (books.data ?? []).length === 0 ? (
        <EmptyState
          icon={Library}
          title="Knihovna je zatím prázdná"
          description="Jakmile server načte audio soubory, objeví se knihy tady i v seznamu knih."
        />
      ) : null}
    </Screen>
  )
}

function SeriesShelfItem({ id, title, books }: { id: string; title: string; books: Book[] }) {
  const router = useRouter()
  return (
    <Pressable onPress={() => router.push(`/series/${id}`)} style={({ pressed }) => ({ opacity: pressed ? 0.7 : 1 })}>
      <SeriesCoverStack books={books} variant="lg" />
      <Title numberOfLines={2} style={{ marginTop: spacing.sm + 4 }}>
        {title}
      </Title>
      <Muted size={12}>{bookCount(books.length)}</Muted>
    </Pressable>
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
  const router = useRouter()
  const player = usePlayer()
  const state = sessionProgress(session)
  const currentBook = state.bookId ? bookById.get(state.bookId) : undefined

  // Otevřený poslech se odtud jen pozastaví a rozjede; načítat ho znovu ze
  // serveru by zahodilo pozici, kterou přehrávač právě drží.
  const isOpen = player.session?.id === session.id
  const isPlaying = isOpen && player.playing

  return (
    <GlassCard glow backdrop={currentBook?.cover_path ? coverUrl(currentBook) : undefined}>
      <View style={styles.heroRow}>
        {books.length > 1 ? (
          <SeriesCoverStack books={books} variant="lg" />
        ) : currentBook ? (
          <Pressable onPress={() => router.push(`/book/${currentBook.id}`)}>
            <BookCover book={currentBook} size={128} />
          </Pressable>
        ) : null}
      </View>

      <Eyebrow style={{ marginTop: spacing.md, marginBottom: 6 }}>Pokračovat v poslechu</Eyebrow>
      <Heading size={26}>{title}</Heading>

      <View style={styles.badges}>
        <Badge label={sessionKindLabel(session)} />
        {state.bookCount > 1 ? <Badge variant="outline" label={`Kniha ${state.bookNumber} z ${state.bookCount}`} /> : null}
        {state.positionSeconds > 0 ? <Badge variant="highlight" label={formatClock(state.positionSeconds)} /> : null}
      </View>

      {currentBook ? (
        <Muted size={14} style={{ marginTop: spacing.sm + 4 }}>
          {currentBook.title}
          {currentBook.authors?.length ? ` · ${authorsLabel(currentBook.authors)}` : ''}
        </Muted>
      ) : null}

      <ActionRow style={{ marginTop: spacing.md + 4 }}>
        <Button
          size="lg"
          icon={isPlaying ? Pause : Play}
          label={isPlaying ? 'Pozastavit' : isOpen ? 'Přehrát' : 'Pokračovat'}
          onPress={() => (isOpen ? void player.toggle() : void player.switchSession(session.id))}
          disabled={player.loading}
        />
        <Button variant="outline" size="lg" icon={Headphones} label="Všechny poslechy" onPress={() => router.push('/sessions')} />
      </ActionRow>
    </GlassCard>
  )
}

/** Hlavička pro účet, který zatím nic neposlouchá. */
function StartHero({ greeting, book, count, loading }: { greeting: string; book: Book | undefined; count: number; loading: boolean }) {
  const router = useRouter()
  const player = usePlayer()
  const { colors } = useTheme()

  return (
    <GlassCard glow backdrop={book?.cover_path ? coverUrl(book) : undefined}>
      {book ? (
        <Pressable onPress={() => router.push(`/book/${book.id}`)} style={styles.heroRow}>
          <BookCover book={book} size={128} />
        </Pressable>
      ) : null}
      <Eyebrow style={{ marginTop: book ? spacing.md : 0, marginBottom: 6 }}>{greeting}</Eyebrow>
      <Heading size={26}>Začněte poslouchat</Heading>
      <Body size={14} style={{ color: colors.mutedForeground, marginTop: spacing.sm + 4 }}>
        {loading
          ? 'Načítám knihovnu…'
          : count > 0
            ? `V knihovně čeká ${bookCount(count)}. Vyberte si, nebo rovnou pusťte poslední přírůstek.`
            : 'Jakmile server načte audio soubory, objeví se knihy tady i v seznamu knih.'}
      </Body>
      <ActionRow style={{ marginTop: spacing.md + 4 }}>
        {book ? (
          <Button size="lg" icon={Play} label={`Přehrát ${book.title}`} onPress={() => void player.playBook(book.id)} disabled={player.loading} />
        ) : null}
        <Button variant="outline" size="lg" icon={Library} label="Do knihovny" onPress={() => router.push('/books')} />
      </ActionRow>
    </GlassCard>
  )
}

const styles = StyleSheet.create({
  heroRow: { alignItems: 'flex-start' },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm, marginTop: spacing.sm + 4 },
})
