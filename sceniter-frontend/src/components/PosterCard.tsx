import { CheckCircle2Icon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { posterUrl, progressRatio, type MovieSummary, type ShowSummary, type WatchProgress } from 'sceniter-shared'
import { Poster } from './Poster'
import { ProgressBar } from './ProgressBar'

/** Největší šířka plakátu v mřížce (CSS px) – načte se zmenšenina. */
const POSTER_WIDTH = 240

/** Dlaždice filmu nebo seriálu: plakát, název, rok, případně rozkoukanost. */
export function PosterCard({
  movie,
  show,
  progress,
  caption,
}: {
  movie?: MovieSummary
  show?: ShowSummary
  progress?: WatchProgress
  /** Popisek pod názvem místo roku / počtu dílů (např. role ve filmografii). */
  caption?: string
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

  return (
    <Link to={to} className="group block outline-none">
      <div className="relative transition-transform duration-200 group-hover:-translate-y-0.5 group-focus-visible:ring-3 group-focus-visible:ring-ring/50 rounded-2xl">
        <Poster src={posterUrl(title, POSTER_WIDTH)} title={title.title} />
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
      <p className="mt-2 line-clamp-2 text-sm leading-snug font-medium">{title.title}</p>
      {subtitle ? <p className="text-xs text-muted-foreground">{subtitle}</p> : null}
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
