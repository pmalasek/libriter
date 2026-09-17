import { useState } from 'react'
import { Pressable, StyleSheet, Text, View } from 'react-native'
import { useRouter } from 'expo-router'
import { useSafeAreaInsets } from 'react-native-safe-area-context'
import { SKIP_BACK, SKIP_FORWARD, SPEEDS } from 'libriter-shared'

import { BookCover } from '@/components/BookCover'
import { SleepTimerSheet, useSleepTimer } from '@/components/SleepTimer'
import { usePlayer } from '@/player/PlayerProvider'
import { colors, formatDuration, radius, spacing } from '@/theme'

/**
 * Celoobrazovkový přehrávač. Ovládání je velké a daleko od sebe: používá se
 * poslepu, jednou rukou a často za jízdy.
 */
export default function PlayerScreen() {
  const player = usePlayer()
  const router = useRouter()
  const insets = useSafeAreaInsets()
  const timer = useSleepTimer()
  const [timerOpen, setTimerOpen] = useState(false)

  if (!player.book) {
    return (
      <View style={styles.empty}>
        <Text style={styles.emptyText}>Nic se nepřehrává.</Text>
      </View>
    )
  }

  const chapterIndex = player.chapters.findIndex((item) => item.id === player.chapter?.id)
  const remaining = Math.max(0, player.duration - player.position)

  return (
    <View style={[styles.screen, { paddingTop: insets.top + spacing.sm, paddingBottom: insets.bottom + spacing.lg }]}>
      <View style={styles.topBar}>
        <Pressable hitSlop={12} onPress={() => router.back()}>
          <Text style={styles.chevron}>⌄</Text>
        </Pressable>
        <Text style={styles.source}>{player.offline ? 'Z telefonu' : 'Ze serveru'}</Text>
        <Pressable hitSlop={12} onPress={() => setTimerOpen(true)}>
          <Text style={[styles.timer, timer.remaining != null && styles.timerActive]}>
            {timer.remaining != null ? formatDuration(timer.remaining) : '⏱'}
          </Text>
        </Pressable>
      </View>

      <View style={styles.coverWrap}>
        <BookCover
          bookId={player.book.id}
          title={player.book.title}
          size={220}
          downloaded={player.offline}
        />
      </View>

      <View style={styles.texts}>
        <Text style={styles.title} numberOfLines={2}>
          {player.book.title}
        </Text>
        <Text style={styles.subtitle} numberOfLines={1}>
          {player.chapter?.title ?? 'Načítání…'}
          {chapterIndex >= 0 && player.chapters.length > 1
            ? ` · ${chapterIndex + 1}/${player.chapters.length}`
            : ''}
        </Text>
      </View>

      <View style={styles.progress}>
        <View style={styles.track}>
          <View
            style={[
              styles.fill,
              { width: `${player.duration > 0 ? (player.position / player.duration) * 100 : 0}%` },
            ]}
          />
        </View>
        <View style={styles.times}>
          <Text style={styles.time}>{formatDuration(player.position)}</Text>
          <Text style={styles.time}>−{formatDuration(remaining)}</Text>
        </View>
      </View>

      <View style={styles.controls}>
        <Control glyph="⏮" label="Předchozí kapitola" onPress={() => void player.prevChapter()} />
        <Control glyph={`−${SKIP_BACK}`} label="Zpět" onPress={() => void player.skip(-SKIP_BACK)} small />
        <Pressable
          style={styles.playButton}
          onPress={() => void player.toggle()}
          accessibilityLabel={player.playing ? 'Pauza' : 'Přehrát'}
        >
          <Text style={styles.playGlyph}>{player.playing ? '⏸' : '▶'}</Text>
        </Pressable>
        <Control glyph={`+${SKIP_FORWARD}`} label="Vpřed" onPress={() => void player.skip(SKIP_FORWARD)} small />
        <Control glyph="⏭" label="Další kapitola" onPress={() => void player.nextChapter()} />
      </View>

      <View style={styles.speeds}>
        {SPEEDS.map((value) => (
          <Pressable
            key={value}
            style={[styles.speed, player.speed === value && styles.speedActive]}
            onPress={() => void player.setSpeed(value)}
          >
            <Text style={[styles.speedText, player.speed === value && styles.speedTextActive]}>
              {value}×
            </Text>
          </Pressable>
        ))}
      </View>

      <SleepTimerSheet open={timerOpen} onClose={() => setTimerOpen(false)} timer={timer} />
    </View>
  )
}

function Control({
  glyph,
  label,
  onPress,
  small = false,
}: {
  glyph: string
  label: string
  onPress: () => void
  small?: boolean
}) {
  return (
    <Pressable hitSlop={10} onPress={onPress} accessibilityLabel={label}>
      <Text style={[styles.control, small && styles.controlSmall]}>{glyph}</Text>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.background, paddingHorizontal: spacing.lg },
  empty: { flex: 1, backgroundColor: colors.background, alignItems: 'center', justifyContent: 'center' },
  emptyText: { color: colors.textMuted },
  topBar: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  chevron: { color: colors.textMuted, fontSize: 28, lineHeight: 28 },
  source: { color: colors.textMuted, fontSize: 12, textTransform: 'uppercase', letterSpacing: 1 },
  timer: { color: colors.textMuted, fontSize: 18 },
  timerActive: { color: colors.accent, fontSize: 14 },
  coverWrap: { alignItems: 'center', marginVertical: spacing.lg },
  texts: { gap: spacing.xs, alignItems: 'center' },
  title: { color: colors.text, fontSize: 22, fontWeight: '700', textAlign: 'center' },
  subtitle: { color: colors.textMuted, fontSize: 14 },
  progress: { marginTop: spacing.lg, gap: spacing.xs },
  track: { height: 4, backgroundColor: colors.border, borderRadius: 2 },
  fill: { height: 4, backgroundColor: colors.accent, borderRadius: 2 },
  times: { flexDirection: 'row', justifyContent: 'space-between' },
  time: { color: colors.textMuted, fontSize: 12 },
  controls: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: spacing.lg,
  },
  control: { color: colors.text, fontSize: 26 },
  controlSmall: { fontSize: 16 },
  playButton: {
    backgroundColor: colors.accent,
    width: 72,
    height: 72,
    borderRadius: 36,
    alignItems: 'center',
    justifyContent: 'center',
  },
  playGlyph: { color: colors.accentText, fontSize: 30 },
  speeds: {
    flexDirection: 'row',
    justifyContent: 'center',
    flexWrap: 'wrap',
    gap: spacing.sm,
    marginTop: spacing.lg,
  },
  speed: {
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs,
    borderRadius: radius.sm,
    backgroundColor: colors.surfaceAlt,
  },
  speedActive: { backgroundColor: colors.accent },
  speedText: { color: colors.textMuted, fontSize: 13 },
  speedTextActive: { color: colors.accentText, fontWeight: '600' },
})
