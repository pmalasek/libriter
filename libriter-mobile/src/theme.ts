/**
 * Barvy a rozměry aplikace. Mobil je tmavý napevno: poslouchá se večer,
 * v autě a v posteli, a světlé rozhraní tam spíš svítí do očí, než pomáhá.
 */
export const colors = {
  background: '#0F1417',
  surface: '#171E23',
  surfaceAlt: '#1F282E',
  border: '#2A353C',
  text: '#ECF2F5',
  textMuted: '#9AAAB3',
  accent: '#3FB8AF',
  accentText: '#06201E',
  danger: '#E5736A',
} as const

export const spacing = {
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
} as const

export const radius = {
  sm: 8,
  md: 12,
  lg: 20,
} as const

/** „1:23:45“ nebo „4:07“ – hodiny se ukazují, jen když nějaké jsou. */
export function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = total % 60

  const pad = (value: number) => String(value).padStart(2, '0')
  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(rest)}` : `${minutes}:${pad(rest)}`
}

/** Velikost souboru pro lidi: „1,2 GB“. */
export function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 MB'
  const mb = bytes / 1024 / 1024
  if (mb < 1024) return `${mb.toFixed(mb < 10 ? 1 : 0)} MB`
  return `${(mb / 1024).toFixed(1).replace('.', ',')} GB`
}
