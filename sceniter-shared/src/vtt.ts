// Minimální parser WebVTT pro klienty, které titulky vykreslují samy (TV).
// Server posílá titulky už převedené do WebVTT (ffmpeg), takže stačí
// časování a text; styly, pozice a regiony se ignorují.

export interface Cue {
  start: number
  end: number
  /** Řádky textu bez značek (<i>, <c.xxx>, <v Jméno>…). */
  lines: string[]
}

const TIME = /(?:(\d+):)?(\d{2}):(\d{2})[.,](\d{3})/

function parseTime(s: string): number | undefined {
  const m = TIME.exec(s)
  if (!m) return undefined
  return Number(m[1] ?? 0) * 3600 + Number(m[2]) * 60 + Number(m[3]) + Number(m[4]) / 1000
}

const ENTITIES: Record<string, string> = { '&amp;': '&', '&lt;': '<', '&gt;': '>', '&nbsp;': ' ', '&lrm;': '', '&rlm;': '' }

function cleanLine(line: string): string {
  return line.replace(/<[^>]*>/g, '').replace(/&(amp|lt|gt|nbsp|lrm|rlm);/g, (e) => ENTITIES[e] ?? e)
}

/** Rozparsuje WebVTT na seřazené titulky. Poškozené bloky přeskočí. */
export function parseVtt(text: string): Cue[] {
  const cues: Cue[] = []
  const blocks = text.replace(/\r\n?/g, '\n').split(/\n{2,}/)
  for (const block of blocks) {
    const lines = block.split('\n')
    const i = lines.findIndex((l) => l.includes('-->'))
    if (i < 0) continue
    const [a, b] = lines[i].split('-->')
    const start = parseTime(a)
    const end = parseTime(b)
    if (start === undefined || end === undefined || end <= start) continue
    const body = lines.slice(i + 1).map(cleanLine).filter((l) => l.trim() !== '')
    if (body.length > 0) cues.push({ start, end, lines: body })
  }
  cues.sort((x, y) => x.start - y.start)
  return cues
}

/**
 * Titulky aktivní v čase t. Seřazené pole se prohledává půlením, takže to
 * jde volat při každém hlášení polohy i u dlouhých filmů.
 */
export function activeCues(cues: Cue[], t: number): Cue[] {
  // první titulek, který začíná po t
  let lo = 0
  let hi = cues.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (cues[mid].start <= t) lo = mid + 1
    else hi = mid
  }
  const out: Cue[] = []
  // Překrývající se titulky jsou vzácné a krátké – stačí projít pár předchozích.
  for (let j = lo - 1; j >= 0 && j >= lo - 8; j--) {
    if (cues[j].end > t) out.unshift(cues[j])
  }
  return out
}
