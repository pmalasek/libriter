import { useState } from 'react'
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native'
import { ChevronDown, Pause, Play } from 'lucide-react-native'
import { chapterCount, formatClock, formatDuration, type Book } from 'libriter-shared'

import { useChapters } from '@/data/hooks'
import { usePlayer } from '@/player/PlayerProvider'
import { fonts, radius, spacing, useTheme } from '@/theme'
import { ErrorState } from './EmptyState'
import { Body, Muted, SectionTitle } from './ui/Text'

/**
 * Kapitoly knihy. Sbalené, protože u většiny návštěv jde jen o technický
 * rozpis; načítají se až po rozbalení, jako na webu.
 */
export function ChapterList({ book }: { book: Book }) {
  const { colors } = useTheme()
  const [open, setOpen] = useState(false)
  const chapters = useChapters(book.id, open)
  const player = usePlayer()
  const list = chapters.data ?? []

  return (
    <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.border }]}>
      <Pressable onPress={() => setOpen((value) => !value)} style={styles.head} accessibilityRole="button">
        <ChevronDown
          color={colors.mutedForeground}
          size={18}
          style={{ transform: [{ rotate: open ? '180deg' : '0deg' }] }}
        />
        <View style={{ flex: 1 }}>
          <SectionTitle>Kapitoly</SectionTitle>
          <Muted size={13}>
            {chapterCount(book.chapter_count)} · {formatDuration(book.duration_seconds)}
          </Muted>
        </View>
      </Pressable>

      {open ? (
        <View style={styles.body}>
          {chapters.isPending ? (
            <ActivityIndicator color={colors.primary} />
          ) : chapters.isError ? (
            <ErrorState error={chapters.error} onRetry={() => void chapters.refetch()} />
          ) : list.length === 0 ? (
            <Muted size={14}>Kniha zatím nemá načtené kapitoly. Soubory přidá scanner při dalším průchodu knihovnou.</Muted>
          ) : (
            <View style={[styles.list, { borderColor: colors.border }]}>
              {list.map((chapter, index) => {
                const isCurrent = player.chapter?.id === chapter.id
                const playingThis = isCurrent && player.playing
                return (
                  <View
                    key={chapter.id}
                    style={[
                      styles.item,
                      index > 0 && { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border },
                      isCurrent && { backgroundColor: colors.accent },
                    ]}
                  >
                    <Muted size={13} style={{ width: 26 }}>
                      {index + 1}.
                    </Muted>
                    <View style={{ flex: 1, minWidth: 0 }}>
                      <Body size={14} medium numberOfLines={1} style={isCurrent ? { color: colors.primary } : undefined}>
                        {chapter.title}
                      </Body>
                      <Muted size={11} numberOfLines={1} style={{ fontFamily: fonts.sans }}>
                        {chapter.file_name}
                      </Muted>
                    </View>
                    <Muted size={13}>{formatClock(chapter.duration_seconds)}</Muted>
                    <Pressable
                      hitSlop={8}
                      onPress={() => (isCurrent ? void player.toggle() : void player.playBook(book.id, chapter.id))}
                      accessibilityLabel={playingThis ? `Pozastavit kapitolu ${chapter.title}` : `Přehrát kapitolu ${chapter.title}`}
                      style={styles.play}
                    >
                      {playingThis ? <Pause color={colors.foreground} size={16} /> : <Play color={colors.foreground} size={16} />}
                    </Pressable>
                  </View>
                )
              })}
            </View>
          )}
        </View>
      ) : null}
    </View>
  )
}

const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: radius['2xl'], marginTop: spacing.lg },
  head: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, padding: spacing.md },
  body: { paddingHorizontal: spacing.md, paddingBottom: spacing.md },
  list: { borderWidth: 1, borderRadius: radius.md, overflow: 'hidden' },
  item: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm + 4, paddingHorizontal: spacing.sm + 4, paddingVertical: spacing.sm + 2 },
  play: { width: 32, height: 32, alignItems: 'center', justifyContent: 'center' },
})
