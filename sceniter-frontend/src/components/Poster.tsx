import { FilmIcon } from 'lucide-react'
import { useState } from 'react'
import { cn } from 'core-web/src/lib/utils'

/**
 * Plakát 2:3. Bez obrázku (nespárovaný titul) kreslí barevnou plochu
 * s názvem, ať mřížka drží tvar.
 */
export function Poster({
  src,
  title,
  className,
  aspect = 'poster',
}: {
  src?: string
  title: string
  className?: string
  aspect?: 'poster' | 'still'
}) {
  const [failed, setFailed] = useState(false)
  const ratio = aspect === 'poster' ? 'aspect-[2/3]' : 'aspect-video'
  if (!src || failed) {
    return (
      <div
        className={cn(
          ratio,
          'bg-brand-gradient relative flex w-full flex-col items-center justify-center gap-2 overflow-hidden rounded-2xl p-3 text-center text-primary-foreground shadow-glass ring-1 ring-white/15',
          className,
        )}
      >
        <FilmIcon className="size-7 opacity-70" />
        <span className="font-heading line-clamp-4 text-sm leading-tight font-semibold text-balance">{title}</span>
      </div>
    )
  }
  return (
    <img
      src={src}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
      className={cn(ratio, 'w-full rounded-2xl bg-muted object-cover shadow-glass ring-1 ring-glass-edge', className)}
    />
  )
}
