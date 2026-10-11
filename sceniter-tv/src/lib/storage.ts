import * as Crypto from 'expo-crypto'
import * as SecureStore from 'expo-secure-store'
import { parseSession, serializeSession, type Session } from 'sceniter-shared'
import { isSort, isSortDir, type Sort, type SortDir } from './sort'

// Token je klíč k celé knihovně, proto SecureStore (Keychain / Keystore).
// Ostatní jsou drobná nastavení – SecureStore stačí i na ně a TV aplikace
// tak nepotřebuje další úložiště.
const KEYS = {
  session: 'sceniter.session',
  server: 'sceniter.server',
  device: 'sceniter.device',
  prefs: 'sceniter.prefs',
  servers: 'sceniter.servers',
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
export const saveServer = async (url: string) => {
  await set(KEYS.server, url)
  await rememberServer(url)
}

const RECENT_SERVERS = 5

/** Naposledy použité servery (nejnovější první) – nabízí je zadání adresy. */
export async function loadRecentServers(): Promise<string[]> {
  try {
    const list: unknown = JSON.parse((await get(KEYS.servers)) ?? '[]')
    return Array.isArray(list) ? list.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

async function rememberServer(url: string): Promise<void> {
  const list = [url, ...(await loadRecentServers()).filter((x) => x !== url)].slice(0, RECENT_SERVERS)
  await set(KEYS.servers, JSON.stringify(list))
}

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
  /** Zvolené řazení podle ID knihovny. */
  librarySort: Record<string, Sort>
  /** Směr řazení podle ID knihovny; chybí = přirozený směr klíče (SORT_DIR). */
  librarySortDir: Record<string, SortDir>
}

export const defaultPrefs: Prefs = { subtitles: 'off', subtitleSize: 'medium', librarySort: {}, librarySortDir: {} }

export async function loadPrefs(): Promise<Prefs> {
  try {
    const stored = JSON.parse((await get(KEYS.prefs)) ?? '{}') as Partial<Prefs>
    const librarySort: Record<string, Sort> = {}
    for (const [id, sort] of Object.entries(stored.librarySort ?? {})) if (isSort(sort)) librarySort[id] = sort
    const librarySortDir: Record<string, SortDir> = {}
    for (const [id, dir] of Object.entries(stored.librarySortDir ?? {})) if (isSortDir(dir)) librarySortDir[id] = dir
    return { ...defaultPrefs, ...stored, librarySort, librarySortDir }
  } catch {
    return defaultPrefs
  }
}

export const savePrefs = (p: Prefs) => set(KEYS.prefs, JSON.stringify(p))
