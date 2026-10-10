import { FilmIcon, TvIcon } from 'lucide-react'
import { useDeferredValue } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams, useSearchParams } from 'react-router'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { useLibraries, useMovies, useShows } from '@/api/hooks'
import { ListToolbar } from '@/components/ListToolbar'
import { SORTS, type Sort } from '@/lib/sort'
import { PosterCard, PosterGrid } from '@/components/PosterCard'

/** Stav filtrů v adrese (?q=&genre=&sort=) – přežije reload i zpět. */
function useListParams() {
  const [params, setParams] = useSearchParams()
  const sortParam = params.get('sort') as Sort | null
  const sort: Sort = sortParam && (SORTS as readonly string[]).includes(sortParam) ? sortParam : 'title'
  const set = (key: string, value: string) =>
    setParams(
      (p) => {
        if (value) p.set(key, value)
        else p.delete(key)
        return p
      },
      { replace: true },
    )
  return { q: params.get('q') ?? '', genre: params.get('genre') ?? '', sort, set }
}

function MovieList({ title, library = '' }: { title: string; library?: string }) {
  const { t } = useTranslation()
  const { q, genre, sort, set } = useListParams()
  const query = useDeferredValue(q)
  const movies = useMovies({ library, q: query, genre, sort })
  const filtered = Boolean(q || genre)

  return (
    <div>
      <PageHeader
        sticky
        title={title}
        description={movies.data ? t('video.movies.count', { count: movies.data.length }) : undefined}
        actions={<ListToolbar query={q} onQuery={(v) => set('q', v)} genre={genre} onGenre={(v) => set('genre', v)} sort={sort} onSort={(v) => set('sort', v === 'title' ? '' : v)} />}
      />
      {movies.isError ? <ErrorState error={movies.error} onRetry={() => void movies.refetch()} /> : null}
      {movies.data && movies.data.length === 0 ? (
        <EmptyState icon={FilmIcon} title={filtered ? t('video.list.noResults') : t('video.movies.empty')} />
      ) : null}
      <PosterGrid>
        {(movies.data ?? []).map((m) => (
          <PosterCard key={m.id} movie={m} />
        ))}
      </PosterGrid>
    </div>
  )
}

function ShowList({ title, library = '' }: { title: string; library?: string }) {
  const { t } = useTranslation()
  const { q, genre, sort, set } = useListParams()
  const query = useDeferredValue(q)
  const shows = useShows({ library, q: query, genre, sort })
  const filtered = Boolean(q || genre)

  return (
    <div>
      <PageHeader
        sticky
        title={title}
        actions={<ListToolbar query={q} onQuery={(v) => set('q', v)} genre={genre} onGenre={(v) => set('genre', v)} sort={sort} onSort={(v) => set('sort', v === 'title' ? '' : v)} />}
      />
      {shows.isError ? <ErrorState error={shows.error} onRetry={() => void shows.refetch()} /> : null}
      {shows.data && shows.data.length === 0 ? (
        <EmptyState icon={TvIcon} title={filtered ? t('video.list.noResults') : t('video.shows.empty')} />
      ) : null}
      <PosterGrid>
        {(shows.data ?? []).map((s) => (
          <PosterCard key={s.id} show={s} />
        ))}
      </PosterGrid>
    </div>
  )
}

/** Všechny filmy napříč knihovnami. */
export function MoviesPage() {
  const { t } = useTranslation()
  return <MovieList title={t('video.movies.title')} />
}

/** Všechny seriály napříč knihovnami. */
export function ShowsPage() {
  const { t } = useTranslation()
  return <ShowList title={t('video.shows.title')} />
}

/** Jedna knihovna (/library/:id) – výpis podle jejího typu. */
export function LibraryPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const libraries = useLibraries()

  if (libraries.isError) return <ErrorState error={libraries.error} onRetry={() => void libraries.refetch()} />
  if (!libraries.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  const lib = libraries.data.find((l) => l.id === id)
  if (!lib) return <EmptyState icon={FilmIcon} title={t('errors.library.not_found')} />

  // key: při přechodu mezi knihovnami se výpis založí znovu (žádná stará data).
  return lib.kind === 'show' ? (
    <ShowList key={lib.id} title={lib.name} library={lib.id} />
  ) : (
    <MovieList key={lib.id} title={lib.name} library={lib.id} />
  )
}
