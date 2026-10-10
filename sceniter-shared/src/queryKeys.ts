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
  season: (showId: string, number: number) => ['sceniter', 'show', showId, 'season', number] as const,
  episode: (id: string) => ['sceniter', 'episode', id] as const,
  person: (id: string) => ['sceniter', 'person', id] as const,
  similar: (kind: string, id: string) => ['sceniter', 'similar', kind, id] as const,
  genres: ['sceniter', 'genres'] as const,
  search: (q: string) => ['sceniter', 'search', q] as const,
  watchlist: ['sceniter', 'watchlist'] as const,
  playback: (mediaId: string, audio?: number) => ['sceniter', 'playback', mediaId, audio ?? null] as const,
  adminStats: ['sceniter', 'admin', 'stats'] as const,
  adminUnmatched: ['sceniter', 'admin', 'unmatched'] as const,
  adminUnmatchedList: (kind: string, attempted: boolean) =>
    ['sceniter', 'admin', 'unmatched', kind, attempted] as const,
  adminTranscode: ['sceniter', 'admin', 'transcode'] as const,
  adminRefresh: ['sceniter', 'admin', 'refresh'] as const,
  adminMatch: ['sceniter', 'admin', 'match'] as const,
  libraries: ['sceniter', 'libraries'] as const,
  adminLibraries: ['sceniter', 'admin', 'libraries'] as const,
  libraryCandidates: ['sceniter', 'admin', 'libraries', 'candidates'] as const,
  metadataProviders: ['sceniter', 'metadata', 'providers'] as const,
  metadataSearch: (kind: string, provider: string, q: string, year?: number, title?: string) =>
    ['sceniter', 'metadata', 'search', kind, provider, q, year ?? null, title ?? null] as const,
}
