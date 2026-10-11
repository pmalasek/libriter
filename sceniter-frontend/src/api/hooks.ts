import { useMutation, useQuery, useQueryClient, type Query } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { usePagedList, type PagedList } from 'core-web/src/lib/paged'
import {
  apiFetch,
  apiUpload,
  getEpisode,
  getGenres,
  getHome,
  getLibraries,
  getMovie,
  getPerson,
  getPlayback,
  getSeason,
  getShow,
  getSimilar,
  getTitlePage,
  getWatchlist,
  list,
  listQuery,
  queryKeys,
  search,
  type Page,
  type Library,
  type LibraryRequest,
  type MatchStatus,
  type MetadataProvider,
  type MetadataSearchResponse,
  type MovieSummary,
  type PlaybackMode,
  type RefreshStatus,
  type SceniterConfig,
  type SceniterStats,
  type ShowIssue,
  type ShowSummary,
  type TitleImageKind,
  type TitleKind,
  type TitlePatch,
  type TranscodeSession,
  type MovieIssue,
  type RenumberPlan,
  type ShowFolder,
  type TrashLibrary,
} from 'sceniter-shared'

const BASE = '/sceniter'

// Prohlížení a přehrávání volá funkce ze sceniter-shared (sdílí je TV klient);
// tady jsou nad nimi jen hooky.
export { saveProgress } from 'sceniter-shared'

function params(p: Record<string, string | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(p)) if (v) q.set(k, v)
  const s = q.toString()
  return s ? `?${s}` : ''
}

export function useSceniterConfig() {
  return useQuery({
    queryKey: queryKeys.config,
    queryFn: () => apiFetch<SceniterConfig>(`${BASE}/config`),
    staleTime: 5 * 60 * 1000,
  })
}

export function useHome() {
  return useQuery({ queryKey: queryKeys.home, queryFn: ({ signal }) => getHome({ signal }) })
}

/** Zapnuté knihovny v nastaveném pořadí – menu a domovská stránka. */
export function useLibraries() {
  return useQuery({
    queryKey: queryKeys.libraries,
    queryFn: ({ signal }) => getLibraries({ signal }),
    staleTime: 60_000,
  })
}

export interface ListParams extends Record<string, string> {
  library: string
  q: string
  genre: string
  sort: string
  dir: string
}

function pageFetcher<T>(path: string, p: Record<string, string | boolean | undefined>) {
  return (offset: number, limit: number, signal: AbortSignal) =>
    apiFetch<Page<T>>(`${BASE}${path}?${listQuery({ ...p, limit, offset })}`, { signal })
}

/**
 * Filmy podle filtru – stránkovaně. Knihovna může mít desítky tisíc titulů;
 * v paměti se drží jen stránky kolem viditelné části mřížky.
 */
export function useMovies(p: ListParams): PagedList<MovieSummary> {
  return usePagedList({
    queryKey: queryKeys.movies(p),
    fetchPage: (offset, limit, signal) => getTitlePage<MovieSummary>('movie', p, offset, limit, { signal }),
  })
}

/** `fresh`: při připojení vždy načíst znovu (pozice pro přehrávač nesmí být z cache). */
export function useMovie(id: string, opts: { fresh?: boolean } = {}) {
  return useQuery({
    queryKey: queryKeys.movie(id),
    queryFn: ({ signal }) => getMovie(id, { signal }),
    enabled: Boolean(id),
    refetchOnMount: opts.fresh ? 'always' : true,
  })
}

/** Seriály podle filtru – stránkovaně. */
export function useShows(p: ListParams): PagedList<ShowSummary> {
  return usePagedList({
    queryKey: queryKeys.shows(p),
    fetchPage: (offset, limit, signal) => getTitlePage<ShowSummary>('show', p, offset, limit, { signal }),
  })
}

export function useShow(id: string) {
  return useQuery({
    queryKey: queryKeys.show(id),
    queryFn: ({ signal }) => getShow(id, { signal }),
    enabled: Boolean(id),
  })
}

