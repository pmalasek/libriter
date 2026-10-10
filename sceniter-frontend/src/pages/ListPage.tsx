import { FilmIcon, TvIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useParams, useSearchParams } from 'react-router'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { Skeleton } from 'core-web/src/components/ui/skeleton'
import { columnsByWidth, VirtualGrid } from 'core-web/src/components/VirtualGrid'
import { useDebouncedValue, type PagedList } from 'core-web/src/lib/paged'
import { useLibraries, useMovies, useShows } from '@/api/hooks'
import { ListToolbar } from '@/components/ListToolbar'
import { SORTS, type Sort } from '@/lib/sort'
import { PosterCard } from '@/components/PosterCard'
import { POSTER_GRID, posterCardHeight } from '@/lib/posterGrid'

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
  // Hledání se pošle až po chvíli klidu, ne po každé klávese.
  const query = useDebouncedValue(q.trim())
  const movies = useMovies({ library, q: query, genre, sort })
  const filtered = Boolean(q || genre)

  return (
    <div>
      <PageHeader
        sticky
        title={title}
        description={movies.total !== undefined ? t('video.movies.count', { count: movies.total }) : undefined}
        actions={<ListToolbar query={q} onQuery={(v) => set('q', v)} genre={genre} onGenre={(v) => set('genre', v)} sort={sort} onSort={(v) => set('sort', v === 'title' ? '' : v)} />}
      />
      {movies.isError && movies.total === undefined ? <ErrorState error={movies.error} onRetry={movies.refetch} /> : null}
      {movies.total === 0 ? (
        <EmptyState icon={FilmIcon} title={filtered ? t('video.list.noResults') : t('video.movies.empty')} />
      ) : null}
      <TitleGrid list={movies} render={(m) => <PosterCard movie={m} />} />
    </div>
  )
}

function ShowList({ title, library = '' }: { title: string; library?: string }) {
  const { t } = useTranslation()
  const { q, genre, sort, set } = useListParams()
  const query = useDebouncedValue(q.trim())
  const shows = useShows({ library, q: query, genre, sort })
  const filtered = Boolean(q || genre)

  return (
    <div>
      <PageHeader
        sticky
        title={title}
        actions={<ListToolbar query={q} onQuery={(v) => set('q', v)} genre={genre} onGenre={(v) => set('genre', v)} sort={sort} onSort={(v) => set('sort', v === 'title' ? '' : v)} />}
      />
      {shows.isError && shows.total === undefined ? <ErrorState error={shows.error} onRetry={shows.refetch} /> : null}
      {shows.total === 0 ? (
        <EmptyState icon={TvIcon} title={filtered ? t('video.list.noResults') : t('video.shows.empty')} />
      ) : null}
      <TitleGrid list={shows} render={(s) => <PosterCard show={s} />} />
    </div>
  )
}

/**
 * Virtualizovaná mřížka plakátů: v DOM jsou jen řádky v okně a stránky
 * výpisu se dotahují podle toho, kam uživatel doroluje.
 */
function TitleGrid<T>({ list, render }: { list: PagedList<T>; render: (item: T) => React.ReactNode }) {
  return (
    <VirtualGrid
      count={list.total ?? 0}
      columns={columnsByWidth(POSTER_GRID.minWidth, POSTER_GRID.gap, POSTER_GRID.minColumns)}
      gap={POSTER_GRID.gap}
      rowGap={POSTER_GRID.rowGap}
      estimateRowHeight={posterCardHeight}
      onRangeChange={list.setRange}
      renderItem={(index) => {
        const item = list.getItem(index)
        return item ? render(item) : <PosterSkeleton />
      }}
    />
  )
}

function PosterSkeleton() {
  return (
    <div>
      <Skeleton className="aspect-[2/3] w-full rounded-2xl" />
      <Skeleton className="mt-2 h-4 w-4/5" />
      <Skeleton className="mt-1 h-3 w-1/3" />
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
