/**
 * Převod OKLCH → sRGB. Web má paletu zapsanou v `oklch()` přímo v CSS
 * (libriter-frontend/src/index.css); React Native tenhle zápis nezná, takže
 * se stejná čísla přepočítají tady. Postup je standardní: OKLCH → Oklab →
 * lineární sRGB → gamma; hodnoty mimo gamut se oříznou.
 */

function gamma(value: number): number {
  const clamped = Math.min(1, Math.max(0, value))
  return clamped <= 0.0031308 ? 12.92 * clamped : 1.055 * clamped ** (1 / 2.4) - 0.055
}

function channel(value: number): number {
  return Math.round(gamma(value) * 255)
}

/** RGB složky 0–255 pro OKLCH (L 0–1, C, H ve stupních). */
export function oklchToRgb(l: number, c: number, h: number): [number, number, number] {
  const rad = (h * Math.PI) / 180
  const a = c * Math.cos(rad)
  const b = c * Math.sin(rad)

  const l_ = l + 0.3963377774 * a + 0.2158037573 * b
  const m_ = l - 0.1055613458 * a - 0.0638541728 * b
  const s_ = l - 0.0894841775 * a - 1.291485548 * b

  const lc = l_ ** 3
  const mc = m_ ** 3
  const sc = s_ ** 3

  return [
    channel(4.0767416621 * lc - 3.3077115913 * mc + 0.2309699292 * sc),
    channel(-1.2684380046 * lc + 2.6097574011 * mc - 0.3413193965 * sc),
    channel(-0.0041960863 * lc - 0.7034186147 * mc + 1.707614701 * sc),
  ]
}

/** `oklch(l, c, h)` → `#rrggbb`. */
export function oklch(l: number, c: number, h: number): string {
  const [r, g, b] = oklchToRgb(l, c, h)
  return `#${[r, g, b].map((v) => v.toString(16).padStart(2, '0')).join('')}`
}

/** `oklch(l, c, h / alpha)` → `rgba(...)` – pro sklo a hrany s průhledností. */
export function oklcha(l: number, c: number, h: number, alpha: number): string {
  const [r, g, b] = oklchToRgb(l, c, h)
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}
