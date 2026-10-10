import { cn } from 'core-web/src/lib/utils'
import { Poster } from './Poster'

/**
 * Hlavička detailu: rozmazané pozadí (backdrop), plakát a obsah vedle něj.
 */
export function Hero({
  backdrop,
  poster,
  title,
  cover,
  children,
}: {
  backdrop?: string
  poster?: string
  title: string
  /** Vlastní obrázek místo plakátu (fotka osoby). */
  cover?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="relative -mx-4 -mt-4 overflow-hidden px-4 pt-6 pb-8 sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8 md:rounded-b-4xl">
      {backdrop ? (
        <>
          <img src={backdrop} alt="" aria-hidden className="absolute inset-0 -z-20 size-full object-cover opacity-60" />
          <div className="absolute inset-0 -z-10 bg-gradient-to-t from-background via-background/85 to-background/40" />
        </>
      ) : null}
      <div className={cn('flex flex-col gap-6 sm:flex-row sm:items-end', backdrop && 'pt-24 sm:pt-40')}>
        <div className="w-40 shrink-0 sm:w-52">
          {cover ?? <Poster src={poster} title={title} className="shadow-glass-lg" />}
        </div>
        <div className="min-w-0 flex-1">{children}</div>
      </div>
    </section>
  )
}

/** Řádek metadat pod titulkem: „2003 · 124 min · Drama, Krimi“. */
export function MetaLine({ items }: { items: (string | undefined | null | false)[] }) {
  const parts = items.filter(Boolean) as string[]
  if (parts.length === 0) return null
  return <p className="mt-2 text-sm text-muted-foreground">{parts.join(' · ')}</p>
}
