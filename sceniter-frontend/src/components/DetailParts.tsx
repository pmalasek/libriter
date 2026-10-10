import { useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { TitleKind } from 'sceniter-shared'
import { Badge } from 'core-web/src/components/ui/badge'
import { Skeleton } from 'core-web/src/components/ui/skeleton'
import { cn } from 'core-web/src/lib/utils'
import { useSimilar } from '@/api/hooks'
import { PosterCard } from './PosterCard'
import { Shelf, ShelfItem } from './Shelf'

/** Žánry jako štítky – odkaz na výpis knihovny filtrovaný na žánr. */
export function GenreChips({ genres, libraryId }: { genres: string[]; libraryId: string }) {
  if (genres.length === 0) return null
  return (
    <div className="mt-3 flex flex-wrap gap-1.5">
      {genres.map((g) => (
        <Badge key={g} variant="secondary" asChild className="h-6 px-2.5 text-[13px]">
          <Link to={`/library/${libraryId}?genre=${encodeURIComponent(g)}`}>{g}</Link>
        </Badge>
      ))}
    </div>
  )
}

/** Dlouhý text sbalený na 4 řádky s tlačítkem Více / Méně. */
export function Expandable({ text, className }: { text: string; className?: string }) {
  const { t } = useTranslation()
  const ref = useRef<HTMLParagraphElement>(null)
  const [open, setOpen] = useState(false)
  const [clamped, setClamped] = useState(false)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el || open) return
    const check = () => setClamped(el.scrollHeight > el.clientHeight + 1)
    check()
    const ro = new ResizeObserver(check)
    ro.observe(el)
    return () => ro.disconnect()
  }, [text, open])

  return (
    <div className={cn('max-w-3xl', className)}>
      <p ref={ref} className={cn('leading-relaxed', !open && 'line-clamp-4')}>
        {text}
      </p>
      {clamped || open ? (
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
          className="mt-1 text-sm font-medium text-primary underline-offset-4 hover:underline"
        >
          {open ? t('video.detail.less') : t('video.detail.more')}
        </button>
      ) : null}
    </div>
  )
}

/** Police „Podobné tituly“ – ze stejné knihovny se společnými žánry. */
export function SimilarShelf({ kind, id }: { kind: TitleKind; id: string }) {
  const { t } = useTranslation()
  const similar = useSimilar(kind, id)
  const items = similar.data
  if (!items || items.movies.length + items.shows.length === 0) return null
  return (
    <Shelf title={t('video.detail.similar')}>
      {items.movies.map((m) => (
        <ShelfItem key={m.id}>
          <PosterCard movie={m} />
        </ShelfItem>
      ))}
      {items.shows.map((s) => (
        <ShelfItem key={s.id}>
          <PosterCard show={s} />
        </ShelfItem>
      ))}
    </Shelf>
  )
}

/** Kostra detailu během načítání (hlavička, popis, police). */
export function DetailSkeleton() {
  return (
    <div aria-busy className="pt-6">
      <div className="flex flex-col gap-6 sm:flex-row sm:items-end">
        <Skeleton className="aspect-[2/3] w-40 shrink-0 rounded-2xl sm:w-52" />
        <div className="flex-1 space-y-3">
          <Skeleton className="h-9 w-2/3" />
          <Skeleton className="h-4 w-1/3" />
          <div className="flex gap-1.5">
            <Skeleton className="h-6 w-16 rounded-4xl" />
            <Skeleton className="h-6 w-20 rounded-4xl" />
          </div>
          <div className="flex gap-2 pt-2">
            <Skeleton className="h-10 w-32" />
            <Skeleton className="h-10 w-10" />
            <Skeleton className="h-10 w-10" />
          </div>
        </div>
      </div>
      <div className="mt-8 max-w-3xl space-y-2">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-4/5" />
      </div>
      <Skeleton className="mt-10 h-6 w-48" />
      <div className="mt-3 flex gap-4 overflow-hidden">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="aspect-[2/3] w-32 shrink-0 rounded-2xl sm:w-40" />
        ))}
      </div>
    </div>
  )
}
