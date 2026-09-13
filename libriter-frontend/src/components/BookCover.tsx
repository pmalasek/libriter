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

const box = 'aspect-2/3 w-full rounded-lg bg-muted'

/**
 * Obálka knihy; bez cover_path nebo při chybě načtení zobrazí zástupnou ikonu.
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

  return (
    <img
      src={coverUrl(book)}
      alt={`Obálka knihy ${book.title}`}
      loading="lazy"
      decoding="async"
      onError={() => setFailed(true)}
      className={cn(box, 'object-cover', className)}
    />
  )
}
