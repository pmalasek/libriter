import { FilmIcon, FolderPlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import type { HomeLibrary } from 'sceniter-shared'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { isAdmin } from 'core-web/src/auth/permissions'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { useHome, useLibraries } from '@/api/hooks'
import { Attribution } from '@/components/Attribution'
import { ContinueCard } from '@/components/ContinueCard'
import { HomeHero } from '@/components/HomeHero'
import { PosterCard } from '@/components/PosterCard'
import { Shelf, ShelfItem } from '@/components/Shelf'
import { libraryIcon } from '@/lib/libraryIcons'

/**
 * Nově přidané v jedné knihovně: plakáty filmů, nebo seriálů s nejnovějšími
 * díly. Název knihovny je nadpisem a ikona je ta z navigace, aby se police
 * knihoven daly rozeznat jedna od druhé i od „Pokračovat“ a „Chci vidět“.
 */
function LibraryShelf({ row, divider }: { row: HomeLibrary; divider: boolean }) {
  const { t } = useTranslation()
  const lib = row.library
  return (
    <Shelf
      eyebrow={t('video.home.newIn')}
      title={lib.name}
      icon={libraryIcon(lib)}
      to={`/library/${lib.id}?sort=added`}
      divider={divider}
    >
      {row.movies.map((m) => (
        <ShelfItem key={m.id}>
          <PosterCard movie={m} overlay />
        </ShelfItem>
      ))}
      {row.shows.map((s) => (
        <ShelfItem key={s.id}>
          <PosterCard show={s} overlay />
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
      <HomeHero rows={rows} />
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
        <Shelf title={t('video.home.continue')} divider={false} className="mt-2">
          {cont.map((item) => (
            <ShelfItem key={(item.movie?.id ?? item.episode?.id) + String(item.next)} wide>
              <ContinueCard item={item} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      {rows.map((row, i) => (
        <LibraryShelf key={row.library.id} row={row} divider={cont.length > 0 || i > 0} />
      ))}

      {watchlist.length > 0 ? (
        <Shelf title={t('video.home.watchlist')} to="/watchlist">
          {watchlist.map((w) => (
            <ShelfItem key={(w.movie?.id ?? w.show?.id) as string}>
              <PosterCard movie={w.movie} show={w.show} overlay />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}

      <Attribution />
    </div>
  )
}
