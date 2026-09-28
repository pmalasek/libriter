import { useEffect, useMemo, useRef } from 'react'
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { formatBytes, t, type Book } from 'libriter-shared'

import { useBooks, useDownloads } from '@/data/hooks'
import type { DownloadRow } from '@/db/downloads'
import { radius, spacing, useTheme } from '@/theme'
import { BookCover } from './BookCover'
import { toast } from './Toast'
import { GlassBackground } from './ui/Blur'
import { Muted, Title } from './ui/Text'

/**
 * Průběh stahování nad lištou tabů. Stahování se spouští i z dialogu
 * přehrávače (offline režim, nestažená kniha) a detail knihy, kde se průběh
 * jinak ukazuje, přitom nikdo nemá otevřený – bez kapsle to vypadalo, že se
 * nic neděje. Klepnutí otevře seznam stažených.
 */
export function DownloadCapsule() {
  // Překreslení při změně jazyka.
  useTranslation()
  const { colors, resolved } = useTheme()
  const router = useRouter()
  const downloads = useDownloads()
  const books = useBooks()

  const rows = useMemo(() => downloads.data ?? [], [downloads.data])
  const active = useMemo(() => rows.filter((row) => row.state === 'downloading' || row.state === 'queued'), [rows])
  const current = active.find((row) => row.state === 'downloading') ?? active[0]
  const book = useMemo(() => (books.data ?? []).find((item) => item.id === current?.bookId), [books.data, current?.bookId])

  useFinishedToast(rows, books.data)

  if (!current) return null

  const ratio = current.bytesTotal > 0 ? Math.min(1, current.bytesDone / current.bytesTotal) : 0
  const status =
    current.state === 'downloading'
      ? t('mobile.downloads.state.downloading', { done: formatBytes(current.bytesDone), total: formatBytes(current.bytesTotal) })
      : t('mobile.downloads.state.queued')

  return (
    <Pressable
      onPress={() => router.push('/downloads')}
      style={({ pressed }) => [
        styles.capsule,
        { borderColor: colors.glassEdge, shadowOpacity: resolved === 'dark' ? 0.5 : 0.18, opacity: pressed ? 0.9 : 1 },
      ]}
      accessibilityRole="button"
      accessibilityLabel={t('mobile.downloads.open')}
    >
      <GlassBackground intensity={80} strong />

      <View style={styles.row}>
        {book ? <BookCover book={book} size={32} rounded={radius.md} /> : null}
        <View style={styles.texts}>
          <Title numberOfLines={1} size={13}>
            {book?.title ?? t('mobile.downloads.unknownBook')}
          </Title>
          <Muted numberOfLines={1} size={11}>
            {status}
            {active.length > 1 ? ` · +${active.length - 1}` : ''}
          </Muted>
        </View>
        {current.state === 'downloading' ? (
          <Muted size={12}>{Math.round(ratio * 100)} %</Muted>
        ) : (
          <ActivityIndicator size="small" color={colors.mutedForeground} />
        )}
      </View>

      <View style={[styles.track, { backgroundColor: colors.muted }]}>
        <View style={[styles.fill, { width: `${ratio * 100}%`, backgroundColor: colors.primary }]} />
      </View>
    </Pressable>
  )
}

/**
 * Ohlásí dokončení i pád. Kapsle v obou případech zmizí a kdo ji
 * nesledoval, jinak nepozná, jestli kniha doběhla, nebo stahování spadlo.
 */
function useFinishedToast(rows: DownloadRow[], books: Book[] | undefined) {
  const previous = useRef<Map<string, DownloadRow['state']> | null>(null)
  // Seznam knih se jen čte pro název; hlásí se změna stavu stahování.
  const bookList = useRef(books)
  bookList.current = books
  const titleOf = (bookId: string) => bookList.current?.find((book) => book.id === bookId)?.title ?? ''
  useEffect(() => {
    const before = previous.current
    previous.current = new Map(rows.map((row) => [row.bookId, row.state]))
    // První načtení jen zapamatuje stav – knihy stažené dřív se nehlásí.
    if (!before) return
    for (const row of rows) {
      const was = before.get(row.bookId)
      if (row.state === 'complete' && (was === 'downloading' || was === 'queued')) {
        toast.success(t('mobile.downloads.finished', { title: titleOf(row.bookId) }))
      } else if (row.state === 'error' && was === 'downloading') {
        toast.error(t('mobile.book.downloadState.error', { error: row.error }))
      }
    }
  }, [rows])
}

const styles = StyleSheet.create({
  capsule: {
    marginHorizontal: spacing.sm + 4,
    marginBottom: spacing.sm,
    borderRadius: radius['2xl'],
    borderWidth: StyleSheet.hairlineWidth,
    paddingVertical: spacing.sm,
    overflow: 'hidden',
    shadowColor: '#000',
    shadowRadius: 18,
    shadowOffset: { width: 0, height: 10 },
    elevation: 10,
  },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm + 2, paddingHorizontal: spacing.sm + 2 },
  texts: { flex: 1, minWidth: 0, gap: 1 },
  track: { height: 3, borderRadius: 2, overflow: 'hidden', marginTop: spacing.sm, marginHorizontal: spacing.sm + 2 },
  fill: { height: 3, borderRadius: 2 },
})
