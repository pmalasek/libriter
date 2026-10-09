import { coreQueryKeys } from 'core-shared'

/** Klíče react-query Sceniteru (včetně klíčů jádra). */
export const queryKeys = {
  ...coreQueryKeys,
  config: ['sceniter', 'config'] as const,
  home: ['sceniter', 'home'] as const,
  movies: (params: Record<string, string> = {}) => ['sceniter', 'movies', params] as const,
  moviesAll: ['sceniter', 'movies'] as const,
  movie: (id: string) => ['sceniter', 'movie', id] as const,
  shows: (params: Record<string, string> = {}) => ['sceniter', 'shows', params] as const,
  showsAll: ['sceniter', 'shows'] as const,
  show: (id: string) => ['sceniter', 'show', id] as const,
  episode: (id: string) => ['sceniter', 'episode', id] as const,
  genres: ['sceniter', 'genres'] as const,
  search: (q: string) => ['sceniter', 'search', q] as const,
  watchlist: ['sceniter', 'watchlist'] as const,
  playback: (mediaId: string, audio?: number) => ['sceniter', 'playback', mediaId, audio ?? null] as const,
  adminStats: ['sceniter', 'admin', 'stats'] as const,
  adminUnmatched: ['sceniter', 'admin', 'unmatched'] as const,
  adminTranscode: ['sceniter', 'admin', 'transcode'] as const,
  tmdbSearch: (kind: string, q: string, year?: number) => ['sceniter', 'tmdb', kind, q, year ?? null] as const,
}
