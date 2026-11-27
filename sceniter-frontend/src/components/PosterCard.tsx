import { CheckCircle2Icon } from 'lucide-react'
import { cn } from 'core-web/src/lib/utils'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { isNewTitle, posterUrl, progressRatio, type MovieSummary, type ShowSummary, type WatchProgress } from 'sceniter-shared'
import { Poster } from './Poster'
import { ProgressBar } from './ProgressBar'
import type { PosterSize } from '@/lib/posterGrid'

/** Největší šířka plakátu v mřížce (CSS px) – načte se zmenšenina. */
const POSTER_WIDTH = 240

/** Dlaždice filmu nebo seriálu: plakát, název, rok, případně rozkoukanost. */
export function PosterCard({
  movie,
  show,
  progress,
  caption,
  size = 'tiles',
  overlay = false,
}: {
  movie?: MovieSummary
  show?: ShowSummary
  progress?: WatchProgress
  /** Popisek pod názvem místo roku / počtu dílů (např. role ve filmografii). */
  caption?: string
  /** Malé dlaždice mají drobnější písmo (viz POSTER_LAYOUT). */
  size?: PosterSize
  /**
   * Název a rok přes spodek plakátu se štítkem „Nové“ – pro police na
   * domovské stránce. Mřížky ho nepoužívají: virtualizace počítá s textem
   * pod plakátem (POSTER_LAYOUT).
   */
  overlay?: boolean
}) {
  const { t } = useTranslation()
  const title = movie ?? show
  if (!title) return null
  const to = movie ? `/movies/${movie.id}` : `/shows/${show!.id}`
  const ratio = progressRatio(progress, movie?.duration_sec)
  const subtitle = show
    ? t('video.shows.episodes', { count: show.episode_count })
    : title.year
      ? String(title.year)
      : ''

  if (overlay) {
    return (
      <Link to={to} className="group block outline-none" aria-label={subtitle ? `${title.title}, ${subtitle}` : title.title}>
        <div className="relative overflow-hidden rounded-2xl ring-1 ring-glass-edge transition-[transform,box-shadow] duration-200 group-hover:-translate-y-0.5 group-hover:ring-2 group-hover:ring-primary group-focus-visible:ring-2 group-focus-visible:ring-primary">
          <Poster src={posterUrl(title, POSTER_WIDTH)} title={title.title} kind={movie ? 'movie' : 'show'} year={title.year} className="rounded-none shadow-none ring-0" />
          <div aria-hidden className="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/90 via-black/50 to-transparent" />
          {isNewTitle(title) ? (
            <span className="absolute top-2 right-2 rounded-full bg-metal px-2 py-0.5 text-[11px] font-semibold text-metal-foreground shadow-sm">
              {t('video.home.badgeNew')}
            </span>
          ) : null}
          {progress?.completed ? (
            <span
              role="img"
              aria-label={t('video.detail.watched')}
              title={t('video.detail.watched')}
              className="absolute top-2 left-2 flex size-6 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-sm"
            >
              <CheckCircle2Icon className="size-4" />
            </span>
          ) : null}
          <div className="absolute inset-x-2.5 bottom-2.5 text-white">
            <p className="font-heading line-clamp-2 text-sm leading-tight font-semibold drop-shadow sm:text-base">{title.title}</p>
            {subtitle ? <p className="mt-0.5 text-xs text-white/75">{subtitle}</p> : null}
            {ratio > 0 && ratio < 1 ? <ProgressBar value={ratio} className="mt-1.5 bg-white/25" /> : null}
          </div>
        </div>
      </Link>
    )
  }

  return (
    <Link to={to} className="group block outline-none">
      <div className="relative transition-transform duration-200 group-hover:-translate-y-0.5 group-focus-visible:ring-3 group-focus-visible:ring-ring/50 rounded-2xl">
        <Poster src={posterUrl(title, POSTER_WIDTH)} title={title.title} kind={movie ? 'movie' : 'show'} year={title.year} />
        {progress?.completed ? (
          <span
            role="img"
            aria-label={t('video.detail.watched')}
            title={t('video.detail.watched')}
            className="absolute top-2 right-2 flex size-6 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-sm"
          >
            <CheckCircle2Icon className="size-4" />
          </span>
        ) : null}
        {ratio > 0 && ratio < 1 ? <ProgressBar value={ratio} className="absolute inset-x-2 bottom-2" /> : null}
      </div>
      <p className={cn('line-clamp-2 leading-snug font-medium', size === 'small' ? 'mt-1.5 text-xs' : 'mt-2 text-sm')}>{title.title}</p>
      {subtitle ? <p className={cn('text-muted-foreground', size === 'small' ? 'text-[11px]' : 'text-xs')}>{subtitle}</p> : null}
      {caption ? <p className="line-clamp-2 text-xs text-muted-foreground">{caption}</p> : null}
    </Link>
  )
}

/** Mřížka dlaždic – počet sloupců podle šířky obsahu. */
export function PosterGrid({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-6 @sm:grid-cols-3 @2xl:grid-cols-4 @4xl:grid-cols-5 @6xl:grid-cols-6 @7xl:grid-cols-7">
      {children}
    </div>
  )
}
