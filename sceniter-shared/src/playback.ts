// Pravidla přehrávání společná pro web a TV: kdy ukládat pozici, kdy zkusit
// dražší režim, jak udržet převod na serveru naživu. Bez DOMu a Reactu.

import type { PlaybackMode } from './types'

/** Jak často se během přehrávání ukládá pozice. */
export const SAVE_INTERVAL_MS = 10_000
/** Větší skok v čase mezi dvěma hlášeními polohy je přetočení, ne sledování. */
export const MAX_TIME_GAP = 3
/** Odpočet před automatickým spuštěním dalšího dílu. */
export const NEXT_COUNTDOWN = 10
/**
 * Jak často přehrávač serveru hlásí, že převod pořád potřebuje. Musí být
 * kratší než TRANSCODE_IDLE_SECONDS (výchozí 60 s, minimum 10 s).
 */
export const KEEPALIVE_MS = 20_000
/** Kolik sekund za místem pokračování musí být hotovo, než se na něj skočí. */
export const RESUME_AHEAD = 6

/** Pořadí režimů: když přehrání selže, zkusí se dražší. */
const ESCALATION: PlaybackMode[] = ['direct', 'remux', 'transcode']

/**
 * Režim, který zkusit po selhání přehrání v režimu `from`; undefined, když
 * dražší není (nebo server převádět neumí).
 */
export function escalateMode(from: PlaybackMode, transcodingAvailable: boolean): PlaybackMode | undefined {
  if (!transcodingAvailable) return undefined
  const i = ESCALATION.indexOf(from === 'transcode_audio' ? 'remux' : from)
  return ESCALATION[i + 1]
}

/**
 * Má smysl na uloženou pozici skočit? Ne na začátku a ne těsně před koncem
 * (to by video hned skončilo).
 */
export function shouldSeekTo(at: number | undefined, durationSec: number | undefined): at is number {
  return at !== undefined && at > 0 && at < (durationSec ?? Infinity) - 5
}

/** Rozdělí relativní adresu na cestu a hodnotu tokenu ?t=. */
function splitUrl(url: string): { path: string; token: string } {
  const [path, query = ''] = url.split('?', 2)
  const token = new URLSearchParams(query).get('t') ?? ''
  return { path, token }
}

/** Adresa keepalive převodu odvozená z adresy master playlistu. */
export function keepaliveUrl(masterUrl: string): string {
  const { path, token } = splitUrl(masterUrl)
  return `${path.replace(/master\.m3u8$/, 'keepalive')}?t=${encodeURIComponent(token)}`
}

/**
 * Adresa master playlistu, který spustí přebalení (převod zvuku) od místa
 * `start` (s) – skok mimo už hotovou část rostoucího playlistu.
 */
export function withStart(masterUrl: string, start: number): string {
  if (!(start > 0)) return masterUrl
  return `${masterUrl}${masterUrl.includes('?') ? '&' : '?'}start=${start.toFixed(3)}`
}

/**
 * Čas (s), od kterého převod běží – nula časové osy playlistu (klíčový
 * snímek před požadovaným startem). 0, když ho server neposlal.
 */
export function masterStart(masterText: string): number {
  const m = /#EXT-X-SESSION-DATA:.*DATA-ID="cz\.sceniter\.start".*VALUE="([\d.]+)"/.exec(masterText)
  const v = m ? Number(m[1]) : 0
  return Number.isFinite(v) && v > 0 ? v : 0
}

/**
 * Adresa pro ukončení převodu odvozená z adresy playlistu varianty
 * (…/hls/{session}/index.m3u8?t=…). Undefined, když tvar nesedí.
 */
export function stopUrlFromVariant(variantUrl: string | undefined): string | undefined {
  if (!variantUrl || !/\/index\.m3u8\?/.test(variantUrl)) return undefined
  return variantUrl.replace(/\/index\.m3u8\?/, '/stop?')
}

/** Absolutní adresa z relativní adresy serveru (TV klient nemá origin stránky). */
export function absoluteUrl(baseUrl: string, path: string): string {
  if (/^https?:\/\//.test(path)) return path
  return baseUrl.replace(/\/+$/, '') + (path.startsWith('/') ? path : `/${path}`)
}

/**
 * Počítadlo skutečně zhlédnutých sekund: sčítá jen plynulé posuny polohy,
 * přetočení se nepočítá. `take` vrátí celé sekundy a zbytek si nechá.
 */
export class WatchMeter {
  private last: number | null = null
  private acc = 0

  /** Nová poloha přehrávání; `playing` false = pauza (nepočítá se). */
  tick(position: number, playing: boolean): void {
    if (this.last !== null && playing) {
      const delta = position - this.last
      if (delta > 0 && delta < MAX_TIME_GAP) this.acc += delta
    }
    this.last = position
  }

  /** Přetočení – další posun se nezapočítá. */
  reset(): void {
    this.last = null
  }

  take(): number {
    const secs = Math.floor(this.acc)
    this.acc -= secs
    return secs
  }
}

/**
 * Adresa playlistu varianty z master playlistu (první URI), vyřešená vůči
 * adrese masteru. Server posílá jedinou variantu.
 */
export function variantUrlFromMaster(masterText: string, masterUrl: string): string | undefined {
  const line = masterText.split('\n').map((l) => l.trim()).find((l) => l && !l.startsWith('#'))
  if (!line) return undefined
  if (/^https?:\/\//.test(line)) return line
  const origin = /^https?:\/\/[^/]+/.exec(masterUrl)?.[0] ?? ''
  if (line.startsWith('/')) return origin + line
  return masterUrl.replace(/[^/?]*(\?.*)?$/, '') + line
}

/**
 * Kolik sekund playlist varianty pokrývá a jestli je kompletní. Přebalení
 * píše playlist průběžně – skočit jde jen do už hotové části.
 */
export function hlsPlaylistInfo(text: string): { seconds: number; ended: boolean } {
  let seconds = 0
  for (const m of text.matchAll(/#EXTINF:([\d.]+)/g)) seconds += Number(m[1])
  return { seconds, ended: text.includes('#EXT-X-ENDLIST') }
}
