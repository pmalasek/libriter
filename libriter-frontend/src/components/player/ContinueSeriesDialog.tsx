import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Book } from '@/api/types'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { CONTINUE_SERIES_SECONDS } from '@/player/playerContext'

/**
 * Nabídka dalšího dílu série po doposlechnutí knihy. Kdo ji neodmítne,
 * tomu se po odpočtu další díl sám spustí.
 */
export function ContinueSeriesDialog({
  book,
  onContinue,
  onDismiss,
}: {
  book: Book | null
  onContinue: (book: Book) => void
  onDismiss: () => void
}) {
  const { t } = useTranslation()

  return (
    <AlertDialog open={book !== null} onOpenChange={(open) => !open && onDismiss()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('player.continueSeries.title')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('player.continueSeries.question', { title: book?.title ?? '' })}{' '}
            {book && <Countdown key={book.id} onDone={() => onContinue(book)} />}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('player.continueSeries.no')}</AlertDialogCancel>
          <AlertDialogAction onClick={() => book && onContinue(book)}>
            {t('player.continueSeries.yes')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
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

  return <>{t('player.continueSeries.countdown', { seconds: Math.max(0, seconds) })}</>
}