/** Díly jedné řady s rozkoukaností – detail seriálu je načítá po řadách. */
export function useSeason(showId: string, number: number | undefined) {
  return useQuery({
    queryKey: queryKeys.season(showId, number ?? -1),
    queryFn: ({ signal }) => getSeason(showId, number!, { signal }),
    enabled: Boolean(showId) && number !== undefined,
  })
}

export function useEpisode(id: string | undefined, opts: { fresh?: boolean } = {}) {
  return useQuery({
    queryKey: queryKeys.episode(id ?? ''),
    queryFn: ({ signal }) => getEpisode(id!, { signal }),
    enabled: Boolean(id),
    refetchOnMount: opts.fresh ? 'always' : true,
  })
}

/** Osoba a její filmy a seriály v knihovně. */
export function usePerson(id: string) {
  return useQuery({
    queryKey: queryKeys.person(id),
    queryFn: ({ signal }) => getPerson(id, { signal }),
    enabled: Boolean(id),
  })
}

/** Podobné tituly (stejná knihovna, společné žánry). */
export function useSimilar(kind: TitleKind, id: string) {
  return useQuery({
    queryKey: queryKeys.similar(kind, id),
    queryFn: ({ signal }) => getSimilar(kind, id, { signal }),
    enabled: Boolean(id),
    staleTime: 5 * 60 * 1000,
  })
}

export function useGenres() {
  return useQuery({
    queryKey: queryKeys.genres,
    queryFn: ({ signal }) => getGenres({ signal }),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: queryKeys.search(q),
    queryFn: ({ signal }) => search(q, { signal }),
    enabled: q.trim().length > 0,
    placeholderData: (prev) => prev,
  })
}

export function useWatchlist() {
  return useQuery({
    queryKey: queryKeys.watchlist,
    queryFn: ({ signal }) => getWatchlist({ signal }),
  })
}

/**
 * Rozhodnutí o přehrávání. Nekešuje se: adresy nesou token s omezenou
 * platností a server při každém dotazu ověří stav souboru.
 */
export function usePlayback(mediaId: string | undefined, profile: string, audio?: number, mode?: string) {
  return useQuery({
    queryKey: [...queryKeys.playback(mediaId ?? '', audio), profile, mode ?? null],
    queryFn: () => getPlayback(mediaId!, { profile, audio, mode: mode as PlaybackMode | undefined }),
    enabled: Boolean(mediaId),
    staleTime: Infinity,
    gcTime: 0,
    retry: false,
  })
}

/**
 * Části cache, které stav uživatele (seznam, hodnocení, zhlédnutí) nemění:
 * výpisy titulů nesou jen metadata, ne rozkoukanost. Velké stránkované
 * výpisy se tak po označení dílu jako zhlédnutého znovu nestahují.
 */
const USER_STATE_INDEPENDENT = new Set(['movies', 'shows', 'genres', 'libraries', 'config', 'admin', 'metadata', 'playback'])

function dependsOnUserState(query: Query): boolean {
  const key = query.queryKey
  return key[0] === 'sceniter' && !USER_STATE_INDEPENDENT.has(String(key[1]))
}

/** Po změně stavu uživatele se obnoví detaily, domovská stránka a seznam ke zhlédnutí. */
function useInvalidateUserState() {
  const qc = useQueryClient()
  return () => {
    void qc.invalidateQueries({ predicate: dependsOnUserState })
  }
}

/** Po změně titulu (metadata, párování, knihovny) zestárne všechno, i výpisy. */
function useInvalidateAll() {
  const qc = useQueryClient()
  return () => {
    void qc.invalidateQueries({ queryKey: ['sceniter'] })
  }
}

export function useSetWatchlist() {
  const invalidate = useInvalidateUserState()
  return useMutation({
    mutationFn: ({ kind, id, on }: { kind: TitleKind; id: string; on: boolean }) =>
      apiFetch<void>(`${BASE}/watchlist/${kind}/${id}`, { method: on ? 'PUT' : 'DELETE' }),
    onSuccess: invalidate,
  })
}

