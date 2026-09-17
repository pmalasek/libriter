import { useMemo } from 'react'
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { useLocalSearchParams, useRouter } from 'expo-router'

import { BookCover } from '@/components/BookCover'
import { useDownloads, useLocalBook, useLocalChapters } from '@/db/queries'
import { downloadManager } from '@/downloads/downloadManager'
import { usePlayer } from '@/player/PlayerProvider'
import { colors, formatBytes, formatDuration, radius, spacing } from '@/theme'

/**
 * Detail knihy: co to je, kolik místa zabere, kapitoly a tlačítka.
 * Správa knihovny (úpravy, metadata) zůstává na webu – tady se jen poslouchá.
 */
export default function BookScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const book = useLocalBook(id)
  const chapters = useLocalChapters(id)
  const downloads = useDownloads()
  const player = usePlayer()
  const router = useRouter()

  const download = useMemo(
    () => (downloads.data ?? []).find((row) => row.bookId === id),
    [downloads.data, id],
  )
  const size = useMemo(
    () => (chapters.data ?? []).reduce((sum, chapter) => sum + chapter.size_bytes, 0),
    [chapters.data],
  )

  const current = book.data
  if (!current) {
    return <Text style={styles.empty}>{book.isLoading ? 'Načítám…' : 'Kniha nenalezena.'}</Text>
  }

  const play = (chapterId?: string) => {
    void player.playBook(current.id, chapterId)
    router.push('/player')
  }

  return (
    <ScrollView contentContainerStyle={styles.content}>
      <View style={styles.head}>
        <BookCover
          bookId={current.id}
          title={current.title}
          size={110}
          downloaded={download?.state === 'complete'}
        />
        <View style={styles.headTexts}>
          <Text style={styles.title}>{current.title}</Text>
          <Text style={styles.author}>
            {current.authors.map((author) => author.name).join(', ') || 'Neznámý autor'}
          </Text>
          <Text style={styles.meta}>
            {formatDuration(current.duration_seconds)} · {current.chapter_count} kapitol
            {size > 0 && ` · ${formatBytes(size)}`}
          </Text>
        </View>
      </View>

      <View style={styles.actions}>
        <Pressable style={styles.primary} onPress={() => play()}>
          <Text style={styles.primaryText}>Přehrát</Text>
        </Pressable>
        <DownloadButton bookId={current.id} state={download?.state} />
      </View>

      <DownloadProgress
        state={download?.state}
        done={download?.bytesDone ?? 0}
        total={download?.bytesTotal ?? 0}
        error={download?.error ?? ''}
      />

      {current.description && <Text style={styles.description}>{current.description}</Text>}

      <Text style={styles.sectionTitle}>Kapitoly</Text>
      {(chapters.data ?? []).map((chapter) => (
        <Pressable
          key={chapter.id}
          style={({ pressed }) => [styles.chapter, pressed && styles.pressed]}
          onPress={() => play(chapter.id)}
        >
          <Text style={styles.chapterPosition}>{chapter.position}</Text>
          <Text style={styles.chapterTitle} numberOfLines={1}>
            {chapter.title}
          </Text>
          <Text style={styles.chapterDuration}>{formatDuration(chapter.duration_seconds)}</Text>
        </Pressable>
      ))}
      {(chapters.data ?? []).length === 0 && (
        <Text style={styles.empty}>Kapitoly se načtou při první synchronizaci.</Text>
      )}
    </ScrollView>
  )
}

function DownloadButton({ bookId, state }: { bookId: string; state?: string }) {
  if (state === 'complete') {
    return (
      <Pressable style={styles.secondary} onPress={() => void downloadManager.remove(bookId)}>
        <Text style={styles.secondaryText}>Smazat z telefonu</Text>
      </Pressable>
    )
  }
  if (state === 'downloading' || state === 'queued') {
    return (
      <Pressable style={styles.secondary} onPress={() => void downloadManager.pause(bookId)}>
        <Text style={styles.secondaryText}>Pozastavit</Text>
      </Pressable>
    )
  }
  return (
    <Pressable style={styles.secondary} onPress={() => void downloadManager.enqueue(bookId)}>
      <Text style={styles.secondaryText}>{state === 'paused' ? 'Pokračovat' : 'Stáhnout'}</Text>
    </Pressable>
  )
}

function DownloadProgress({
  state,
  done,
  total,
  error,
}: {
  state?: string
  done: number
  total: number
  error: string
}) {
  if (!state || state === 'complete') return null
  if (state === 'error') return <Text style={styles.error}>Stahování selhalo: {error}</Text>

  const ratio = total > 0 ? Math.min(1, done / total) : 0
  return (
    <View style={styles.progress}>
      <View style={styles.progressTrack}>
        <View style={[styles.progressFill, { width: `${ratio * 100}%` }]} />
      </View>
      <Text style={styles.meta}>
        {state === 'paused' ? 'Pozastaveno' : 'Stahuji'} · {formatBytes(done)} z {formatBytes(total)}
        {error !== '' && ` · ${error}`}
      </Text>
    </View>
  )
}

const styles = StyleSheet.create({
  content: { padding: spacing.md, paddingBottom: spacing.xl, gap: spacing.md },
  head: { flexDirection: 'row', gap: spacing.md },
  headTexts: { flex: 1, gap: spacing.xs },
  title: { color: colors.text, fontSize: 22, fontWeight: '700' },
  author: { color: colors.textMuted, fontSize: 15 },
  meta: { color: colors.textMuted, fontSize: 13 },
  actions: { flexDirection: 'row', gap: spacing.sm },
  primary: {
    flex: 1,
    backgroundColor: colors.accent,
    borderRadius: radius.sm,
    paddingVertical: spacing.sm + 4,
    alignItems: 'center',
  },
  primaryText: { color: colors.accentText, fontSize: 16, fontWeight: '600' },
  secondary: {
    flex: 1,
    backgroundColor: colors.surfaceAlt,
    borderRadius: radius.sm,
    paddingVertical: spacing.sm + 4,
    alignItems: 'center',
  },
  secondaryText: { color: colors.text, fontSize: 15 },
  progress: { gap: spacing.xs },
  progressTrack: { height: 4, backgroundColor: colors.border, borderRadius: 2 },
  progressFill: { height: 4, backgroundColor: colors.accent, borderRadius: 2 },
  description: { color: colors.textMuted, fontSize: 14, lineHeight: 21 },
  sectionTitle: {
    color: colors.textMuted,
    fontSize: 13,
    textTransform: 'uppercase',
    letterSpacing: 1,
    marginTop: spacing.sm,
  },
  chapter: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: spacing.sm,
    borderBottomColor: colors.border,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  pressed: { opacity: 0.6 },
  chapterPosition: { color: colors.textMuted, fontSize: 13, width: 24 },
  chapterTitle: { color: colors.text, fontSize: 15, flex: 1 },
  chapterDuration: { color: colors.textMuted, fontSize: 13 },
  error: { color: colors.danger, fontSize: 14 },
  empty: { color: colors.textMuted, textAlign: 'center', marginTop: spacing.xl },
})
