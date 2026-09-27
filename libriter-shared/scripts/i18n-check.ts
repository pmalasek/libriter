// Přehled úplnosti překladů proti angličtině (zdroj pravdy).
// Všechny jazyky musí být úplné – chybějící klíč je chyba (i v typechecku).
//
//   npm run i18n:check -w libriter-shared            souhrn
//   npm run i18n:check -w libriter-shared -- --list  i se seznamem klíčů

import cs from '../src/i18n/locales/cs'
import de from '../src/i18n/locales/de'
import en from '../src/i18n/locales/en'
import es from '../src/i18n/locales/es'
import fr from '../src/i18n/locales/fr'

const PLURAL = /_(zero|one|two|few|many|other)$/

/** Klíče katalogu; plurálové tvary se sloučí do základního klíče. */
function keys(catalog: unknown, prefix = ''): Set<string> {
  const out = new Set<string>()
  if (!catalog || typeof catalog !== 'object') return out
  for (const [key, value] of Object.entries(catalog)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') out.add(path.replace(PLURAL, ''))
    else for (const nested of keys(value, path)) out.add(nested)
  }
  return out
}

const reference = keys(en)
const list = process.argv.includes('--list')
let failed = false

for (const [lang, catalog, required] of [
  ['cs', cs, true],
  ['fr', fr, true],
  ['de', de, true],
  ['es', es, true],
] as const) {
  const present = keys(catalog)
  const missing = [...reference].filter((key) => !present.has(key))
  const extra = [...present].filter((key) => !reference.has(key))
  const done = reference.size - missing.length
  console.log(`${lang}: ${done}/${reference.size} přeloženo, chybí ${missing.length}, navíc ${extra.length}`)
  if (list || required) for (const key of missing) console.log(`  - ${key}`)
  for (const key of extra) console.log(`  + ${key} (není v angličtině)`)
  if (required && missing.length > 0) failed = true
}

if (failed) process.exitCode = 1
