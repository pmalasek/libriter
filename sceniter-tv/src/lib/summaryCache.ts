import { useMemo } from 'react'
import { useQueryClient, type InfiniteData, type QueryClient } from '@tanstack/react-query'
import {
  queryKeys,
  type Home,
  type Movie,
  type MovieSummary,
  type Page,
  type SearchResults,
  type Show,
  type ShowSummary,
  type TitleKind,
} from 'sceniter-shared'

/** Co o titulu víme ze seznamu, ze kterého se do detailu přišlo. */
export type CachedTitle = (MovieSummary | ShowSummary) & Partial<Pick<Movie, 'overview' | 'genres' | 'runtime_min'>>

type Lists = { movies: Movie[]; shows: Show[] }

function find(qc: QueryClient, kind: TitleKind, id: string): CachedTitle | undefined {
  const hit = (t: { id: string } | undefined) => t?.id === id
  for (const [, data] of qc.getQueriesData<InfiniteData<Page<CachedTitle>>>({ queryKey: ['sceniter', 'tv-titles', kind] })) {
    for (const page of data?.pages ?? []) {
      const found = page.items.find(hit)
      if (found) return found
    }
  }
  const home = qc.getQueryData<Home>(queryKeys.home)
  if (home) {
    const pool: (Movie | Show | undefined)[] =
      kind === 'movie'
        ? [...home.libraries.flatMap((l) => l.movies), ...home.continue.map((c) => c.movie), ...home.watchlist.map((w) => w.movie)]
        : [...home.libraries.flatMap((l) => l.shows), ...home.continue.map((c) => c.show), ...home.watchlist.map((w) => w.show)]
    const found = pool.find(hit)
    if (found) return found
  }
  for (const key of [['sceniter', 'search'], ['sceniter', 'similar']]) {
    for (const [, data] of qc.getQueriesData<SearchResults | Lists>({ queryKey: key })) {
      const found = (kind === 'movie' ? data?.movies : data?.shows)?.find(hit)
      if (found) return found
    }
  }
  return undefined
}

/**
 * Titul z už načtených seznamů (výpis knihovny, domů, hledání, podobné) –
 * detail podle něj ukáže pozadí a název hned, než dojde plný detail.
 */
export function useCachedTitle(kind: TitleKind, id: string): CachedTitle | undefined {
  const qc = useQueryClient()
  return useMemo(() => (id ? find(qc, kind, id) : undefined), [qc, kind, id])
}
