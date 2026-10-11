import { StarIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { formatClock, formatDate, posterUrl, type MovieSummary, type ShowSummary } from 'sceniter-shared'
import { Poster } from './Poster'

/** Šířka miniatury plakátu v řádku (CSS px) – načte se zmenšenina. */
const THUMB_WIDTH = 80

/** Záhlaví seznamového zobrazení – sloupce odpovídají TitleRow. */
export function TitleRowHeader({ kind }: { kind: 'movie' | 'show' }) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-3 bg-foreground/4 px-3 py-2 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
      <span className="w-8 shrink-0" />
      <span className="min-w-0 flex-1">{t('video.list.columns.title')}</span>
      <span className="w-12 shrink-0 text-right">{t('video.list.columns.year')}</span>
      <span className="hidden w-16 shrink-0 text-right sm:block">
        {kind === 'show' ? t('video.list.columns.episodes') : t('video.list.columns.length')}
      </span>
      <span className="hidden w-14 shrink-0 text-right sm:block">{t('video.list.columns.rating')}</span>
      <span className="hidden w-28 shrink-0 text-right md:block">{t('video.list.columns.added')}</span>
    </div>
  )
}

/** Řádek filmu nebo seriálu v seznamovém zobrazení. */
export function TitleRow({ movie, show }: { movie?: MovieSummary; show?: ShowSummary }) {
  const title = movie ?? show
  if (!title) return null
  const to = movie ? `/movies/${movie.id}` : `/shows/${show!.id}`
  const original = title.original_title && title.original_title !== title.title ? title.original_title : ''

  return (
    <Link
      to={to}
      className="flex items-center gap-3 px-3 py-2 outline-none transition-colors hover:bg-primary/5 focus-visible:bg-primary/10"
    >
      <Poster src={posterUrl(title, THUMB_WIDTH)} title={title.title} kind={movie ? 'movie' : 'show'} compact className="w-8 shrink-0 rounded-md shadow-none" />
      <div className="min-w-0 flex-1">
        <p className="font-heading truncate text-sm font-semibold tracking-tight">{title.title}</p>
        {original ? <p className="truncate text-xs text-muted-foreground">{original}</p> : null}
      </div>
      {/* Rok se ukazuje v každé šířce; délka a hodnocení od sm, datum přidání od md. */}
      <p className="w-12 shrink-0 text-right text-xs tabular-nums text-muted-foreground">{title.year ?? ''}</p>
      <p className="hidden w-16 shrink-0 text-right text-xs tabular-nums text-muted-foreground sm:block">
        {show ? show.episode_count : movie?.duration_sec ? formatClock(movie.duration_sec) : ''}
      </p>
      <p className="hidden w-14 shrink-0 items-center justify-end gap-1 text-xs tabular-nums text-muted-foreground sm:flex">
        {title.rating ? (
          <>
            <StarIcon className="size-3 fill-current text-primary" />
            {title.rating} %
          </>
        ) : null}
      </p>
      <p className="hidden w-28 shrink-0 text-right text-xs text-muted-foreground md:block">{formatDate(title.created_at)}</p>
    </Link>
  )
}