export function useSetRating() {
  const invalidate = useInvalidateUserState()
  return useMutation({
    mutationFn: ({ kind, id, rating }: { kind: TitleKind; id: string; rating: number | null }) =>
      apiFetch<void>(`${BASE}/ratings/${kind}/${id}`, { method: 'PUT', json: { rating } }),
    onSuccess: invalidate,
  })
}

export function useSetWatched() {
  const invalidate = useInvalidateUserState()
  return useMutation({
    mutationFn: ({ kind, id, watched }: { kind: 'movie' | 'episode'; id: string; watched: boolean }) =>
      apiFetch<void>(`${BASE}/progress/${kind}/${id}${watched ? '/watched' : ''}`, {
        method: watched ? 'PUT' : 'DELETE',
      }),
    onSuccess: invalidate,
  })
}

export function usePatchTitle(kind: TitleKind, id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (patch: TitlePatch) =>
      apiFetch<void>(`${BASE}/${kind === 'movie' ? 'movies' : 'shows'}/${id}`, { method: 'PATCH', json: patch }),
    onSuccess: invalidate,
  })
}

/** Nahrání vlastního plakátu nebo pozadí (tělo = soubor); obnova metadat ho nepřepíše. */
export function useUploadTitleImage(kind: TitleKind, id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ image, file }: { image: TitleImageKind; file: File }) =>
      apiUpload<void>(`${BASE}/${kind === 'movie' ? 'movies' : 'shows'}/${id}/images/${image}`, file, { method: 'PUT' }),
    onSuccess: invalidate,
  })
}

/** Odebrání vlastního obrázku – vrátí se obrázek ze zdroje metadat, pokud zůstal na disku. */
export function useRemoveTitleImage(kind: TitleKind, id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (image: TitleImageKind) =>
      apiFetch<void>(`${BASE}/${kind === 'movie' ? 'movies' : 'shows'}/${id}/images/${image}`, { method: 'DELETE' }),
    onSuccess: invalidate,
  })
}

// ---- párování se zdrojem metadat (editor) ----

/** Zdroje video metadat (pluginy) pro výběr u knihovny a při párování. */
export function useMetadataProviders() {
  return useQuery({
    queryKey: queryKeys.metadataProviders,
    queryFn: async () => list(await apiFetch<MetadataProvider[] | null>(`${BASE}/metadata/providers`)),
    staleTime: 5 * 60 * 1000,
  })
}

/**
 * Hledání u zdroje. Prázdný provider = výchozí zdroj knihovny titulu
 * (server ho dohledá podle `title`); odpověď říká, který zdroj hledal.
 */
export function useMetadataSearch(kind: TitleKind, provider: string, q: string, year?: number, title?: string) {
  return useQuery({
    queryKey: queryKeys.metadataSearch(kind, provider, q, year, title),
    queryFn: async () => {
      const r = await apiFetch<MetadataSearchResponse>(
        `${BASE}/metadata/search${params({ kind, provider, q, year: year ? String(year) : undefined, title })}`,
      )
      return { provider: r.provider, results: list(r.results) }
    },
    enabled: q.trim().length > 1,
    staleTime: 60_000,
    retry: false,
  })
}

export function useMatchTitle() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ kind, id, provider, metaId }: { kind: TitleKind; id: string; provider: string; metaId: string }) =>
      apiFetch<{ id: string }>(`${BASE}/${kind === 'movie' ? 'movies' : 'shows'}/${id}/match`, {
        method: 'PUT',
        json: { provider, id: metaId },
      }),
    onSuccess: invalidate,
  })
}

/** Oddělí soubor od filmu, se kterým se chybně sloučil; vrací ID nového filmu. */
export function useDetachFile() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (mediaId: string) => apiFetch<{ movie_id: string }>(`${BASE}/media/${mediaId}/detach`, { method: 'POST' }),
    onSuccess: invalidate,
  })
}

