import { Pressable, StyleSheet, Text, View } from 'react-native'
import { useRouter } from 'expo-router'

import { usePlayer } from '@/player/PlayerProvider'
import { BookCover } from './BookCover'
import { colors, spacing } from '@/theme'

/**
 * Lišta nad taby. Ukazuje, co hraje, a je zároveň cestou k celému
 * přehrávači – bez ní by se uživatel k rozehrané knize proklikával knihovnou.
 */
export function MiniPlayer() {
  const player = usePlayer()
  const router = useRouter()

  if (!player.book) return null

  const progress = player.duration > 0 ? player.position / player.duration : 0

  return (
    <View style={styles.wrap}>
      <View style={styles.track}>
        <View style={[styles.fill, { width: `${Math.min(100, progress * 100)}%` }]} />
      </View>

      <Pressable style={styles.row} onPress={() => router.push('/player')}>
        <BookCover bookId={player.book.id} title={player.book.title} size={38} downloaded={player.offline} />

        <View style={styles.texts}>
          <Text style={styles.title} numberOfLines={1}>
            {player.book.title}
          </Text>
          <Text style={styles.subtitle} numberOfLines={1}>
            {player.chapter?.title ?? 'Načítání…'}
          </Text>
        </View>

        <Pressable
          hitSlop={12}
          onPress={() => void player.toggle()}
          style={styles.button}
          accessibilityLabel={player.playing ? 'Pauza' : 'Přehrát'}
        >
          <Text style={styles.glyph}>{player.playing ? '⏸' : '▶'}</Text>
        </Pressable>
      </Pressable>
    </View>
  )
}

const styles = StyleSheet.create({
  wrap: { backgroundColor: colors.surfaceAlt, borderTopColor: colors.border, borderTopWidth: 1 },
  track: { height: 2, backgroundColor: colors.border },
  fill: { height: 2, backgroundColor: colors.accent },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, padding: spacing.sm },
  texts: { flex: 1 },
  title: { color: colors.text, fontSize: 14, fontWeight: '600' },
  subtitle: { color: colors.textMuted, fontSize: 12 },
  button: { paddingHorizontal: spacing.sm },
  glyph: { color: colors.text, fontSize: 22 },
})
