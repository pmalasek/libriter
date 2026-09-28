/**
 * Přepis systémových adres před tím, než je uvidí router.
 *
 * react-native-track-player na Androidu otevírá aplikaci natvrdo adresou
 * `trackplayer://notification.click` (klepnutí na notifikaci) a
 * `trackplayer://service-bound` (probuzení službou, např. Android Auto).
 * Router takové trasy nezná a ukázal by „Unmatched Route“. Míří se domů,
 * ne na /player: modal jako první obrazovka po studeném startu by neměl
 * kam jít zpět a stav přehrávače se v tu chvíli teprve obnovuje.
 */
export function redirectSystemPath({ path }: { path: string; initial: boolean }): string {
  if (path.startsWith('trackplayer://')) return '/'
  return path
}
