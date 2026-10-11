import { Platform } from 'react-native'
import { getPlayback, type PlaybackMode } from 'sceniter-shared'

export const PROFILE = Platform.OS === 'ios' ? 'tvos' : 'androidtv'

/**
 * Rozhodnutí o přehrávání – sdílené přehrávačem a přednačítáním z detailu.
 * Server při něm jen vybere režim a vydá token, převod nespouští, takže se
 * smí ptát dopředu. gcTime > 0, jinak by se přednačtená odpověď hned zahodila;
 * staleTime Infinity, aby běžící přehrávač nedostal pod rukama nový objekt.
 */
export function playbackQueryOptions(mediaId: string, o: { audio?: number; mode?: PlaybackMode; attempt?: number } = {}) {
  return {
    queryKey: ['sceniter', 'playback', mediaId, PROFILE, o.audio ?? null, o.mode ?? null, o.attempt ?? 0] as const,
    queryFn: () => getPlayback(mediaId, { profile: PROFILE, audio: o.audio, mode: o.mode }),
    staleTime: Infinity,
    gcTime: 60_000,
    retry: false,
  }
}
