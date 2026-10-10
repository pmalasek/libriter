import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  apiFetch,
  queryKeys,
  type EpisodeDetail,
  type Home,
  type Library,
  type LibraryRequest,
  type MetadataProvider,
  type MetadataSearchResponse,
  type Movie,
  type MovieDetail,
  type PersonDetail,
  type Playback,
  type ProgressRequest,
  type RefreshStatus,
  type SceniterConfig,
  type SceniterStats,
  type SearchResults,
  type Show,
  type ShowDetail,
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

export function useMovies(p: ListParams) {
  return useQuery({
    queryKey: queryKeys.movies(p),
    queryFn: async () => list(await apiFetch<Movie[] | null>(`${BASE}/movies${params(p)}`)),
    placeholderData: (prev) => prev,
  })
}

export function useMovie(id: string) {
  return useQuery({
    queryKey: queryKeys.movie(id),
    queryFn: () => apiFetch<MovieDetail>(`${BASE}/movies/${id}`),
    enabled: Boolean(id),
  })
}

export function useShows(p: ListParams) {
  return useQuery({
    queryKey: queryKeys.shows(p),
    queryFn: async () => list(await apiFetch<Show[] | null>(`${BASE}/shows${params(p)}`)),
    placeholderData: (prev) => prev,
  })
}

export function useShow(id: string) {
  return useQuery({
    queryKey: queryKeys.show(id),
    queryFn: () => apiFetch<ShowDetail>(`${BASE}/shows/${id}`),
    enabled: Boolean(id),
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

/** Po změně stavu uživatele (seznam, hodnocení, zhlédnutí) se obnoví výpisy. */
function useInvalidateUserState() {
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
  const invalidate = useInvalidateUserState()
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
  const invalidate = useInvalidateUserState()
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
  const invalidate = useInvalidateUserState()
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

export function useAdminUnmatched() {
  return useQuery({
    queryKey: queryKeys.adminUnmatched,
    queryFn: async () => {
      const r = await apiFetch<{ movies: Movie[] | null; shows: Show[] | null }>(`${BASE}/admin/library/unmatched`)
      return { movies: list(r.movies), shows: list(r.shows) }
    },
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

export function useAdminTranscodes() {
  return useQuery({
    queryKey: queryKeys.adminTranscode,
    queryFn: async () => list(await apiFetch<TranscodeSession[] | null>(`${BASE}/admin/transcode`)),
    refetchInterval: 5000,
  })
}

export function useAdminAction() {
  const invalidate = useInvalidateUserState()
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
  const invalidate = useInvalidateUserState()
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
  const invalidate = useInvalidateUserState()
  return useMutation({
    mutationFn: (id: string) => apiFetch<void>(`${BASE}/admin/libraries/${id}`, { method: 'DELETE' }),
    onSuccess: invalidate,
  })
}

export function useReorderLibraries() {
  const invalidate = useInvalidateUserState()
  return useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch<Library[]>(`${BASE}/admin/libraries/order`, { method: 'PUT', json: { ids } }),
    onSuccess: invalidate,
  })
}
