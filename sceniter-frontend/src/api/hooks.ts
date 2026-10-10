import { useMutation, useQuery, useQueryClient, type Query } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { usePagedList, type PagedList } from 'core-web/src/lib/paged'
import {
  apiFetch,
  listQuery,
  queryKeys,
  type Page,
  type EpisodeDetail,
  type Home,
  type Library,
  type LibraryRequest,
  type MatchStatus,
  type MetadataProvider,
  type MetadataSearchResponse,
  type Movie,
  type MovieDetail,
  type MovieSummary,
  type PersonDetail,
  type Playback,
  type ProgressRequest,
  type RefreshStatus,
  type SceniterConfig,
  type SceniterStats,
  type SearchResults,
  type Season,
  type Show,
  type ShowDetail,
  type ShowSummary,
  type TitleKind,
  type TitlePatch,
  type TranscodeSession,
  type WatchlistItem,
} from 'sceniter-shared'

const BASE = '/sceniter'

/** Seznam z API; null (prázdný Go slice) se převede na []. */
function list<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}

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
  return useQuery({ queryKey: queryKeys.home, queryFn: () => apiFetch<Home>(`${BASE}/home`) })
}

/** Zapnuté knihovny v nastaveném pořadí – menu a domovská stránka. */
export function useLibraries() {
  return useQuery({
    queryKey: queryKeys.libraries,
    queryFn: async () => list(await apiFetch<Library[] | null>(`${BASE}/libraries`)),
    staleTime: 60_000,
  })
}

export interface ListParams extends Record<string, string> {
  library: string
  q: string
  genre: string
  sort: string
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
  return usePagedList({ queryKey: queryKeys.movies(p), fetchPage: pageFetcher<MovieSummary>('/movies', p) })
}

export function useMovie(id: string) {
  return useQuery({
    queryKey: queryKeys.movie(id),
    queryFn: () => apiFetch<MovieDetail>(`${BASE}/movies/${id}`),
    enabled: Boolean(id),
  })
}

/** Seriály podle filtru – stránkovaně. */
export function useShows(p: ListParams): PagedList<ShowSummary> {
  return usePagedList({ queryKey: queryKeys.shows(p), fetchPage: pageFetcher<ShowSummary>('/shows', p) })
}

export function useShow(id: string) {
  return useQuery({
    queryKey: queryKeys.show(id),
    queryFn: () => apiFetch<ShowDetail>(`${BASE}/shows/${id}`),
    enabled: Boolean(id),
  })
}

/** Díly jedné řady s rozkoukaností – detail seriálu je načítá po řadách. */
export function useSeason(showId: string, number: number | undefined) {
  return useQuery({
    queryKey: queryKeys.season(showId, number ?? -1),
    queryFn: async () => {
      const s = await apiFetch<Season>(`${BASE}/shows/${showId}/seasons/${number}`)
      return { ...s, episodes: list(s.episodes) }
    },
    enabled: Boolean(showId) && number !== undefined,
  })
}

export function useEpisode(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.episode(id ?? ''),
    queryFn: () => apiFetch<EpisodeDetail>(`${BASE}/episodes/${id}`),
    enabled: Boolean(id),
  })
}

/** Osoba a její filmy a seriály v knihovně. */
export function usePerson(id: string) {
  return useQuery({
    queryKey: queryKeys.person(id),
    queryFn: async () => {
      const p = await apiFetch<PersonDetail>(`${BASE}/people/${id}`)
      return { ...p, credits: list(p.credits) }
    },
    enabled: Boolean(id),
  })
}

/** Podobné tituly (stejná knihovna, společné žánry). */
export function useSimilar(kind: TitleKind, id: string) {
  return useQuery({
    queryKey: queryKeys.similar(kind, id),
    queryFn: async () =>
      kind === 'movie'
        ? { movies: list(await apiFetch<Movie[] | null>(`${BASE}/movies/${id}/similar`)), shows: [] as Show[] }
        : { movies: [] as Movie[], shows: list(await apiFetch<Show[] | null>(`${BASE}/shows/${id}/similar`)) },
    enabled: Boolean(id),
    staleTime: 5 * 60 * 1000,
  })
}

export function useGenres() {
  return useQuery({
    queryKey: queryKeys.genres,
    queryFn: async () => list(await apiFetch<string[] | null>(`${BASE}/genres`)),
    staleTime: 5 * 60 * 1000,
  })
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: queryKeys.search(q),
    queryFn: async () => {
      const r = await apiFetch<SearchResults>(`${BASE}/search${params({ q })}`)
      return { movies: list(r.movies), shows: list(r.shows), people: list(r.people) }
    },
    enabled: q.trim().length > 0,
    placeholderData: (prev) => prev,
  })
}

export function useWatchlist() {
  return useQuery({
    queryKey: queryKeys.watchlist,
    queryFn: async () => list(await apiFetch<WatchlistItem[] | null>(`${BASE}/watchlist`)),
  })
}

/**
 * Rozhodnutí o přehrávání. Nekešuje se: adresy nesou token s omezenou
 * platností a server při každém dotazu ověří stav souboru.
 */
export function usePlayback(mediaId: string | undefined, profile: string, audio?: number, mode?: string) {
  return useQuery({
    queryKey: [...queryKeys.playback(mediaId ?? '', audio), profile, mode ?? null],
    queryFn: () =>
      apiFetch<Playback>(
        `${BASE}/media/${mediaId}/playback${params({ profile, audio: audio?.toString(), mode })}`,
      ),
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

/** Zápis pozice z přehrávače; 409 (novější pozice jinde) není chyba. */
export function saveProgress(body: ProgressRequest, keepalive = false): Promise<void> {
  return apiFetch<void>(`${BASE}/progress`, { method: 'PUT', json: body, keepalive }).catch((err: unknown) => {
    if (err && typeof err === 'object' && 'status' in err && err.status === 409) return
    throw err
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
