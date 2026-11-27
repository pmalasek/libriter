// Adresa serveru zadaná v mobilní a televizní aplikaci.
//
// Pravidla (rozhodnutí z bezpečnostního auditu):
// - zadané schéma platí přesně tak, jak je (http:// klient přijme, ale ukáže
//   varování – spojení není šifrované),
// - bez schématu se u adres v místní síti zkusí https:// a pak http://
//   (domácí server často certifikát nemá),
// - u veřejných adres bez schématu jen https:// – útočník v cestě by jinak
//   mohl https zablokovat a klient by heslo či token poslal nešifrovaně.

const SCHEME = /^https?:\/\//i

/**
 * Ořízne mezery, koncová lomítka a omylem vloženou cestu k API (/api/v1).
 * Schéma ani zbytek adresy nemění.
 */
export function normalizeServerInput(input: string): string {
  const raw = input
    .trim()
    .replace(/\/+$/, '')
    .replace(/\/api\/v1$/i, '')
    .replace(/\/+$/, '')
  // Samotné schéma (nevyplněné „https://“) není adresa.
  return /^https?:$/i.test(raw) ? '' : raw
}

/** Má adresa výslovně uvedené schéma http:// nebo https://? */
export function hasServerScheme(input: string): boolean {
  return SCHEME.test(input.trim())
}

/** Je adresa nešifrovaná (http://)? */
export function isInsecure(url: string): boolean {
  return /^http:\/\//i.test(url.trim())
}

/** Adresa pro zobrazení – bez https:// (http:// zůstává, ať je vidět). */
export function displayServer(url: string): string {
  return url.replace(/^https:\/\//i, '')
}

/**
 * Hostitel z adresy: bez schématu, přihlašovacích údajů, portu a cesty;
 * malými písmeny, IPv6 bez hranatých závorek a bez zóny (%eth0).
 */
export function serverHost(input: string): string {
  let rest = input.trim().replace(SCHEME, '')
  rest = rest.split(/[/?#]/, 1)[0] ?? ''
  const at = rest.lastIndexOf('@')
  if (at >= 0) rest = rest.slice(at + 1)

  let host: string
  if (rest.startsWith('[')) {
    const end = rest.indexOf(']')
    host = end > 0 ? rest.slice(1, end) : rest.slice(1)
  } else if (rest.indexOf(':') !== rest.lastIndexOf(':')) {
    // Víc dvojteček bez závorek = holá IPv6 adresa (port u ní zapsat nejde).
    host = rest
  } else {
    host = rest.split(':', 1)[0] ?? ''
  }
  const zone = host.indexOf('%')
  if (zone >= 0) host = host.slice(0, zone)
  return host.toLowerCase().replace(/\.$/, '')
}

function parseIPv4(host: string): number[] | null {
  const m = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.exec(host)
  if (!m) return null
  const parts = m.slice(1).map(Number)
  return parts.every((n) => n <= 255) ? parts : null
}

function isPrivateIPv4([a, b]: number[]): boolean {
  return (
    a === 10 || // 10.0.0.0/8
    a === 127 || // 127.0.0.0/8 loopback
    (a === 172 && b >= 16 && b <= 31) || // 172.16.0.0/12
    (a === 192 && b === 168) || // 192.168.0.0/16
    (a === 169 && b === 254) || // 169.254.0.0/16 link-local
    (a === 100 && b >= 64 && b <= 127) // 100.64.0.0/10 CGNAT (Tailscale apod.)
  )
}

/** Rozepíše IPv6 adresu na 8 šestnáctibitových skupin; null = není IPv6. */
function parseIPv6(host: string): number[] | null {
  if (!host.includes(':')) return null
  let text = host
  // Vložená IPv4 na konci (::ffff:192.168.1.5).
  const tail: number[] = []
  const v4 = /^(.*:)(\d{1,3}(?:\.\d{1,3}){3})$/.exec(text)
  if (v4) {
    const parts = parseIPv4(v4[2])
    if (!parts) return null
    tail.push((parts[0] << 8) | parts[1], (parts[2] << 8) | parts[3])
    text = v4[1].endsWith('::') ? v4[1] : v4[1].slice(0, -1)
  }
  const halves = text.split('::')
  if (halves.length > 2) return null
  const group = (s: string) => (s === '' ? [] : s.split(':'))
  const head = group(halves[0])
  const rest = halves.length === 2 ? group(halves[1]) : []
  const hex = /^[0-9a-f]{1,4}$/
  if (![...head, ...rest].every((g) => hex.test(g))) return null
  const known = head.length + rest.length + tail.length
  if (halves.length === 1 ? known !== 8 : known > 7) return null
  const zeros = new Array<number>(8 - known).fill(0)
  return [...head.map((g) => parseInt(g, 16)), ...zeros, ...rest.map((g) => parseInt(g, 16)), ...tail]
}

function isLocalIPv6(g: number[]): boolean {
  const allZeroUntil = (n: number) => g.slice(0, n).every((x) => x === 0)
  if (allZeroUntil(7) && g[7] === 1) return true // ::1 loopback
  if ((g[0] & 0xfe00) === 0xfc00) return true // fc00::/7 ULA
  if ((g[0] & 0xffc0) === 0xfe80) return true // fe80::/10 link-local
  // IPv4 mapovaná do IPv6 (::ffff:a.b.c.d)
  if (allZeroUntil(5) && g[5] === 0xffff) {
    return isPrivateIPv4([g[6] >> 8, g[6] & 0xff, g[7] >> 8, g[7] & 0xff])
  }
  return false
}

const LOCAL_SUFFIXES = ['.local', '.lan', '.home.arpa', '.internal', '.localhost']

/**
 * Je adresa (hostitel, hostitel:port, [IPv6]:port nebo celá URL) v místní
 * síti? Soukromé a loopback rozsahy IPv4/IPv6, localhost, domácí přípony
 * (.local, .lan, .home.arpa, .internal) a jednoslovné názvy počítačů.
 */
export function isLocalHost(input: string): boolean {
  const host = serverHost(input)
  if (!host) return false
  const v4 = parseIPv4(host)
  if (v4) return isPrivateIPv4(v4)
  const v6 = parseIPv6(host)
  if (v6) return isLocalIPv6(v6)
  if (host.includes(':')) return false // rozbitá IPv6
  if (host === 'localhost') return true
  if (LOCAL_SUFFIXES.some((suffix) => host.endsWith(suffix))) return true
  // Jednoslovný název (nas, synology) se v internetu přeložit nedá.
  return !host.includes('.')
}

/**
 * Adresy, na kterých zkusit server zadaný uživatelem, v pořadí:
 * - se schématem jen ta jedna,
 * - místní adresa bez schématu nejdřív https://, pak http://,
 * - jiná adresa bez schématu jen https://.
 */
export function serverCandidates(input: string): string[] {
  let raw = normalizeServerInput(input)
  if (!raw) return []
  if (hasServerScheme(raw)) return [raw]
  // Holá IPv6 adresa potřebuje v URL hranaté závorky.
  if (!raw.startsWith('[') && /^[0-9a-f:.%]+$/i.test(raw) && parseIPv6(serverHost(raw))) raw = `[${raw}]`
  return isLocalHost(raw) ? [`https://${raw}`, `http://${raw}`] : [`https://${raw}`]
}
