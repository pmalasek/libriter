import { PlayIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import {
  episodeLabel,
  imageUrl,
  posterUrl,
  progressRatio,
  remainingMinutesLabel,
  type ContinueItem,
} from 'sceniter-shared'
import { Poster } from './Poster'
import { ProgressBar } from './ProgressBar'

/** Největší šířka obrázku dílu na polici (CSS px) – načte se zmenšenina. */
const STILL_WIDTH = 640

/**
 * Karta „Pokračovat ve sledování“: záběr dílu nebo pozadí filmu, přes jeho
 * spodek název, díl, průběh a zbývající čas. Najetí nebo fokus ji orámuje
 * kovem a ukáže tlačítko přehrát.
 */
export function ContinueCard({ item }: { item: ContinueItem }) {
  const { t } = useTranslation()
  const movie = item.movie
  const episode = item.episode
  const to = movie ? `/watch/movie/${movie.id}` : `/watch/episode/${episode!.id}`
  const still = movie
    ? (imageUrl(movie.backdrop_path, movie.updated_at, STILL_WIDTH) ?? posterUrl(movie, STILL_WIDTH))
    : (imageUrl(episode!.still_path, undefined, STILL_WIDTH) ??
      imageUrl(episode!.show_backdrop_path, undefined, STILL_WIDTH) ??
      (item.show ? imageUrl(item.show.backdrop_path, item.show.updated_at, STILL_WIDTH) : undefined))
  const title = movie ? movie.title : (item.show?.title ?? episode!.show_title ?? '')
  const sub = movie ? (movie.year ? String(movie.year) : '') : episodeLabel(episode!)
  const duration = movie?.duration_sec ?? episode?.duration_sec
  const ratio = progressRatio(item.progress, duration)
  const remaining = remainingMinutesLabel(item.progress, duration)

  return (
    <Link to={to} className="group block outline-none" aria-label={`${title}, ${sub}`}>
      <div className="relative overflow-hidden rounded-2xl ring-1 ring-glass-edge transition-[transform,box-shadow] duration-200 group-hover:-translate-y-0.5 group-hover:ring-2 group-hover:ring-primary group-focus-visible:ring-2 group-focus-visible:ring-primary">
        <Poster src={still} title={title} aspect="still" className="rounded-none shadow-none ring-0" bare />
        <div aria-hidden className="absolute inset-0 bg-gradient-to-t from-black/90 via-black/35 to-transparent" />

        {item.next ? (
          <span className="absolute top-2.5 left-2.5 rounded-full bg-metal px-2 py-0.5 text-xs font-semibold text-metal-foreground shadow-sm">
            {t('video.home.nextEpisode')}
          </span>
        ) : null}
        <span
          aria-hidden
          className="absolute top-2.5 right-2.5 flex size-9 items-center justify-center rounded-xl bg-metal text-metal-foreground opacity-0 shadow-glass transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
        >
          <PlayIcon className="size-4 fill-current" />
        </span>

        <div className="absolute inset-x-3 bottom-2.5 text-white">
          <p className="font-heading truncate text-base leading-tight font-semibold drop-shadow">{title}</p>
          {sub ? <p className="mt-0.5 truncate text-xs text-white/80">{sub}</p> : null}
          <div className="mt-2 flex items-center gap-3">
            <ProgressBar value={ratio} className="flex-1 bg-white/25" />
            {remaining ? <span className="shrink-0 text-[11px] text-white/85 tabular-nums">{remaining}</span> : null}
          </div>
        </div>
      </div>
    </Link>
  )
}
