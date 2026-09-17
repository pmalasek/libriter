import { useMemo } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Download, Trash2 } from 'lucide-react-native'
import { formatBytes } from 'libriter-shared'

import { BookCover } from '@/components/BookCover'
import { EmptyState } from '@/components/EmptyState'
import { PageHeader } from '@/components/PageHeader'
import { BackButton, Screen } from '@/components/Screen'
import { Body, Muted } from '@/components/ui/Text'
import { useBooks, useDownloads } from '@/data/hooks'
import type { DownloadRow } from '@/db/downloads'
import { downloadManager } from '@/downloads/downloadManager'
import { radius, spacing, useTheme } from '@/theme'

/** Co je v telefonu: hotové knihy i rozpracované stahování. */
export default function DownloadsScreen() {
  const { colors } = useTheme()
  const downloads = useDownloads()
  const books = useBooks()
  const router = useRouter()

  const bookById = useMemo(() => new Map((books.data ?? []).map((book) => [book.id, book])), [books.data])
  const rows = downloads.data ?? []
  const total = rows.reduce((sum, row) => sum + row.bytesDone, 0)

  return (
    <Screen>
      <BackButton label="Zpět" />
      <PageHeader title="Stažené" description={rows.length > 0 ? `Zabráno ${formatBytes(total)}` : 'Knihy uložené v telefonu'} />

      {rows.length === 0 ? (
        <EmptyState icon={Download} title="Zatím nic staženého" description="V detailu knihy najdete tlačítko Stáhnout." />
      ) : (
        <View style={[styles.list, { backgroundColor: colors.card, borderColor: colors.border }]}>
          {rows.map((row, index) => {
            const book = bookById.get(row.bookId)
            return (
              <Pressable
                key={row.bookId}
                onPress={() => router.push(`/book/${row.bookId}`)}
                style={({ pressed }) => [
                  styles.row,
                  index > 0 && { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border },
                  pressed && { backgroundColor: colors.muted },
                ]}
              >
                {book ? (
                  <BookCover book={book} size={48} rounded={radius.md} downloaded={row.state === 'complete'} />
                ) : (
                  <View style={{ width: 48, height: 48, borderRadius: radius.md, backgroundColor: colors.muted }} />
                )}
                <View style={{ flex: 1, minWidth: 0 }}>
                  <Body medium numberOfLines={1}>
                    {book?.title ?? 'Neznámá kniha'}
                  </Body>
                  <Muted size={12}>{describe(row)}</Muted>
                </View>
                <Pressable hitSlop={10} onPress={() => void downloadManager.remove(row.bookId)} accessibilityLabel="Smazat z telefonu">
                  <Trash2 color={colors.destructive} size={18} />
                </Pressable>
              </Pressable>
            )
          })}
        </View>
      )}
    </Screen>
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
  list: { borderWidth: 1, borderRadius: radius['2xl'], overflow: 'hidden' },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm + 4, paddingHorizontal: spacing.sm + 4, paddingVertical: spacing.sm },
})
