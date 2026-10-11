import * as Crypto from 'expo-crypto'
import * as SecureStore from 'expo-secure-store'
import { parseSession, serializeSession, type Session } from 'sceniter-shared'

// Token je klíč k celé knihovně, proto SecureStore (Keychain / Keystore).
// Ostatní jsou drobná nastavení – SecureStore stačí i na ně a TV aplikace
// tak nepotřebuje další úložiště.
const KEYS = {
  session: 'sceniter.session',
  server: 'sceniter.server',
  device: 'sceniter.device',
  prefs: 'sceniter.prefs',
}

async function get(key: string): Promise<string | null> {
  try {
    return await SecureStore.getItemAsync(key)
  } catch {
    return null
  }
}

async function set(key: string, value: string | null): Promise<void> {
  try {
    if (value === null) await SecureStore.deleteItemAsync(key)
    else await SecureStore.setItemAsync(key, value)
  } catch {
    // Nepovedený zápis znamená jen to, že se příště bude znovu párovat.
  }
}

export async function loadSession(): Promise<Session | null> {
  return parseSession(await get(KEYS.session))
}

export const saveSession = (s: Session | null) => set(KEYS.session, s ? serializeSession(s) : null)

export const loadServer = () => get(KEYS.server)
export const saveServer = (url: string) => set(KEYS.server, url)

let deviceIdCache: string | null = null

/** Stálé ID televize pro hlášení pozice (server podle něj řeší souběh zařízení). */
export async function deviceId(): Promise<string> {
  if (deviceIdCache) return deviceIdCache
  let id = await get(KEYS.device)
  if (!id) {
    id = Crypto.randomUUID()
    await set(KEYS.device, id)
  }
  deviceIdCache = id
  return id
}

export type SubtitleMode = 'off' | 'language'
export type SubtitleSize = 'small' | 'medium' | 'large'

export interface Prefs {
  subtitles: SubtitleMode
  subtitleSize: SubtitleSize
}

export const defaultPrefs: Prefs = { subtitles: 'off', subtitleSize: 'medium' }

export async function loadPrefs(): Promise<Prefs> {
  try {
    return { ...defaultPrefs, ...JSON.parse((await get(KEYS.prefs)) ?? '{}') }
  } catch {
    return defaultPrefs
  }
}

export const savePrefs = (p: Prefs) => set(KEYS.prefs, JSON.stringify(p))
