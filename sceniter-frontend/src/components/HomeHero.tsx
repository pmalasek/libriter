import { BookmarkCheckIcon, BookmarkIcon, CheckIcon, ChevronLeftIcon, ChevronRightIcon, InfoIcon, PlayIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from 'react-router'
import { toast } from 'sonner'
import { backdropUrl, isUhd, type HomeLibrary, type Movie, type Show } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import { cn } from 'core-web/src/lib/utils'
import { useMovie, useSetWatched, useSetWatchlist, useShow } from '@/api/hooks'

/** Kolik titulů se v banneru střídá. */
const SLIDES = 5
/** Jak dlouho jeden titul vydrží, než banner přejde na další (ms). */
const INTERVAL = 9000

type Featured = { kind: 'movie'; title: Movie } | { kind: 'show'; title: Show }

/**
 * Doporučené tituly pro banner: nejnovější s obrázkem pozadí, střídavě
 * z jednotlivých knihoven, ať banner neukazuje jen jednu z nich.
 */
export function pickFeatured(rows: HomeLibrary[]): Featured[] {
  const queues: Featured[][] = rows.map((row) => [
    ...row.movies.filter((m) => m.backdrop_path).map((title) => ({ kind: 'movie' as const, title })),
    ...row.shows.filter((s) => s.backdrop_path).map((title) => ({ kind: 'show' as const, title })),
  ])
  const out: Featured[] = []
  for (let i = 0; out.length < SLIDES && queues.some((q) => q.length > i); i++) {
    for (const q of queues) {
      if (q[i] && out.length < SLIDES) out.push(q[i])
    }
  }
  return out
}

function runtime(title: Movie | Show, kind: Featured['kind']): string | null {
  if (kind !== 'movie') return null
  const movie = title as Movie
  const minutes = movie.runtime_min ?? (movie.duration_sec ? Math.round(movie.duration_sec / 60) : 0)
  if (!minutes) return null
  const h = Math.floor(minutes / 60)
  return h > 0 ? `${h} h ${minutes % 60} min` : `${minutes} min`
}

/**
 * Banner na domovské stránce: pozadí titulu přes celou šířku, které se
 * vlevo a dole rozpouští do stránky, název, popis a hlavní akce. Tituly se
 * samy střídají; při najetí myší, fokusu a omezení pohybu banner stojí.
 */
export function HomeHero({ rows }: { rows: HomeLibrary[] }) {
  const { t } = useTranslation()
  const slides = pickFeatured(rows)
  const [index, setIndex] = useState(0)
  const [paused, setPaused] = useState(false)
  const count = slides.length
  const current = slides[Math.min(index, count - 1)]

  useEffect(() => {
    if (count < 2 || paused || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    const timer = window.setTimeout(() => setIndex((i) => (i + 1) % count), INTERVAL)
    return () => window.clearTimeout(timer)
  }, [count, paused, index])

  if (!current) return null
  const go = (delta: number) => setIndex((i) => (i + delta + count) % count)

  return (
    <section
      aria-roledescription="carousel"
      aria-label={t('video.home.featured')}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
      className="relative -mx-4 -mt-4 mb-2 overflow-hidden sm:-mx-6 lg:-mx-8"
    >
      {/* Všechna pozadí jsou v DOM a prolínají se průhledností. */}
      <div aria-hidden className="absolute inset-0">
        {slides.map((slide, i) => (
          <img
            key={slide.title.id}
            src={backdropUrl(slide.title)}
            alt=""
            className={cn(
              'absolute inset-0 size-full object-cover object-[70%_25%] transition-opacity duration-1000 motion-reduce:transition-none',
              i === index ? 'opacity-100' : 'opacity-0',
            )}
          />
        ))}
        <div className="absolute inset-0 bg-gradient-to-r from-background via-background/75 to-background/0 md:via-background/55 md:to-transparent" />
        <div className="absolute inset-x-0 bottom-0 h-2/3 bg-gradient-to-t from-background via-background/60 to-transparent" />
      </div>

      <HeroSlide key={current.title.id} featured={current} />

      {count > 1 ? (
        <>
          <div className="absolute bottom-6 left-1/2 flex -translate-x-1/2 gap-2 md:left-auto md:right-8 md:translate-x-0">
            {slides.map((slide, i) => (
              <button
                key={slide.title.id}
                type="button"
                onClick={() => setIndex(i)}
                aria-label={t('video.home.heroSlide', { n: i + 1, total: count })}
                aria-current={i === index}
                className={cn(
                  'h-2 rounded-full outline-none transition-all focus-visible:ring-3 focus-visible:ring-ring/50',
                  i === index ? 'w-6 bg-primary' : 'w-2 bg-foreground/30 hover:bg-foreground/50',
                )}
              />
            ))}
          </div>
          <button
            type="button"
            onClick={() => go(-1)}
            aria-label={t('video.home.heroPrev')}
            className="absolute top-1/2 left-1 hidden size-10 -translate-y-1/2 items-center justify-center rounded-full text-foreground/70 outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 lg:flex"
          >
            <ChevronLeftIcon className="size-7" />
          </button>
          <button
            type="button"
            onClick={() => go(1)}
            aria-label={t('video.home.heroNext')}
            className="absolute top-1/2 right-1 hidden size-10 -translate-y-1/2 items-center justify-center rounded-full text-foreground/70 outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 lg:flex"
          >
            <ChevronRightIcon className="size-7" />
          </button>
        </>
      ) : null}
    </section>
  )
}

function HeroSlide({ featured }: { featured: Featured }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { kind, title } = featured
  // Stav uživatele (seznam, zhlédnuto, další díl) je jen v detailu titulu.
  const movie = useMovie(kind === 'movie' ? title.id : '')
  const show = useShow(kind === 'show' ? title.id : '')
  const detail = kind === 'movie' ? movie.data : show.data
  const watchlist = useSetWatchlist()
  const setWatched = useSetWatched()
  const onError = (error: Error) => toast.error(error.message)

  const to = kind === 'movie' ? `/movies/${title.id}` : `/shows/${title.id}`
  const upNext = kind === 'show' ? show.data?.up_next : undefined
  const playTo = kind === 'movie' ? (title as Movie).media_id && `/watch/movie/${title.id}` : upNext && `/watch/episode/${upNext.id}`
  const watched = kind === 'movie' ? Boolean(movie.data?.user.progress?.completed) : undefined
  const meta = [title.year ? String(title.year) : null, title.genres.slice(0, 2).join(' / ') || null, runtime(title, kind)].filter(Boolean)

  return (
    <div className="relative flex min-h-[26rem] flex-col justify-end px-4 pt-24 pb-16 sm:px-6 md:min-h-[32rem] md:pb-20 lg:px-12 xl:min-h-[36rem]">
      <div className="max-w-2xl animate-in fade-in slide-in-from-bottom-2 duration-500 motion-reduce:animate-none">
        <div className="flex flex-wrap gap-2">
          <span className="rounded-full px-3 py-0.5 text-xs font-semibold text-primary ring-1 ring-primary/70">
            {t('video.home.featured')}
          </span>
          {isUhd(title) ? (
            <span className="rounded-full px-3 py-0.5 text-xs font-semibold text-foreground/90 ring-1 ring-foreground/40">4K</span>
          ) : null}
        </div>
        <h2 className="font-heading mt-4 text-4xl font-bold tracking-tight text-balance drop-shadow-sm sm:text-5xl xl:text-6xl">
          <Link to={to} className="outline-none focus-visible:underline">
            {title.title}
          </Link>
        </h2>
        {meta.length > 0 ? <p className="mt-3 text-sm text-foreground/85 sm:text-base">{meta.join('  ·  ')}</p> : null}
        {title.overview ? <p className="mt-2 line-clamp-2 max-w-xl text-sm text-foreground/80 sm:text-base">{title.overview}</p> : null}

        <div className="mt-6 flex flex-wrap items-center gap-3">
          {playTo ? (
            <Button size="lg" className="h-12 rounded-full px-7 text-base" onClick={() => navigate(playTo)}>
              <PlayIcon className="fill-current" />
              {t('video.detail.play')}
            </Button>
          ) : null}
          <Button size="lg" variant="outline" className="h-12 rounded-full bg-background/30 px-6 text-base" asChild>
            <Link to={to}>
              <InfoIcon />
              {t('video.home.moreInfo')}
            </Link>
          </Button>
          {detail ? (
            <Button
              size="icon-lg"
              variant="outline"
              className="size-12 rounded-full bg-background/30"
              aria-label={detail.user.in_watchlist ? t('video.detail.removeWatchlist') : t('video.detail.addWatchlist')}
              title={detail.user.in_watchlist ? t('video.detail.removeWatchlist') : t('video.detail.addWatchlist')}
              aria-pressed={detail.user.in_watchlist}
              disabled={watchlist.isPending}
              onClick={() => watchlist.mutate({ kind, id: title.id, on: !detail.user.in_watchlist }, { onError })}
            >
              {detail.user.in_watchlist ? <BookmarkCheckIcon className="text-primary" /> : <BookmarkIcon />}
            </Button>
          ) : null}
          {watched !== undefined && movie.data ? (
            <Button
              size="icon-lg"
              variant="outline"
              className={cn('size-12 rounded-full bg-background/30', watched && 'text-primary ring-primary/60')}
              aria-label={watched ? t('video.detail.markUnwatched') : t('video.detail.markWatched')}
              title={watched ? t('video.detail.markUnwatched') : t('video.detail.markWatched')}
              aria-pressed={watched}
              disabled={setWatched.isPending}
              onClick={() => setWatched.mutate({ kind: 'movie', id: title.id, watched: !watched }, { onError })}
            >
              <CheckIcon />
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  )
}
