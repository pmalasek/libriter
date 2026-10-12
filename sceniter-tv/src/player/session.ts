import {
  absoluteUrl,
  getBaseUrl,
  hlsPlaylistInfo,
  keepaliveUrl,
  masterStart,
  RESUME_AHEAD,
  stopUrlFromVariant,
  variantUrlFromMaster,
  withStart,
  KEEPALIVE_MS,
  type Playback,
} from 'sceniter-shared'

export class TranscodeBusyError extends Error {}

/**
 * Převod na serveru pro jedno rozhodnutí o přehrávání: udržuje ho naživu,
 * umí počkat, až převod dojede k místu pokračování, a na konci ho ukončí.
 * U přímého přehrávání nedělá nic.
 */
export class StreamSession {
  readonly url: string
  /**
   * Čas (s), od kterého převod běží: pozice 0 přehrávače odpovídá tomuto
   * místu ve videu. Nenulový jen u přebalení spuštěného od místa skoku.
   */
  start = 0
  private variant?: string
  private stopUrl?: string
  private keepalive?: ReturnType<typeof setInterval>
  private closed = false

  constructor(
    private readonly pb: Playback,
    /** Převod zmizel (úklid serveru, restart) – je potřeba nové rozhodnutí. */
    private readonly onGone: () => void,
    /** Místo (s), od kterého má přebalení začít; plný převod skáče sám. */
    from?: number,
  ) {
    const growing = pb.hls && pb.mode !== 'transcode'
    this.url = absoluteUrl(getBaseUrl(), growing && from ? withStart(pb.url, from) : pb.url)
  }

  /**
   * Připraví HLS: načte master playlist (tím server převod spustí nebo
   * znovu použije), zjistí playlist varianty a spustí keepalive.
   */
  async open(): Promise<void> {
    if (!this.pb.hls) return
    const res = await fetch(this.url)
    if (res.status === 503) throw new TranscodeBusyError()
    if (!res.ok) throw new Error(`master ${res.status}`)
    const master = await res.text()
    this.start = masterStart(master)
    this.variant = variantUrlFromMaster(master, this.url)
    this.stopUrl = stopUrlFromVariant(this.variant)

    const keep = absoluteUrl(getBaseUrl(), keepaliveUrl(this.pb.url))
    this.keepalive = setInterval(() => {
      void fetch(keep, { method: 'POST' })
        .then((r) => {
          if (r.status === 404 && !this.closed) this.onGone()
        })
        .catch(() => {
          // Výpadek sítě – zkusí se při dalším tiku.
        })
    }, KEEPALIVE_MS)
  }

  /**
   * Počká, až rostoucí playlist (přebalení, převod zvuku) pokryje místo
   * `at` (čas ve videu) s rezervou. Plný převod má playlist v celé délce hned.
   */
  async waitForPosition(at: number, isCancelled: () => boolean): Promise<void> {
    if (!this.pb.hls || !this.variant || this.pb.mode === 'transcode') return
    while (!isCancelled() && !this.closed) {
      try {
        const res = await fetch(this.variant)
        if (res.ok) {
          const info = hlsPlaylistInfo(await res.text())
          if (info.ended || this.start + info.seconds >= at + RESUME_AHEAD) return
        }
      } catch {
        // zkusit znovu
      }
      await new Promise((r) => setTimeout(r, 800))
    }
  }

  /** Ukončí převod hned, ne až po době nečinnosti. */
  close(): void {
    this.closed = true
    clearInterval(this.keepalive)
    if (this.stopUrl) void fetch(this.stopUrl, { method: 'POST' }).catch(() => undefined)
  }
}
