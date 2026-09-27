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

interface UploadOptions {
  method?: 'PUT' | 'POST'
  /** Průběh odesílání v bajtech (fetch ho neumí hlásit, proto XHR). */
  onProgress?: (loaded: number, total: number) => void
  signal?: AbortSignal
}

/** Odešle binární tělo (soubor) s autorizací; odpověď je JSON. */
export function apiUpload<T>(path: string, body: Blob, options: UploadOptions = {}): Promise<T> {
  const { method = 'PUT', onProgress, signal } = options
  const token = authToken

  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open(method, apiUrl(path))
    xhr.setRequestHeader('Accept', 'application/json')
    xhr.setRequestHeader('Content-Type', 'application/octet-stream')
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)

    if (onProgress) {
      xhr.upload.onprogress = (e) => onProgress(e.loaded, e.lengthComputable ? e.total : body.size)
    }

    const onAbort = () => xhr.abort()
    signal?.addEventListener('abort', onAbort, { once: true })
    const done = () => signal?.removeEventListener('abort', onAbort)

    xhr.onload = () => {
      done()
      const res = new Response(xhr.responseText, { status: xhr.status })
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve((xhr.responseText ? JSON.parse(xhr.responseText) : undefined) as T)
        return
      }
      if (xhr.status === 401 && token) onUnauthorized?.()
      void parseError(res).then(reject)
    }
    xhr.onerror = () => {
      done()
      reject(new ApiError(0, t('errors.network')))
    }
    xhr.onabort = () => {
      done()
      reject(new DOMException('Aborted', 'AbortError'))
    }

    xhr.send(body)
  })
}

/** Stáhne binární odpověď s autorizací (obrázky, které <img> neumí poslat s tokenem). */
export async function apiBlob(path: string, signal?: AbortSignal): Promise<Blob> {
  const headers = new Headers()
  if (authToken) headers.set('Authorization', `Bearer ${authToken}`)
  let res: Response
  try {
    res = await fetch(apiUrl(path), { headers, signal })
  } catch {
    throw new ApiError(0, t('errors.network'))
  }
  if (!res.ok) throw await parseError(res)
  return res.blob()
}
