import { LayersIcon } from 'lucide-react'
import type { Book } from '@/api/types'
import { BookCover } from '@/components/BookCover'
import { cn } from '@/lib/utils'

/**
 * Geometrie stohu. Rám má pevnou šířku bez ohledu na počet obálek, aby
 * text na všech kartách sérií začínal ve stejném místě. Obálky jsou uvnitř
 * absolutně, protože záporný margin v procentech se počítá z šířky rodiče.
 */
const VARIANTS = {
  sm: {
    frame: 'h-14 w-26',
    cover: 'size-14',
    offsets: ['left-0', 'left-6', 'left-12'],
    tilts: ['-rotate-6', 'rotate-0', 'rotate-6'],
    icon: 'size-7',
  },
  lg: {
    frame: 'h-24 w-44',
    cover: 'size-24',
    offsets: ['left-0', 'left-10', 'left-20'],
    tilts: ['-rotate-6', 'rotate-0', 'rotate-6'],
    icon: 'size-12',
  },
} as const

/**
 * Stoh prvních dílů série – až tři překrývající se obálky. Bez knih se
 * zobrazí zástupný rám s ikonou vrstev.
 */
export function SeriesCoverStack({
  books,
  className,
  variant = 'sm',
}: {
  books: Book[]
  className?: string
  variant?: keyof typeof VARIANTS
}) {
  const style = VARIANTS[variant]
  const shown = books.slice(0, 3)

  return (
    <div className={cn('relative shrink-0', style.frame, className)} aria-hidden>
      {shown.length === 0 ? (
        <div
          className={cn(
            style.cover,
            'flex items-center justify-center rounded-2xl bg-secondary text-primary/60',
          )}
        >
          <LayersIcon className={style.icon} />
        </div>
      ) : (
        shown.map((book, index) => (
          <div
            key={book.id}
            className={cn('absolute top-0', style.cover, style.offsets[index], style.tilts[index])}
          >
            <BookCover book={book} lift={false} className="rounded-xl shadow-md" />
          </div>
        ))
      )}
    </div>
  )
}
