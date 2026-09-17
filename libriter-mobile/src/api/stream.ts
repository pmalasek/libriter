import { apiFetch, type StreamToken } from 'libriter-shared'

/**
 * Token do adresy audia. Platí 24 hodin, takže stahování dlouhé knihy i delší
 * poslech si o něj musí říct znovu – volající si ho nedrží napořád.
 */
export function fetchStreamToken(): Promise<StreamToken> {
  return apiFetch<StreamToken>('/auth/stream-token')
}
