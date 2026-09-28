import { useEffect, useState } from 'react'
import { Modal, StyleSheet, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import { CONTINUE_SERIES_SECONDS, type Book } from 'libriter-shared'

import { radius, spacing, useTheme } from '@/theme'
import { Button } from './ui/Button'
import { Body, Muted, SectionTitle } from './ui/Text'

/**
 * Nabídka dalšího dílu série po doposlechnutí knihy. Kdo ji neodmítne,
 * tomu se po odpočtu další díl sám spustí.
 */
export function ContinueSeriesModal({
  book,
  onContinue,
  onDismiss,
}: {
  book: Book | null
  onContinue: (book: Book) => void
  onDismiss: () => void
}) {
  const { t } = useTranslation()
  const { colors } = useTheme()
  return (
    <Modal visible={book !== null} transparent animationType="fade" onRequestClose={onDismiss}>
      <View style={styles.backdrop}>
        <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.border }]}>
          <SectionTitle>{t('player.continueSeries.title')}</SectionTitle>
          <Body>{t('player.continueSeries.question', { title: book?.title ?? '' })}</Body>
          {book ? <Countdown key={book.id} onDone={() => onContinue(book)} /> : null}
          <View style={styles.actions}>
            <Button variant="ghost" label={t('player.continueSeries.no')} onPress={onDismiss} />
            <Button label={t('player.continueSeries.yes')} onPress={() => book && onContinue(book)} />
          </View>
        </View>
      </View>
    </Modal>
  )
}

/** Odpočet do spuštění; s každou nabídnutou knihou začíná znovu (přes key). */
function Countdown({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation()
  const [seconds, setSeconds] = useState(CONTINUE_SERIES_SECONDS)

  useEffect(() => {
    const timer = setInterval(() => setSeconds((s) => s - 1), 1000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    if (seconds <= 0) onDone()
  }, [seconds, onDone])

  return <Muted>{t('player.continueSeries.countdown', { seconds: Math.max(0, seconds) })}</Muted>
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, backgroundColor: '#000000AA', justifyContent: 'center', padding: spacing.lg },
  card: { borderRadius: radius['4xl'], borderWidth: 1, padding: spacing.lg, gap: spacing.md },
  actions: { flexDirection: 'row', justifyContent: 'flex-end', gap: spacing.sm },
})
