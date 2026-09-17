import type { User } from './types'

/**
 * Přihlášení uložené u klienta. Perzistenci si řeší každá aplikace sama –
 * web `localStorage`, mobil SecureStore – protože se liší i v tom, co je
 * bezpečné. Společné je jen to, jak se session čte a jak se pozná prošlý token.
 */
export interface Session {
  token: string
  user: User
}

/** Dekóduje `exp` z JWT payloadu. Bez podpisu – jen aby klient zbytečně
 *  neposílal prošlý token; autoritou zůstává backend. */
export function isTokenExpired(token: string): boolean {
  const parts = token.split('.')
  if (parts.length !== 3) return true

  try {
    const decoded: unknown = JSON.parse(decodeBase64Url(parts[1]))
    if (!decoded || typeof decoded !== 'object') return true

    const exp = (decoded as { exp?: unknown }).exp
    if (typeof exp !== 'number') return true

    return exp * 1000 <= Date.now()
  } catch {
    return true
  }
}

/**
 * Rozebere uloženou session. Vrací null na cokoliv, co nedává smysl –
 * poškozený zápis i prošlý token. Volající tak řeší jedinou větev: buď je
 * session použitelná, nebo se uživatel přihlásí znovu.
 */
export function parseSession(raw: string | null | undefined): Session | null {
  if (!raw) return null

  try {
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null

    const { token, user } = parsed as Partial<Session>
    if (typeof token !== 'string' || !user || typeof user !== 'object') return null
    if (isTokenExpired(token)) return null

    return { token, user: user as User }
  } catch {
    return null
  }
}

/** Serializuje session k uložení. Protipól parseSession. */
export function serializeSession(session: Session): string {
  return JSON.stringify(session)
}

/**
 * Base64url → text. `atob` je v prohlížeči i v Hermesu (React Native), ale
 * jen pro standardní base64; JWT používá variantu bez výplně a s `-_`.
 */
function decodeBase64Url(value: string): string {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=')
  return atob(padded)
}
