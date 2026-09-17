import { useMemo, useState } from 'react'
import { FlatList, Pressable, RefreshControl, StyleSheet, Text, View } from 'react-native'
import { useRouter } from 'expo-router'
import { sessionProgress, sessionTitle, type Book, type PlaySession } from 'libriter-shared'

import { BookCover } from '@/components/BookCover'
import { SyncBadge } from '@/components/SyncBadge'
import { useDownloads, useLocalBooks, useLocalSessions } from '@/db/queries'
import { usePlayer } from '@/player/PlayerProvider'
import { syncEngine } from '@/sync/syncEngine'
import { colors, formatDuration, spacing } from '@/theme'

/**
 * Knihovna: nahoře rozposlouchané poslechy, pod nimi všechny knihy.
 * Data jdou z lokální databáze, takže se seznam ukáže okamžitě i bez sítě.
 */
export default function LibraryScreen() {
  const books = useLocalBooks()
  const sessions = useLocalSessions()
  const downloads = useDownloads()
  const player = usePlayer()
  const router = useRouter()
  const [refreshing, setRefreshing] = useState(false)

  const bookById = useMemo(
    () => new Map((books.data ?? []).map((book) => [book.id, book])),
    [books.data],
  )
  const downloaded = useMemo(
    () =>
      new Set(
        (downloads.data ?? []).filter((row) => row.state === 'complete').map((row) => row.bookId),
      ),
    [downloads.data],
  )
  const open = (sessions.data ?? []).filter((session) => !session.finished_at)

  const refresh = async () => {
    setRefreshing(true)
    await syncEngine.syncNow()
    setRefreshing(false)
  }

  return (
    <FlatList
      data={books.data ?? []}
      keyExtractor={(book) => book.id}
      contentContainerStyle={styles.list}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={() => void refresh()} tintColor={colors.accent} />
      }
      ListHeaderComponent={
        <View style={styles.header}>
          <SyncBadge />
          {open.length > 0 && (
            <>
              <Text style={styles.sectionTitle}>Rozposlouchané</Text>
              {open.map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  bookById={bookById}
                  onPress={() => {
                    void player.openSession(session.id)
                    router.push('/player')
                  }}
                />
              ))}
            </>
          )}
          <Text style={styles.sectionTitle}>Knihovna</Text>
        </View>
      }
      ListEmptyComponent={
        <Text style={styles.empty}>
          {books.isLoading ? 'Načítám…' : 'Knihovna je prázdná. Zkuste stáhnout data dolů.'}
        </Text>
      }
      renderItem={({ item }) => (
        <BookRow
          book={item}
          downloaded={downloaded.has(item.id)}
          onPress={() => router.push(`/book/${item.id}`)}
        />
      )}
    />
  )
}

function SessionRow({
  session,
  bookById,
  onPress,
}: {
  session: PlaySession
  bookById: Map<string, Book>
  onPress: () => void
}) {
  const progress = sessionProgress(session)
  const book = progress.bookId ? bookById.get(progress.bookId) : undefined
  const title = sessionTitle(session, bookById, new Map())

  return (
    <Pressable style={({ pressed }) => [styles.row, pressed && styles.pressed]} onPress={onPress}>
      {book && <BookCover bookId={book.id} title={book.title} size={48} />}
      <View style={styles.texts}>
        <Text style={styles.title} numberOfLines={1}>
          {title}
        </Text>
        <Text style={styles.meta}>
          {progress.bookCount > 1 && `kniha ${progress.bookNumber}/${progress.bookCount} · `}
          {formatDuration(progress.positionSeconds)}
        </Text>
      </View>
      <Text style={styles.play}>▶</Text>
    </Pressable>
  )
}

function BookRow({
  book,
  downloaded,
  onPress,
}: {
  book: Book
  downloaded: boolean
  onPress: () => void
}) {
  return (
    <Pressable style={({ pressed }) => [styles.row, pressed && styles.pressed]} onPress={onPress}>
      <BookCover bookId={book.id} title={book.title} size={48} downloaded={downloaded} />
      <View style={styles.texts}>
        <Text style={styles.title} numberOfLines={1}>
          {book.title}
        </Text>
        <Text style={styles.meta} numberOfLines={1}>
          {book.authors.map((author) => author.name).join(', ') || 'Neznámý autor'}
          {' · '}
          {formatDuration(book.duration_seconds)}
        </Text>
      </View>
      {downloaded && <Text style={styles.badge}>offline</Text>}
    </Pressable>
  )
}

const styles = StyleSheet.create({
  list: { padding: spacing.md, paddingBottom: spacing.xl },
  header: { gap: spacing.xs },
  sectionTitle: {
    color: colors.textMuted,
    fontSize: 13,
    textTransform: 'uppercase',
    letterSpacing: 1,
    marginTop: spacing.md,
    marginBottom: spacing.xs,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.sm,
  },
  pressed: { opacity: 0.6 },
  texts: { flex: 1 },
  title: { color: colors.text, fontSize: 16, fontWeight: '600' },
  meta: { color: colors.textMuted, fontSize: 13, marginTop: 2 },
  play: { color: colors.accent, fontSize: 20 },
  badge: { color: colors.accent, fontSize: 11, textTransform: 'uppercase', letterSpacing: 1 },
  empty: { color: colors.textMuted, textAlign: 'center', marginTop: spacing.xl },
})
