import type { User } from '@/api/types'

const STORAGE_KEY = 'libriter.session'

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
    const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const decoded: unknown = JSON.parse(atob(payload))
    if (!decoded || typeof decoded !== 'object') return true

    const exp = (decoded as { exp?: unknown }).exp
    if (typeof exp !== 'number') return true

    return exp * 1000 <= Date.now()
  } catch {
    return true
  }
}

export function loadSession(): Session | null {
  let raw: string | null = null
  try {
    raw = localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
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

export function saveSession(session: Session) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(session))
  } catch {
    // private mode / zakázané storage – session přežije jen do reloadu
  }
}

export function clearSession() {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignorujeme
  }
}
