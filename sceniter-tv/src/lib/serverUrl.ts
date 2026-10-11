/**
 * Normalizuje adresu serveru zadanou ovladačem: doplní https://, odstraní
 * koncová lomítka i omylem vloženou cestu k API.
 */
export function normalizeServerUrl(input: string): string {
  let url = input.trim()
  if (!url) return ''
  if (!/^https?:\/\//i.test(url)) url = `https://${url}`
  return url.replace(/\/+$/, '').replace(/\/api\/v1$/, '')
}
