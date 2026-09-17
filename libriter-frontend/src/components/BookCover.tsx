import { BookHeadphonesIcon } from 'lucide-react'
import { useState } from 'react'
import { API_PREFIX } from '@/api/client'
import type { Book } from '@/api/types'
import { cn } from '@/lib/utils'

/**
 * URL obálky knihy. Endpoint je veřejný (<img> neumí poslat token),
 * ?v= podle updated_at obchází cache prohlížeče po změně obálky.
 */
export function coverUrl(book: Book) {
  return `${API_PREFIX}/books/${book.id}/cover?v=${encodeURIComponent(book.updated_at)}`
}

// Obálky mají různé poměry stran (zhruba 60 % čtverec, 30 % portrét, zbytek na šířku),
// proto čtvercový rám - drží mřížku zarovnanou a odpovídá mediánu sbírky.
// after:* kreslí lesklou vnitřní hranu; inset stín by skryl <img> nad ním.
const box =
  'relative aspect-square w-full overflow-hidden rounded-2xl bg-foreground/8 ring-1 ring-foreground/8 after:pointer-events-none after:absolute after:inset-0 after:z-10 after:rounded-[inherit] after:ring-1 after:ring-inset after:ring-white/20 dark:after:ring-white/10'

// Uvnitř skupiny (dlaždice knihy) se obálka při najetí nadzvedne; v řádku
// seznamu a ve velké hlavičce detailu se místo toho použije lift={false}.
const lifted =
  'shadow-glass transition duration-200 group-hover:-translate-y-1 group-hover:shadow-glass-lg motion-reduce:transition-none'

/**
 * Obálka knihy. Vždy je vidět celá (object-contain); prázdné místo kolem
 * vyplní rozmazaná kopie téže obálky, aby nevznikaly šedé pruhy.
 * Bez cover_path nebo při chybě načtení zobrazí zástupnou ikonu.
 * Používat s key={book.id}, aby se stav při přechodu mezi knihami resetoval.
 */
export function BookCover({
  book,
  className,
  lift = true,
}: {
  book: Book
  className?: string
  /** Nadzvednutí při najetí na kartu; vypnout v seznamu a v hlavičce detailu. */
  lift?: boolean
}) {
  const [failed, setFailed] = useState(false)

  if (!book.cover_path || failed) {
    return (
      <div
        className={cn(
          box,
          lift && lifted,
          'flex items-center justify-center bg-secondary text-primary/60',
          className,
        )}
        aria-hidden
      >
        <BookHeadphonesIcon className="size-1/3" />
      </div>
    )
  }

  const src = coverUrl(book)

  return (
    <div className={cn(box, lift && lifted, className)}>
      {/* Výplň pozadí - stejné URL, takže se stahuje jen jednou.
          scale-110 schová rozmazané okraje za hranu rámu. */}
      <img
        src={src}
        alt=""
        aria-hidden
        loading="lazy"
        decoding="async"
        className="absolute inset-0 size-full scale-110 object-cover blur-xl"
      />
      <img
        src={src}
        alt={`Obálka knihy ${book.title}`}
        loading="lazy"
        decoding="async"
        onError={() => setFailed(true)}
        className="relative size-full object-contain"
      />
    </div>
  )
}
