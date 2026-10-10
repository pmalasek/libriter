import { FilmIcon, TvIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useParams, useSearchParams } from 'react-router'
import { SORT_DIRS, VIEW_MODES, type SortDir, type ViewMode } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { Skeleton } from 'core-web/src/components/ui/skeleton'
import { columnsByWidth, VirtualGrid } from 'core-web/src/components/VirtualGrid'
import { useDebouncedValue, type PagedList } from 'core-web/src/lib/paged'
import { usePersistedChoice } from 'core-web/src/lib/prefs'
import { useLibraries, useMovies, useShows } from '@/api/hooks'
import { ListToolbar } from '@/components/ListToolbar'
import { PosterCard } from '@/components/PosterCard'
import { TitleRow, TitleRowHeader } from '@/components/TitleRow'
import { POSTER_LAYOUT, posterCardHeight } from '@/lib/posterGrid'
import { SORT_DEFAULT_DIR, SORTS, type Sort } from '@/lib/sort'

/**
 * Stav filtrů v adrese (?q=&genre=&sort=&dir=) – přežije reload i zpět.
 * Výchozí hodnoty (název, přirozený směr klíče) se do adresy nepíšou.
 */
function useListParams() {
  const [params, setParams] = useSearchParams()
  const sortParam = params.get('sort') as Sort | null
  const sort: Sort = sortParam && (SORTS as readonly string[]).includes(sortParam) ? sortParam : 'title'
  const dirParam = params.get('dir') as SortDir | null
  const dir: SortDir = dirParam && (SORT_DIRS as readonly string[]).includes(dirParam) ? dirParam : SORT_DEFAULT_DIR[sort]
  const update = (values: Record<string, string>) =>
    setParams(
      (p) => {
        for (const [key, value] of Object.entries(values)) {
          if (value) p.set(key, value)
          else p.delete(key)
        }
        return p
      },
      { replace: true },
    )
  return {
    q: params.get('q') ?? '',
    genre: params.get('genre') ?? '',
    sort,
    dir,
    setQuery: (q: string) => update({ q }),
    setGenre: (genre: string) => update({ genre }),
    // Změna klíče vrátí jeho přirozený směr (nejnovější první u data přidání).
    setSort: (key: Sort) => update({ sort: key === 'title' ? '' : key, dir: '' }),
    setDir: (d: SortDir) => update({ dir: d === SORT_DEFAULT_DIR[sort] ? '' : d }),
  }
}

/** Panel nástrojů výpisu – filtry z adresy, zobrazení z uložené předvolby. */
function Toolbar({ list, view, onView }: { list: ReturnType<typeof useListParams>; view: ViewMode; onView: (v: ViewMode) => void }) {
  return (
    <ListToolbar
      query={list.q}
      onQuery={list.setQuery}
      genre={list.genre}
      onGenre={list.setGenre}
      sort={list.sort}
      onSort={list.setSort}
      dir={list.dir}
      onDir={list.setDir}
      view={view}
      onView={onView}
    />
  )
}

function MovieList({ title, library = '' }: { title: string; library?: string }) {
  const { t } = useTranslation()
  const list = useListParams()
  const [view, setView] = usePersistedChoice<ViewMode>('movies.view', 'tiles', VIEW_MODES)
  // Hledání se pošle až po chvíli klidu, ne po každé klávese.
  const query = useDebouncedValue(list.q.trim())
  const movies = useMovies({ library, q: query, genre: list.genre, sort: list.sort, dir: list.dir })
  const filtered = Boolean(list.q || list.genre)

  return (
    <div>
      <PageHeader
        sticky
        title={title}
        description={movies.total !== undefined ? t('video.movies.count', { count: movies.total }) : undefined}
        actions={<Toolbar list={list} view={view} onView={setView} />}
      />
      {movies.isError && movies.total === undefined ? <ErrorState error={movies.error} onRetry={movies.refetch} /> : null}
      {movies.total === 0 ? (
        <EmptyState icon={FilmIcon} title={filtered ? t('video.list.noResults') : t('video.movies.empty')} />
      ) : null}
      <TitleGrid
        list={movies}
        view={view}
        kind="movie"
        renderCard={(m, size) => <PosterCard movie={m} size={size} />}
        renderRow={(m) => <TitleRow movie={m} />}
      />
    </div>
  )
}

function ShowList({ title, library = '' }: { title: string; library?: string }) {
  const { t } = useTranslation()
  const list = useListParams()
  const [view, setView] = usePersistedChoice<ViewMode>('shows.view', 'tiles', VIEW_MODES)
  const query = useDebouncedValue(list.q.trim())
  const shows = useShows({ library, q: query, genre: list.genre, sort: list.sort, dir: list.dir })
  const filtered = Boolean(list.q || list.genre)

  return (
    <div>
      <PageHeader
        sticky
        title={title}
        actions={<Toolbar list={list} view={view} onView={setView} />}
      />
      {shows.isError && shows.total === undefined ? <ErrorState error={shows.error} onRetry={shows.refetch} /> : null}
      {shows.total === 0 ? (
        <EmptyState icon={TvIcon} title={filtered ? t('video.list.noResults') : t('video.shows.empty')} />
      ) : null}
      <TitleGrid
        list={shows}
        view={view}
        kind="show"
        renderCard={(s, size) => <PosterCard show={s} size={size} />}
        renderRow={(s) => <TitleRow show={s} />}
      />
    </div>
  )
}

/**
 * Virtualizovaná mřížka plakátů nebo seznam: v DOM jsou jen řádky v okně
 * a stránky výpisu se dotahují podle toho, kam uživatel doroluje.
 */
function TitleGrid<T>({
  list,
  view,
  kind,
  renderCard,
  renderRow,
}: {
  list: PagedList<T>
  view: ViewMode
  kind: 'movie' | 'show'
  renderCard: (item: T, size: 'tiles' | 'small') => React.ReactNode
  renderRow: (item: T) => React.ReactNode
}) {
  if (list.total === 0) return null
  if (view === 'list') {
    return (
      <div className="glass inset-shadow-glass overflow-hidden rounded-2xl shadow-glass ring-1 ring-glass-edge">
        <TitleRowHeader kind={kind} />
        <VirtualGrid
          count={list.total ?? 0}
          columns={() => 1}
          gap={0}
          estimateRowHeight={() => 65}
          onRangeChange={list.setRange}
          rowClassName="border-t border-foreground/6"
          renderItem={(index) => {
            const item = list.getItem(index)
            return item ? renderRow(item) : <Skeleton className="mx-3 my-2 h-12 rounded-lg" />
          }}
        />
      </div>
    )
  }
  const layout = POSTER_LAYOUT[view]
  return (
    <VirtualGrid
      // Při změně velikosti dlaždic se rozvržení řádků počítá znovu.
      key={view}
      count={list.total ?? 0}
      columns={columnsByWidth(layout.minWidth, layout.gap, layout.minColumns)}
      gap={layout.gap}
      rowGap={layout.rowGap}
      estimateRowHeight={(width) => posterCardHeight(width, view)}
      onRangeChange={list.setRange}
      renderItem={(index) => {
        const item = list.getItem(index)
        return item ? renderCard(item, view) : <PosterSkeleton />
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
