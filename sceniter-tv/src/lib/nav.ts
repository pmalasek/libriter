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

/**
 * Hledat z tlačítka v knihovně: sekce se přepínají `replace` jako z bočního
 * menu (Zpět pak vede do menu). `focus` dá fokus rovnou do pole s klávesnicí.
 */
export const openSearch = () => router.replace({ pathname: '/search', params: { focus: '1' } } as never)

/**
 * Z přehrávače na detail seriálu se zaměřeným dílem. Když přehrávač leží
 * přímo nad detailem toho seriálu (přišlo se z něj), stačí krok zpět a nové
 * params – přehrávač se odmontuje a video se zastaví. Jinak přehrávač nahradí
 * (Zpět pak vede tam, odkud se přehrávalo – třeba na Domů).
 *
 * `router.dismissTo` se nehodí: cestu porovnává i s params, takže detail
 * s jinými params nenajde a otevřel by druhý nad přehrávačem.
 */
export function goToShowEpisode(e: { show_id: string; id: string; season: number }, belowPlayer: boolean) {
  const params = { episode: e.id, season: String(e.season) }
  if (belowPlayer) {
    router.back()
    router.setParams(params as never)
  } else router.replace({ pathname: '/show/[id]', params: { id: e.show_id, ...params } } as never)
}

export function play(p: PlayParams) {
  const params: Record<string, string> = { mediaId: p.mediaId, title: p.title }
  if (p.movieId) params.movieId = p.movieId
  if (p.episodeId) params.episodeId = p.episodeId
  if (p.subtitle) params.subtitle = p.subtitle
  if (p.start) params.start = String(Math.floor(p.start))
  router.push({ pathname: '/player', params } as never)
}

/**
 * Zpět uvnitř hlavní sekce (bez zásobníku nad ní). Nastavuje ji layout sekcí;
 * true = Zpět vyřízené (zaměřilo se menu). Jediný handler Zpět je v root
 * layoutu, aby pořadí registrací BackHandleru nerozhodovalo o chování.
 */
let sectionBack: (() => boolean) | null = null
export function setSectionBack(fn: (() => boolean) | null) {
  sectionBack = fn
}
export const handleSectionBack = () => sectionBack?.() ?? false
