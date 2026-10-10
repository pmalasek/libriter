import { FilmIcon, FolderPlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { episodeLabel, imageUrl, posterUrl, progressRatio, remainingLabel, type ContinueItem, type HomeLibrary } from 'sceniter-shared'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { isAdmin } from 'core-web/src/auth/permissions'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { useHome, useLibraries } from '@/api/hooks'
import { Attribution } from '@/components/Attribution'
import { Poster } from '@/components/Poster'
import { PosterCard } from '@/components/PosterCard'
import { ProgressBar } from '@/components/ProgressBar'
import { Shelf, ShelfItem } from '@/components/Shelf'

/** Karta „Pokračovat“: obrázek dílu nebo pozadí filmu, název a zbývající čas. */
function ContinueCard({ item }: { item: ContinueItem }) {
  const { t } = useTranslation()
  const movie = item.movie
  const episode = item.episode
  const to = movie ? `/watch/movie/${movie.id}` : `/watch/episode/${episode!.id}`
  const still = movie
    ? imageUrl(movie.backdrop_path, movie.updated_at) ?? posterUrl(movie)
    : imageUrl(episode!.still_path) ?? (item.show ? imageUrl(item.show.backdrop_path, item.show.updated_at) : undefined)
  const title = movie ? movie.title : (item.show?.title ?? episode!.show_title ?? '')
  const sub = movie ? remainingLabel(item.progress, movie.duration_sec) : episodeLabel(episode!)
  const ratio = progressRatio(item.progress, movie?.duration_sec ?? episode?.duration_sec)

  return (
    <Link to={to} className="group block outline-none">
      <div className="relative rounded-2xl transition-transform duration-200 group-hover:-translate-y-0.5 group-focus-visible:ring-3 group-focus-visible:ring-ring/50">
        <Poster src={still} title={title} aspect="still" />
        {item.next ? (
          <span className="absolute top-2 left-2 rounded-full bg-primary px-2 py-0.5 text-xs font-medium text-primary-foreground shadow-sm">
            {t('video.home.nextEpisode')}
          </span>
        ) : null}
        <ProgressBar value={ratio} className="absolute inset-x-3 bottom-3" />
      </div>
      <p className="mt-2 truncate text-sm font-medium">{title}</p>
      <p className="truncate text-xs text-muted-foreground">{sub}</p>
    </Link>
  )
}

/** Nově přidané v jedné knihovně: plakáty filmů, nebo obrázky nových dílů. */
function LibraryShelf({ row }: { row: HomeLibrary }) {
  const { t } = useTranslation()
  const lib = row.library
  return (
    <Shelf title={t('video.home.inLibrary', { name: lib.name })} to={`/library/${lib.id}?sort=added`}>
      {row.movies.map((m) => (
        <ShelfItem key={m.id}>
          <PosterCard movie={m} />
        </ShelfItem>
      ))}
      {row.episodes.map((e) => (
        <ShelfItem key={e.id} wide>
          <Link to={`/shows/${e.show_id}`} className="group block outline-none">
            <Poster src={imageUrl(e.still_path)} title={e.show_title ?? ''} aspect="still" className="transition-transform duration-200 group-hover:-translate-y-0.5" />
            <p className="mt-2 truncate text-sm font-medium">{e.show_title}</p>
            <p className="truncate text-xs text-muted-foreground">{episodeLabel(e)}</p>
          </Link>
        </ShelfItem>
      ))}
    </Shelf>
  )
}

export function HomePage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const home = useHome()
  const libraries = useLibraries()

  if (home.isError) return <ErrorState error={home.error} onRetry={() => void home.refetch()} />
  if (!home.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>

  const { continue: cont, libraries: rows, watchlist } = home.data
  const noLibraries = libraries.data?.length === 0
  const empty = !noLibraries && rows.length === 0

  return (
    <div>
      <h1 className="font-heading sr-only">{t('video.nav.home')}</h1>
      {noLibraries ? (
        <div className="mt-6">
          <EmptyState
            icon={FolderPlusIcon}
            title={t('video.home.noLibrariesTitle')}
            description={t('video.home.noLibrariesDescription')}
          />
          {isAdmin(user) ? (
            <div className="mt-4 flex justify-center">
              <Button asChild>
                <Link to="/admin/libraries">{t('video.home.noLibrariesAction')}</Link>
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}
      {empty ? (
        <div className="mt-6">
          <EmptyState icon={FilmIcon} title={t('video.home.emptyTitle')} description={t('video.home.emptyDescription')} />
        </div>
      ) : null}

      {cont.length > 0 ? (
        <Shelf title={t('video.home.continue')} className="mt-4">
          {cont.map((item) => (
            <ShelfItem key={(item.movie?.id ?? item.episode?.id) + String(item.next)} wide>
              <ContinueCard item={item} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {rows.map((row) => (
        <LibraryShelf key={row.library.id} row={row} />
      ))}

      {watchlist.length > 0 ? (
        <Shelf title={t('video.home.watchlist')} to="/watchlist">
          {watchlist.map((w) => (
            <ShelfItem key={(w.movie?.id ?? w.show?.id) as string}>
              <PosterCard movie={w.movie} show={w.show} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      <Attribution />
    </div>
  )
}
