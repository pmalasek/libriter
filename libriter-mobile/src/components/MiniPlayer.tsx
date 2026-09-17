import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Pause, Play, RotateCcw, RotateCw } from 'lucide-react-native'
import { SKIP_BACK, SKIP_FORWARD, formatClock } from 'libriter-shared'

import { usePlayer } from '@/player/PlayerProvider'
import { GlassBackground } from './ui/Blur'
import { radius, spacing, useTheme } from '@/theme'
import { BookCover } from './BookCover'
import { Muted, Title } from './ui/Text'

/**
 * Plovoucí kapsle nad lištou tabů – protějšek `PlayerCapsule` na webu.
 *
 * Pozadí je rozostřené přes `expo-blur`, stejně jako `backdrop-blur` na webu;
 * bez něj by poloprůhledná plocha nechala prosvítat text knih pod sebou.
 * Od obsahu ji navíc drží barevný okraj a výrazný stín.
 */
export function MiniPlayer() {
  const { colors, resolved } = useTheme()
  const player = usePlayer()
  const router = useRouter()

  if (!player.book) return null

  const ratio = player.duration > 0 ? Math.min(1, player.position / player.duration) : 0
  const remaining = Math.max(0, player.duration - player.position)

  return (
    <Pressable
      onPress={() => router.push('/player')}
      style={({ pressed }) => [
        styles.capsule,
        {
          // Jemná skleněná hrana jako `ring-glass-edge` na webu; od obsahu
          // kapsli odděluje stín, ne barevný obrys.
          borderColor: colors.glassEdge,
          shadowOpacity: resolved === 'dark' ? 0.5 : 0.18,
          opacity: pressed ? 0.9 : 1,
        },
      ]}
      accessibilityRole="button"
      accessibilityLabel={`Otevřít přehrávač: ${player.book.title}`}
    >
      <GlassBackground intensity={80} strong />

      <View style={styles.row}>
        <BookCover book={player.book} size={46} rounded={radius.lg} downloaded={player.offline} />

        <View style={styles.texts}>
          <Title numberOfLines={1} size={14}>
            {player.book.title}
          </Title>
          <Muted numberOfLines={1} size={12}>
            {player.chapter?.title ?? 'Načítání…'}
            {player.duration > 0 ? ` · −${formatClock(remaining)}` : ''}
          </Muted>
        </View>

        <Pressable hitSlop={8} onPress={() => void player.skip(-SKIP_BACK)} accessibilityLabel={`Zpět o ${SKIP_BACK} s`}>
          <RotateCcw color={colors.foreground} size={21} />
        </Pressable>
        <Pressable
          hitSlop={8}
          onPress={() => void player.toggle()}
          style={[styles.play, { backgroundColor: colors.primary }]}
          accessibilityLabel={player.playing ? 'Pauza' : 'Přehrát'}
        >
          {player.playing ? (
            <Pause color={colors.primaryForeground} size={19} fill={colors.primaryForeground} />
          ) : (
            <Play color={colors.primaryForeground} size={19} fill={colors.primaryForeground} style={{ marginLeft: 2 }} />
          )}
        </Pressable>
        <Pressable hitSlop={8} onPress={() => void player.skip(SKIP_FORWARD)} accessibilityLabel={`Vpřed o ${SKIP_FORWARD} s`}>
          <RotateCw color={colors.foreground} size={21} />
        </Pressable>
      </View>

      {/* Ukazatel postupu je odsazený a zakulacený – pruh přes celou spodní
          hranu vypadal jako nalepený proužek, ne jako součást kapsle. */}
      <View style={[styles.track, { backgroundColor: colors.muted }]}>
        <View style={[styles.fill, { width: `${ratio * 100}%`, backgroundColor: colors.primary }]} />
      </View>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  capsule: {
    marginHorizontal: spacing.sm + 4,
    marginBottom: spacing.sm,
    borderRadius: radius['3xl'],
    borderWidth: StyleSheet.hairlineWidth,
    paddingVertical: spacing.sm + 2,
    overflow: 'hidden',
    shadowColor: '#000',
    shadowRadius: 18,
    shadowOffset: { width: 0, height: 10 },
    elevation: 10,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm + 4,
    paddingHorizontal: spacing.sm + 2,
  },
  texts: { flex: 1, minWidth: 0, gap: 1 },
  play: { width: 40, height: 40, borderRadius: 20, alignItems: 'center', justifyContent: 'center' },
  track: {
    height: 3,
    borderRadius: 2,
    overflow: 'hidden',
    marginTop: spacing.sm + 2,
    marginHorizontal: spacing.sm + 2,
  },
  fill: { height: 3, borderRadius: 2 },
})
