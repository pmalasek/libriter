import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  apiFetch,
  queryKeys,
  type EpisodeDetail,
  type Home,
  type Movie,
  type MovieDetail,
  type Playback,
  type ProgressRequest,
  type SceniterConfig,
  type SceniterStats,
  type SearchResults,
  type Show,
  type ShowDetail,
  type TitleKind,
  type TitlePatch,
  type TmdbResult,
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

export interface ListParams extends Record<string, string> {
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
    queryFn: () => apiFetch<SearchResults>(`${BASE}/search${params({ q })}`),
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

// ---- párování s TMDB (editor) ----

export function useTmdbSearch(kind: TitleKind, q: string, year?: number) {
  return useQuery({
    queryKey: queryKeys.tmdbSearch(kind, q, year),
    queryFn: async () =>
      list(
        await apiFetch<TmdbResult[] | null>(
          `${BASE}/metadata/search${params({ kind, q, year: year ? String(year) : undefined })}`,
        ),
      ),
    enabled: q.trim().length > 1,
    staleTime: 60_000,
    retry: false,
  })
}

export function useMatchTitle() {
  const invalidate = useInvalidateUserState()
  return useMutation({
    mutationFn: ({ kind, id, tmdbId }: { kind: TitleKind; id: string; tmdbId: number }) =>
      apiFetch<{ id: string }>(`${BASE}/${kind === 'movie' ? 'movies' : 'shows'}/${id}/match`, {
        method: 'PUT',
        json: { tmdb_id: tmdbId },
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
