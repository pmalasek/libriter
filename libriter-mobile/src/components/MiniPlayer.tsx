import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Pause, Play, RotateCcw, RotateCw } from 'lucide-react-native'
import { SKIP_BACK, SKIP_FORWARD } from 'libriter-shared'

import { usePlayer } from '@/player/PlayerProvider'
import { radius, spacing, useTheme } from '@/theme'
import { BookCover } from './BookCover'
import { Muted, Title } from './ui/Text'

/**
 * Plovoucí kapsle nad lištou tabů – jako `PlayerCapsule` na webu: obálka,
 * název a kapitola, skok vzad/vpřed, play/pause a proužek postupu.
 * Klepnutím se otevře celý přehrávač.
 */
export function MiniPlayer() {
  const { colors } = useTheme()
  const player = usePlayer()
  const router = useRouter()

  if (!player.book) return null

  const ratio = player.duration > 0 ? Math.min(1, player.position / player.duration) : 0

  return (
    <Pressable
      onPress={() => router.push('/player')}
      style={[styles.capsule, { backgroundColor: colors.glassStrong, borderColor: colors.glassEdge }]}
      accessibilityRole="button"
      accessibilityLabel="Otevřít přehrávač"
    >
      <View style={styles.row}>
        <BookCover book={player.book} size={40} rounded={radius.md} downloaded={player.offline} />
        <View style={{ flex: 1, minWidth: 0 }}>
          <Title numberOfLines={1} size={13}>
            {player.book.title}
          </Title>
          <Muted numberOfLines={1} size={11}>
            {player.chapter?.title ?? 'Načítání…'}
          </Muted>
        </View>
        <Pressable hitSlop={8} onPress={() => void player.skip(-SKIP_BACK)} accessibilityLabel={`Zpět o ${SKIP_BACK} s`}>
          <RotateCcw color={colors.foreground} size={20} />
        </Pressable>
        <Pressable
          hitSlop={8}
          onPress={() => void player.toggle()}
          style={[styles.play, { backgroundColor: colors.primary }]}
          accessibilityLabel={player.playing ? 'Pauza' : 'Přehrát'}
        >
          {player.playing ? (
            <Pause color={colors.primaryForeground} size={18} fill={colors.primaryForeground} />
          ) : (
            <Play color={colors.primaryForeground} size={18} fill={colors.primaryForeground} />
          )}
        </Pressable>
        <Pressable hitSlop={8} onPress={() => void player.skip(SKIP_FORWARD)} accessibilityLabel={`Vpřed o ${SKIP_FORWARD} s`}>
          <RotateCw color={colors.foreground} size={20} />
        </Pressable>
      </View>
      <View style={[styles.track, { backgroundColor: colors.border }]}>
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
    borderWidth: 1,
    padding: spacing.sm + 2,
    gap: spacing.sm,
    overflow: 'hidden',
    shadowColor: '#000',
    shadowOpacity: 0.12,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 8 },
    elevation: 5,
  },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm + 4 },
  play: { width: 36, height: 36, borderRadius: 18, alignItems: 'center', justifyContent: 'center' },
  track: { height: 3, borderRadius: 2, overflow: 'hidden' },
  fill: { height: 3 },
})