/** Přesune soubor do koše knihovny (.trash) a odebere ho z knihovny. */
export function useTrashFile() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (mediaId: string) => apiFetch<{ trash_path: string }>(`${BASE}/media/${mediaId}`, { method: 'DELETE' }),
    onSuccess: invalidate,
  })
}

export function useSetMatchState() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ kind, id, state }: { kind: TitleKind; id: string; state: 'unmatched' | 'ignored' }) =>
      apiFetch<void>(`${BASE}/${kind === 'movie' ? 'movies' : 'shows'}/${id}/${state === 'ignored' ? 'ignore' : 'match'}`, {
        method: state === 'ignored' ? 'PUT' : 'DELETE',
      }),
    onSuccess: invalidate,
  })
}

// ---- administrace ----

export function useAdminStats() {
  return useQuery({
    queryKey: queryKeys.adminStats,
    queryFn: () => apiFetch<SceniterStats>(`${BASE}/admin/stats`),
    // Během kontroly knihovny se stav obnovuje průběžně.
    refetchInterval: (query) => (query.state.data?.scanner.running ? 2000 : false),
  })
}

/**
 * Nespárované tituly jednoho druhu – stránkovaně. attempted: true jsou ty,
 * pro které žádný zdroj nenašel jistou shodu, false ty, na které párování
 * ještě nedošlo.
 */
export function useAdminUnmatched<K extends TitleKind>(
  kind: K,
  attempted: boolean,
): PagedList<K extends 'movie' ? MovieSummary : ShowSummary> {
  return usePagedList({
    queryKey: queryKeys.adminUnmatchedList(kind, attempted),
    fetchPage: pageFetcher('/admin/library/unmatched', { kind, attempted: attempted ? '1' : '0' }),
  })
}

/** Seriály s podezřelým číslováním dílů (kontrola v administraci). */
export function useAdminShowIssues() {
  return useQuery({
    queryKey: queryKeys.adminShowIssues,
    queryFn: async () => list((await apiFetch<{ items: ShowIssue[] | null }>(`${BASE}/admin/library/show-issues`)).items),
  })
}

/** Filmy s přisloučeným cizím souborem (kontrola v administraci). */
export function useAdminMovieIssues() {
  return useQuery({
    queryKey: queryKeys.adminMovieIssues,
    queryFn: async () => list((await apiFetch<{ items: MovieIssue[] | null }>(`${BASE}/admin/library/movie-issues`)).items),
  })
}

/** Složky seriálu se soubory dílů (editor číslování). */
export function useShowFolders(showId: string) {
  return useQuery({
    queryKey: queryKeys.showFolders(showId),
    queryFn: async () => list((await apiFetch<{ items: ShowFolder[] | null }>(`${BASE}/shows/${showId}/folders`)).items),
    enabled: Boolean(showId),
  })
}

/**
 * Přečísluje soubory jedné složky seriálu: přejmenuje je na disku na
 * "SxxEyy" a přeřadí k dílům. S dryRun jen vrátí plán přejmenování.
 */
export function useRenumber() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, dir, season, dryRun, files }: {
      id: string
      dir: string
      season: number
      dryRun: boolean
      files: { media_id: string; episode: number }[]
    }) =>
      apiFetch<RenumberPlan>(`${BASE}/shows/${id}/renumber`, {
        method: 'POST',
        json: { dir, season, dry_run: dryRun, files },
      }),
    onSuccess: (_plan, vars) => {
      if (!vars.dryRun) invalidate()
    },
  })
}

/** Koše knihoven (.trash) s videi. */
export function useAdminTrash() {
  return useQuery({
    queryKey: queryKeys.adminTrash,
    queryFn: async () => list((await apiFetch<{ items: TrashLibrary[] | null }>(`${BASE}/admin/trash`)).items),
  })
}

