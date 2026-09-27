import { i18n, t } from './i18n'

export const API_PREFIX = '/api/v1'

/**
 * Adresa serveru bez koncového lomítka. Web ji nechává prázdnou – rozhraní
 * servíruje tentýž server, takže stačí relativní cesta. Mobil si ji nastaví
 * z přihlašovací obrazovky, protože svůj server zná až od uživatele.
 */
let baseUrl = ''

export function setBaseUrl(url: string) {
  baseUrl = url.replace(/\/+$/, '')
}

export function getBaseUrl(): string {
  return baseUrl
}

/** Úplná adresa endpointu – pro věci mimo apiFetch (audio, obálky). */
export function apiUrl(path: string): string {
  return baseUrl + API_PREFIX + path
}

/**
 * Chyba z API včetně HTTP statusu. `message` je zpráva v jazyce rozhraní –
 * přeložená podle stabilního `code` z backendu; když překlad chybí, zůstane
 * zpráva serveru (česky).
 */
export class ApiError extends Error {
  readonly status: number
  /** Stabilní kód chyby z backendu („user.not_found“); chybí u chyb sítě. */
  readonly code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/** Zpráva k chybě v jazyce rozhraní podle kódu; undefined, když kód neznáme. */
export function apiErrorText(code: string | undefined): string | undefined {
  if (!code || !/^[a-z_]+\.[a-z_]+$/.test(code)) return undefined
  const key = `errors.${code}`
  if (!i18n.exists(key)) return undefined
  return t(key as 'errors.common.internal')
}

type UnauthorizedHandler = () => void

let onUnauthorized: UnauthorizedHandler | null = null

/** Registruje reakci na vypršelý/neplatný token (nastavuje AuthProvider). */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null) {
  onUnauthorized = handler
}

let authToken: string | null = null

/**
 * Nastaví JWT posílaný v hlavičce Authorization (nastavuje AuthProvider).
 * Volá se synchronně během renderu – efekty potomků (react-query) běží dřív
 * než efekt rodiče, takže z useEffect by první dotazy odešly bez hlavičky.
 */
export function setAuthToken(token: string | null) {
  authToken = token
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  /** Tělo požadavku; serializuje se jako JSON. */
  json?: unknown
  /** Neposílat Authorization ani nespouštět odhlášení při 401 (login, registrace). */
  anonymous?: boolean
  signal?: AbortSignal
  /**
   * Nechá požadavek doběhnout i po zavření stránky. Používá přehrávač při
   * ukládání pozice na odchodu – sendBeacon by neuměl poslat token v hlavičce.
   */
  keepalive?: boolean
}

/**
 * Chyby z API jsou JSON {"error": "...", "code": "..."}. Tělo se čte jako
 * text a JSON se parsuje až dodatečně – před API může stát proxy, která
 * vrátí text nebo HTML.
 */
async function parseError(res: Response): Promise<ApiError> {
  let body = ''
  try {
    body = (await res.text()).trim()
  } catch {
    body = ''
  }

  let message = ''
  let code: string | undefined
  if (body) {
    try {
      const parsed: unknown = JSON.parse(body)
      if (parsed && typeof parsed === 'object') {
        const { error, code: c } = parsed as { error?: unknown; code?: unknown }
        if (typeof error === 'string') message = error
        if (typeof c === 'string' && c) code = c
      }
    } catch {
      // není JSON – použijeme surový text
      if (!body.startsWith('<')) message = body
    }
  }

  const text = apiErrorText(code) || message || t('errors.server', { status: res.status })
  return new ApiError(res.status, text, code)
}

export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', json, anonymous = false, signal, keepalive } = options

  const headers = new Headers({ Accept: 'application/json' })
  const token = anonymous ? null : authToken
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (json !== undefined) headers.set('Content-Type', 'application/json')

  let res: Response
  try {
    res = await fetch(apiUrl(path), {
      method,
      headers,
      body: json === undefined ? undefined : JSON.stringify(json),
      signal,
      keepalive,
    })
  } catch {
    throw new ApiError(0, t('errors.network'))
  }

  if (!res.ok) {
    // Odhlašujeme jen když jsme token skutečně poslali – 401 z loginu
    // znamená špatné heslo, ne vypršelou session.
    if (res.status === 401 && token) onUnauthorized?.()
    throw await parseError(res)
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

/** Seznamy z backendu mohou být JSON `null` (prázdný Go slice). */
export function asList<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}
