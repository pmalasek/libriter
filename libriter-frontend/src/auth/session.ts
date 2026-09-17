// Čtení a platnost session se sdílí s mobilní aplikací
// (libriter-shared/src/session.ts); perzistence je na každé aplikaci zvlášť,
// protože se liší úložiště: tady localStorage, v mobilu SecureStore.
import { parseSession, serializeSession, type Session } from 'libriter-shared'

export { isTokenExpired, type Session } from 'libriter-shared'

const STORAGE_KEY = 'libriter.session'

export function loadSession(): Session | null {
  try {
    return parseSession(localStorage.getItem(STORAGE_KEY))
  } catch {
    // private mode / zakázané storage
    return null
  }
}

export function saveSession(session: Session) {
  try {
    localStorage.setItem(STORAGE_KEY, serializeSession(session))
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
