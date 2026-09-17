import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { CheckCircle2, Pause, Play, Trash2 } from 'lucide-react-native'
import {
  formatClock,
  formatDateTime,
  sessionKindLabel,
  sessionProgress,
  type Book,
  type PlaySession,
} from 'libriter-shared'

import { usePlayer } from '@/player/PlayerProvider'
import { radius, spacing, useTheme } from '@/theme'
import { BookCover } from './BookCover'
import { SeriesCoverStack } from './SeriesCoverStack'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { Body, Muted } from './ui/Text'

/** Karta poslechu na stránce Právě posloucháno – jako `SessionCard` na webu. */
export function SessionCard({
  session,
  title,
  books,
  bookById,
  onRemove,
}: {
  session: PlaySession
  title: string
  books: Book[]
  bookById: Map<string, Book>
  onRemove: () => void
}) {
  const { colors } = useTheme()
  const router = useRouter()
  const player = usePlayer()
  const progress = sessionProgress(session)
  const currentBook = progress.bookId ? bookById.get(progress.bookId) : undefined

  // Otevřený poslech se odtud jen pozastaví a rozjede; načítat ho znovu ze
  // serveru by zahodilo pozici, kterou přehrávač právě drží.
  const isOpen = player.session?.id === session.id
  const isPlaying = isOpen && player.playing

  return (
    <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.border }]}>
      <View style={styles.top}>
        {books.length > 1 ? (
          <SeriesCoverStack books={books} />
        ) : currentBook ? (
          <Pressable onPress={() => router.push(`/book/${currentBook.id}`)}>
            <BookCover book={currentBook} size={56} rounded={radius.xl} />
          </Pressable>
        ) : (
          <View style={{ width: 56, height: 56, borderRadius: radius.xl, backgroundColor: colors.muted }} />
        )}

        <View style={{ flex: 1, minWidth: 0, gap: 4 }}>
          <Body medium numberOfLines={2}>
            {title}
          </Body>
          <View style={styles.badges}>
            <Badge label={sessionKindLabel(session)} />
            {session.finished_at ? <Badge variant="outline" icon={CheckCircle2} label="Doposlechnuto" /> : null}
          </View>
        </View>
      </View>

      <Muted size={13} numberOfLines={2}>
        {progress.bookCount > 1 ? `Kniha ${progress.bookNumber} z ${progress.bookCount} · ` : ''}
        {currentBook ? currentBook.title : 'Kniha už není v knihovně'}
        {progress.positionSeconds > 0 ? ` · ${formatClock(progress.positionSeconds)}` : ''}
      </Muted>
      <Muted size={12}>Naposledy {formatDateTime(session.updated_at)}</Muted>

      <View style={styles.actions}>
        <Button
          icon={isPlaying ? Pause : Play}
          label={isPlaying ? 'Pozastavit' : isOpen ? 'Přehrát' : 'Pokračovat'}
          onPress={() => (isOpen ? void player.toggle() : void player.switchSession(session.id))}
          disabled={player.loading}
          style={{ flex: 1 }}
        />
        <Button variant="ghost" size="icon" icon={Trash2} onPress={onRemove} accessibilityLabel={`Odebrat poslech ${title}`} />
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: radius['2xl'], padding: spacing.md, gap: spacing.sm },
  top: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.xs },
  actions: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, marginTop: spacing.xs },
})
