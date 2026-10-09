// Kontrola úplnosti překladů proti angličtině (zdroj pravdy). Používají ji
// skripty i18n:check jádra i produktů.

const PLURAL = /_(zero|one|two|few|many|other)$/

/** Klíče katalogu; plurálové tvary se sloučí do základního klíče. */
function catalogKeys(catalog: unknown, prefix = ''): Set<string> {
  const out = new Set<string>()
  if (!catalog || typeof catalog !== 'object') return out
  for (const [key, value] of Object.entries(catalog)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') out.add(path.replace(PLURAL, ''))
    else for (const nested of catalogKeys(value, path)) out.add(nested)
  }
  return out
}

/**
 * Porovná katalogy jazyků s angličtinou a vypíše souhrn. Vrací false, když
 * některému jazyku chybí klíč – skript pak skončí chybou.
 */
export function checkCatalogs(
  name: string,
  catalogs: { en: object } & Record<string, object>,
  options: { list?: boolean } = {},
): boolean {
  const reference = catalogKeys(catalogs.en)
  let ok = true
  for (const [lang, catalog] of Object.entries(catalogs)) {
    if (lang === 'en') continue
    const present = catalogKeys(catalog)
    const missing = [...reference].filter((key) => !present.has(key))
    const extra = [...present].filter((key) => !reference.has(key))
    console.log(`${name} ${lang}: ${reference.size - missing.length}/${reference.size} přeloženo, chybí ${missing.length}, navíc ${extra.length}`)
    for (const key of missing) console.log(`  - ${key}`)
    if (options.list) for (const key of extra) console.log(`  + ${key} (není v angličtině)`)
    if (missing.length > 0) ok = false
  }
  return ok
}