/** Vrátí video z koše na původní místo a do knihovny. */
export function useRestoreTrash() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (path: string) => apiFetch<{ path: string }>(`${BASE}/admin/trash/restore`, { method: 'POST', json: { path } }),
    onSuccess: invalidate,
  })
}

/** Trvale smaže video z koše. */
export function useDeleteTrash() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (path: string) => apiFetch<void>(`${BASE}/admin/trash/delete`, { method: 'POST', json: { path } }),
    onSuccess: invalidate,
  })
}

/** Vysype koš knihovny (library = složka knihovny). */
export function useEmptyTrash() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (library: string) => apiFetch<{ freed_bytes: number }>(`${BASE}/admin/trash/empty`, { method: 'POST', json: { library } }),
    onSuccess: invalidate,
  })
}

/** Průběh obnovy metadat; během obnovy se obnovuje každé 3 s. */
export function useRefreshStatus() {
  return useQuery({
    queryKey: queryKeys.adminRefresh,
    queryFn: () => apiFetch<RefreshStatus>(`${BASE}/admin/metadata/refresh`),
    refetchInterval: (query) => (query.state.data?.running ? 3000 : false),
  })
}

/**
 * Průběh automatického párování; během párování nebo pauzy zdroje se
 * obnovuje průběžně. Po doběhnutí obnoví počty a seznam nespárovaných.
 */
export function useMatchStatus() {
  const qc = useQueryClient()
  const query = useQuery({
    queryKey: queryKeys.adminMatch,
    queryFn: () => apiFetch<MatchStatus>(`${BASE}/admin/metadata/match`),
    refetchInterval: (query) => (query.state.data?.running ? 2000 : query.state.data?.paused_until ? 15000 : false),
  })
  const running = query.data?.running ?? false
  const wasRunning = useRef(false)
  useEffect(() => {
    if (wasRunning.current && !running) {
      void qc.invalidateQueries({ queryKey: queryKeys.adminStats })
      void qc.invalidateQueries({ queryKey: queryKeys.adminUnmatched })
    }
    wasRunning.current = running
  }, [qc, running])
  return query
}

export function useAdminTranscodes() {
  return useQuery({
    queryKey: queryKeys.adminTranscode,
    queryFn: async () => list(await apiFetch<TranscodeSession[] | null>(`${BASE}/admin/transcode`)),
    // Jen když něco běží – nečinná administrace server nezatěžuje.
    refetchInterval: (query) => ((query.state.data?.length ?? 0) > 0 ? 5000 : 30_000),
    refetchIntervalInBackground: false,
  })
}

export function useAdminAction() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ path, method = 'POST' }: { path: string; method?: 'POST' | 'DELETE' }) =>
      apiFetch<unknown>(`${BASE}${path}`, { method }),
    onSuccess: invalidate,
  })
}

// ---- knihovny (admin) ----

export function useAdminLibraries() {
  return useQuery({
    queryKey: queryKeys.adminLibraries,
    queryFn: async () => list(await apiFetch<Library[] | null>(`${BASE}/admin/libraries`)),
  })
}

export function useLibraryCandidates(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.libraryCandidates,
    queryFn: async () => list((await apiFetch<{ dirs: string[] | null }>(`${BASE}/admin/libraries/candidates`)).dirs),
    enabled,
  })
}

export function useSaveLibrary() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id?: string; body: LibraryRequest }) =>
      apiFetch<Library>(id ? `${BASE}/admin/libraries/${id}` : `${BASE}/admin/libraries`, {
        method: id ? 'PATCH' : 'POST',
        json: body,
      }),
    onSuccess: invalidate,
  })
}

export function useDeleteLibrary() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (id: string) => apiFetch<void>(`${BASE}/admin/libraries/${id}`, { method: 'DELETE' }),
    onSuccess: invalidate,
  })
}

export function useReorderLibraries() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch<Library[]>(`${BASE}/admin/libraries/order`, { method: 'PUT', json: { ids } }),
    onSuccess: invalidate,
  })
}
