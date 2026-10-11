import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { placeholderGradient, placeholderInitial, type TitleKind } from 'sceniter-shared'
import { cn } from 'core-web/src/lib/utils'

/**
 * Plakát 2:3 (nebo záběr 16:9). Bez obrázku kreslí „typografický plakát“:
 * odstín z názvu, velké počáteční písmeno jako vodoznak, štítek druhu a rok.
 */
export function Poster({
  src,
  title,
  kind,
  year,
  className,
  aspect = 'poster',
  compact = false,
}: {
  src?: string
  title: string
  /** Štítek Film/Seriál na náhradním plakátu. */
  kind?: TitleKind
  year?: number
  className?: string
  aspect?: 'poster' | 'still'
  /** Miniatura (řádek seznamu) – bez obrázku jen písmeno, název se nevejde. */
  compact?: boolean
}) {
  const ratio = aspect === 'poster' ? 'aspect-[2/3]' : 'aspect-video'
  const [failed, setFailed] = useState(false)
  if (!src || failed) return <Placeholder title={title} kind={kind} year={year} className={cn(ratio, className)} compact={compact} still={aspect === 'still'} />
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

function Placeholder({
  title,
  kind,
  year,
  className,
  compact,
  still,
}: {
  title: string
  kind?: TitleKind
  year?: number
  className?: string
  compact: boolean
  still: boolean
}) {
  const { t } = useTranslation()
  const [top, mid, bottom] = placeholderGradient(title)
  const initial = placeholderInitial(title)
  return (
    <div
      aria-label={title}
      className={cn(
        '@container relative w-full overflow-hidden rounded-2xl text-white shadow-glass ring-1 ring-white/10 select-none',
        // Jemný odlesk nahoře, ať plocha nepůsobí jako plný barevný blok.
        'before:absolute before:inset-0 before:bg-[radial-gradient(80%_55%_at_20%_0%,rgba(255,255,255,0.16),transparent_70%)]',
        className,
      )}
      style={{
        background: `linear-gradient(165deg, ${top} 0%, ${mid} 55%, ${bottom} 100%)`,
      }}
    >
      {initial ? (
        <span
          aria-hidden
          className={cn(
            'font-heading pointer-events-none absolute leading-none font-bold',
            compact
              ? 'inset-0 flex items-center justify-center text-[70cqw] text-white/80'
              : still
                ? 'top-1/2 right-[-4cqw] -translate-y-1/2 text-[70cqh] text-white/10'
                : 'right-[-6cqw] bottom-[-8cqw] text-[95cqw] text-white/10',
          )}
        >
          {initial}
        </span>
      ) : null}
      {compact ? null : still ? (
        <div className="absolute inset-x-[5cqw] bottom-[6cqh]">
          <p className="font-heading line-clamp-2 text-[6cqw] leading-tight font-semibold text-balance">{title}</p>
        </div>
      ) : (
        <div className="absolute inset-0 flex flex-col p-[7cqw]">
          {kind ? (
            <p className="text-[6cqw] font-medium tracking-[0.2em] text-white/70 uppercase">{t(kind === 'movie' ? 'video.kind.movie' : 'video.kind.show')}</p>
          ) : null}
          <p className="font-heading my-auto line-clamp-4 text-[11cqw] leading-tight font-semibold text-balance">{title}</p>
          {year ? <p className="self-end text-[7cqw] text-white/70 tabular-nums">{year}</p> : null}
        </div>
      )}
    </div>
  )
}
