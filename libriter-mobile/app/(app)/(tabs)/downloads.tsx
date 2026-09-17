import { useMemo } from 'react'
import { FlatList, Pressable, StyleSheet, Text, View } from 'react-native'
import { useRouter } from 'expo-router'

import { BookCover } from '@/components/BookCover'
import { useDownloads, useLocalBooks } from '@/db/queries'
import { downloadManager } from '@/downloads/downloadManager'
import { colors, formatBytes, spacing } from '@/theme'
import type { DownloadRow } from '@/db/downloads'

/** Co je v telefonu: hotové knihy i rozpracované stahování. */
export default function DownloadsScreen() {
  const downloads = useDownloads()
  const books = useLocalBooks()
  const router = useRouter()

  const bookById = useMemo(
    () => new Map((books.data ?? []).map((book) => [book.id, book])),
    [books.data],
  )
  const rows = downloads.data ?? []
  const total = rows.reduce((sum, row) => sum + row.bytesDone, 0)

  return (
    <FlatList
      data={rows}
      keyExtractor={(row) => row.bookId}
      contentContainerStyle={styles.list}
      ListHeaderComponent={
        rows.length > 0 ? <Text style={styles.summary}>Zabráno {formatBytes(total)}</Text> : null
      }
      ListEmptyComponent={
        <Text style={styles.empty}>
          Zatím nic staženého. V detailu knihy najdete tlačítko Stáhnout.
        </Text>
      }
      renderItem={({ item }) => (
        <Row
          row={item}
          title={bookById.get(item.bookId)?.title ?? 'Neznámá kniha'}
          onPress={() => router.push(`/book/${item.bookId}`)}
        />
      )}
    />
  )
}

function Row({ row, title, onPress }: { row: DownloadRow; title: string; onPress: () => void }) {
  return (
    <Pressable style={({ pressed }) => [styles.row, pressed && styles.pressed]} onPress={onPress}>
      <BookCover bookId={row.bookId} title={title} size={44} downloaded={row.state === 'complete'} />
      <View style={styles.texts}>
        <Text style={styles.title} numberOfLines={1}>
          {title}
        </Text>
        <Text style={styles.meta}>{describe(row)}</Text>
      </View>
      <Pressable hitSlop={10} onPress={() => void downloadManager.remove(row.bookId)}>
        <Text style={styles.delete}>Smazat</Text>
      </Pressable>
    </Pressable>
  )
}

function describe(row: DownloadRow): string {
  switch (row.state) {
    case 'complete':
      return `V telefonu · ${formatBytes(row.bytesDone)}`
    case 'downloading':
      return `Stahuji · ${formatBytes(row.bytesDone)} z ${formatBytes(row.bytesTotal)}`
    case 'queued':
      return 'Ve frontě'
    case 'paused':
      return row.error !== '' ? `Pozastaveno · ${row.error}` : 'Pozastaveno'
    case 'error':
      return `Chyba · ${row.error}`
  }
}

const styles = StyleSheet.create({
  list: { padding: spacing.md },
  summary: { color: colors.textMuted, fontSize: 13, marginBottom: spacing.sm },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, paddingVertical: spacing.sm },
  pressed: { opacity: 0.6 },
  texts: { flex: 1 },
  title: { color: colors.text, fontSize: 16, fontWeight: '600' },
  meta: { color: colors.textMuted, fontSize: 13, marginTop: 2 },
  delete: { color: colors.danger, fontSize: 14 },
  empty: { color: colors.textMuted, textAlign: 'center', marginTop: spacing.xl },
})
