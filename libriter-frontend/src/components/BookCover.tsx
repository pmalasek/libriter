import { BookHeadphonesIcon } from 'lucide-react'
import { useState } from 'react'
import { API_PREFIX } from '@/api/client'
import type { Book } from '@/api/types'
import { cn } from '@/lib/utils'

/**
 * URL obálky knihy. Endpoint je veřejný (<img> neumí poslat token),
 * ?v= podle updated_at obchází cache prohlížeče po změně obálky.
 */
function coverUrl(book: Book) {
  return `${API_PREFIX}/books/${book.id}/cover?v=${encodeURIComponent(book.updated_at)}`
}

// Obálky mají různé poměry stran (zhruba 60 % čtverec, 30 % portrét, zbytek na šířku),
// proto čtvercový rám - drží mřížku zarovnanou a odpovídá mediánu sbírky.
const box = 'relative aspect-square w-full overflow-hidden rounded-lg bg-muted'

/**
 * Obálka knihy. Vždy je vidět celá (object-contain); prázdné místo kolem
 * vyplní rozmazaná kopie téže obálky, aby nevznikaly šedé pruhy.
 * Bez cover_path nebo při chybě načtení zobrazí zástupnou ikonu.
 * Používat s key={book.id}, aby se stav při přechodu mezi knihami resetoval.
 */
export function BookCover({ book, className }: { book: Book; className?: string }) {
  const [failed, setFailed] = useState(false)

  if (!book.cover_path || failed) {
    return (
      <div
        className={cn(box, 'flex items-center justify-center text-muted-foreground', className)}
        aria-hidden
      >
        <BookHeadphonesIcon className="size-8 opacity-60" />
      </div>
    )
  }

  const src = coverUrl(book)

  return (
    <div className={cn(box, className)}>
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
