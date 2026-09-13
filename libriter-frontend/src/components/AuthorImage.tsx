import { UserRoundIcon } from 'lucide-react'
import { useState } from 'react'
import { API_PREFIX } from '@/api/client'
import type { Author } from '@/api/types'
import { cn } from '@/lib/utils'

/**
 * URL fotky autora. Endpoint je veřejný (<img> neumí poslat token),
 * ?v= podle image_path obchází cache prohlížeče po výměně fotky.
 */
function imageUrl(author: Author) {
  return `${API_PREFIX}/authors/${author.id}/image?v=${encodeURIComponent(author.image_path ?? '')}`
}

// Portréty jsou skoro vždy na výšku, ale poměry se liší – čtvercový rám
// drží mřížku zarovnanou stejně jako u obálek knih.
const box = 'relative aspect-square overflow-hidden rounded-full bg-muted'

/**
 * Fotka autora. Bez image_path nebo při chybě načtení zobrazí zástupnou ikonu.
 * Používat s key={author.id}, aby se stav při přechodu mezi autory resetoval.
 */
export function AuthorImage({ author, className }: { author: Author; className?: string }) {
  const [failed, setFailed] = useState(false)

  if (!author.image_path || failed) {
    return (
      <div
        className={cn(box, 'flex items-center justify-center text-muted-foreground', className)}
        aria-hidden
      >
        <UserRoundIcon className="size-1/2 opacity-50" />
      </div>
    )
  }

  return (
    <div className={cn(box, className)}>
      <img
        src={imageUrl(author)}
        alt={`Fotografie autora ${author.name}`}
        loading="lazy"
        decoding="async"
        onError={() => setFailed(true)}
        className="size-full object-cover"
      />
    </div>
  )
}

/** Roky života pro popisek – "1890–1938", "* 1890" nebo prázdno. */
export function lifeYears(author: Author): string {
  if (author.birth_year && author.death_year) return `${author.birth_year}–${author.death_year}`
  if (author.birth_year) return `* ${author.birth_year}`
  if (author.death_year) return `† ${author.death_year}`
  return ''
}
