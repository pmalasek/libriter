import { useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { ChevronDown, Pause, Play, RotateCcw, RotateCw, SkipBack, SkipForward, Timer, X } from 'lucide-react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'
import { formatClock, SKIP_BACK, SKIP_FORWARD, SPEEDS } from 'libriter-shared'

import { BookCover } from '@/components/BookCover'
import { SleepTimerSheet, useSleepTimer } from '@/components/SleepTimer'
import { Body, Heading, Muted } from '@/components/ui/Text'
import { usePlayer } from '@/player/PlayerProvider'
import { fonts, radius, spacing, useTheme } from '@/theme'

/**
 * Celoobrazovkový přehrávač – obdoba `PlayerSheet` / `NowPlayingPanel` na
 * webu. Ovládání je velké a daleko od sebe: používá se poslepu, jednou
 * rukou a často za jízdy.
 */
export default function PlayerScreen() {
  const { colors } = useTheme()
  const player = usePlayer()
  const router = useRouter()
  const insets = useSafeAreaInsets()
  const timer = useSleepTimer()
  const [timerOpen, setTimerOpen] = useState(false)

  if (!player.book) {
    return (
      <View style={[styles.empty, { backgroundColor: colors.background }]}>
        <Muted>Nic se nepřehrává.</Muted>
      </View>
    )
  }

  const chapterIndex = player.chapters.findIndex((item) => item.id === player.chapter?.id)
  const remaining = Math.max(0, player.duration - player.position)
  const ratio = player.duration > 0 ? player.position / player.duration : 0
  const itemIndex = player.session?.items.findIndex((item) => item.book_id === player.book?.id) ?? -1

  return (
    <View
      style={[
        styles.screen,
        { backgroundColor: colors.background, paddingTop: insets.top + spacing.sm, paddingBottom: insets.bottom + spacing.lg },
      ]}
    >
      <View style={styles.topBar}>
        <Pressable hitSlop={12} onPress={() => router.back()} accessibilityLabel="Zavřít přehrávač">
          <ChevronDown color={colors.mutedForeground} size={28} />
        </Pressable>
        <Muted size={11} style={{ textTransform: 'uppercase', letterSpacing: 1 }}>
          {player.offline ? 'Z telefonu' : 'Ze serveru'}
        </Muted>
        <View style={{ flexDirection: 'row', gap: spacing.md }}>
          <Pressable hitSlop={12} onPress={() => setTimerOpen(true)} accessibilityLabel="Časovač vypnutí">
            {timer.remaining != null ? (
              <Body size={13} medium style={{ color: colors.primary }}>
                {formatClock(timer.remaining)}
              </Body>
            ) : (
              <Timer color={colors.mutedForeground} size={22} />
            )}
          </Pressable>
          <Pressable
            hitSlop={12}
            onPress={() => {
              void player.close()
              router.back()
            }}
            accessibilityLabel="Ukončit poslech"
          >
            <X color={colors.mutedForeground} size={22} />
          </Pressable>
        </View>
      </View>

      <View style={styles.coverWrap}>
        <BookCover book={player.book} size={240} downloaded={player.offline} style={styles.coverShadow} />
      </View>

      <View style={styles.texts}>
        <Heading size={22} numberOfLines={2} style={{ textAlign: 'center' }}>
          {player.book.title}
        </Heading>
        <Muted size={14} numberOfLines={1}>
          {player.chapter?.title ?? 'Načítání…'}
          {chapterIndex >= 0 && player.chapters.length > 1 ? ` · ${chapterIndex + 1}/${player.chapters.length}` : ''}
          {player.session && player.session.items.length > 1 && itemIndex >= 0 ? ` · kniha ${itemIndex + 1}/${player.session.items.length}` : ''}
        </Muted>
      </View>

      <View style={styles.progress}>
        <View style={[styles.track, { backgroundColor: colors.border }]}>
          <View style={[styles.fill, { backgroundColor: colors.primary, width: `${Math.min(100, ratio * 100)}%` }]} />
        </View>
        <View style={styles.times}>
          <Muted size={12}>{formatClock(player.position)}</Muted>
          <Muted size={12}>−{formatClock(remaining)}</Muted>
        </View>
      </View>

      <View style={styles.controls}>
        <Pressable hitSlop={10} onPress={() => void player.prevChapter()} accessibilityLabel="Předchozí kapitola">
          <SkipBack color={colors.foreground} size={28} />
        </Pressable>
        <Pressable hitSlop={10} onPress={() => void player.skip(-SKIP_BACK)} accessibilityLabel={`Zpět o ${SKIP_BACK} s`} style={styles.skip}>
          <RotateCcw color={colors.foreground} size={30} />
          <Body size={10} medium style={styles.skipLabel}>
            {SKIP_BACK}
          </Body>
        </Pressable>
        <Pressable
          style={[styles.playButton, { backgroundColor: colors.primary }]}
          onPress={() => void player.toggle()}
          accessibilityLabel={player.playing ? 'Pauza' : 'Přehrát'}
        >
          {player.playing ? (
            <Pause color={colors.primaryForeground} size={32} fill={colors.primaryForeground} />
          ) : (
            <Play color={colors.primaryForeground} size={32} fill={colors.primaryForeground} style={{ marginLeft: 4 }} />
          )}
        </Pressable>
        <Pressable hitSlop={10} onPress={() => void player.skip(SKIP_FORWARD)} accessibilityLabel={`Vpřed o ${SKIP_FORWARD} s`} style={styles.skip}>
          <RotateCw color={colors.foreground} size={30} />
          <Body size={10} medium style={styles.skipLabel}>
            {SKIP_FORWARD}
          </Body>
        </Pressable>
        <Pressable hitSlop={10} onPress={() => void player.nextChapter()} accessibilityLabel="Další kapitola">
          <SkipForward color={colors.foreground} size={28} />
        </Pressable>
      </View>

      <View style={styles.speeds}>
        {SPEEDS.map((value) => {
          const active = player.speed === value
          return (
            <Pressable
              key={value}
              style={[styles.speed, { backgroundColor: active ? colors.primary : colors.secondary }]}
              onPress={() => void player.setSpeed(value)}
              accessibilityState={{ selected: active }}
            >
              <Body size={13} style={{ color: active ? colors.primaryForeground : colors.secondaryForeground, fontFamily: fonts.sansMedium }}>
                {value}×
              </Body>
            </Pressable>
          )
        })}
      </View>

      <SleepTimerSheet open={timerOpen} onClose={() => setTimerOpen(false)} timer={timer} />
    </View>
  )
}

const styles = StyleSheet.create({
  screen: { flex: 1, paddingHorizontal: spacing.lg },
  empty: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  topBar: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  coverWrap: { alignItems: 'center', marginVertical: spacing.lg },
  coverShadow: { shadowColor: '#000', shadowOpacity: 0.25, shadowRadius: 24, shadowOffset: { width: 0, height: 12 }, elevation: 8 },
  texts: { gap: spacing.xs, alignItems: 'center' },
  progress: { marginTop: spacing.lg, gap: spacing.xs },
  track: { height: 4, borderRadius: 2, overflow: 'hidden' },
  fill: { height: 4 },
  times: { flexDirection: 'row', justifyContent: 'space-between' },
  controls: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: spacing.lg },
  skip: { alignItems: 'center', justifyContent: 'center' },
  skipLabel: { position: 'absolute', top: 11 },
  playButton: { width: 76, height: 76, borderRadius: 38, alignItems: 'center', justifyContent: 'center' },
  speeds: { flexDirection: 'row', justifyContent: 'center', flexWrap: 'wrap', gap: spacing.sm, marginTop: spacing.lg },
  speed: { paddingHorizontal: spacing.md, paddingVertical: spacing.xs + 2, borderRadius: radius.lg },
})
