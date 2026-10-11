import { router } from 'expo-router'

export interface PlayParams {
  mediaId: string
  movieId?: string
  episodeId?: string
  title: string
  subtitle?: string
  /** Pozice v sekundách, od které pokračovat; 0 = od začátku. */
  start?: number
}

export const openMovie = (id: string) => router.push({ pathname: '/movie/[id]', params: { id } } as never)
export const openShow = (id: string) => router.push({ pathname: '/show/[id]', params: { id } } as never)
export const openPerson = (id: string) => router.push({ pathname: '/person/[id]', params: { id } } as never)

export function play(p: PlayParams) {
  const params: Record<string, string> = { mediaId: p.mediaId, title: p.title }
  if (p.movieId) params.movieId = p.movieId
  if (p.episodeId) params.episodeId = p.episodeId
  if (p.subtitle) params.subtitle = p.subtitle
  if (p.start) params.start = String(Math.floor(p.start))
  router.push({ pathname: '/player', params } as never)
}
