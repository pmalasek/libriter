/**
 * Adresy, na kterých zkusit server zadaný ovladačem. Bez schématu se zkouší
 * nejdřív https://, pak http:// (server v domácí síti bez certifikátu).
 * Koncová lomítka i omylem vložená cesta k API se odstraní.
 */
export function serverCandidates(input: string): string[] {
  const raw = input.trim().replace(/\/+$/, '').replace(/\/api\/v1$/, '')
  if (!raw) return []
  if (/^https?:\/\//i.test(raw)) return [raw]
  return [`https://${raw}`, `http://${raw}`]
}

/** Adresa pro zobrazení – bez https:// (http:// zůstává, ať je vidět). */
export function displayServer(url: string): string {
  return url.replace(/^https:\/\//i, '')
}
