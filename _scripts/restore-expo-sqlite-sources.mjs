// Vrátí do expo-sqlite vendorované zdroje SQLite.
//
// Podspec ExpoSQLite si při `pod install` kopíruje `vendor/sqlite3/sqlite3.c`
// a `.h` do `node_modules/expo-sqlite/ios/` (CocoaPods neumí `source_files`
// mimo adresář podu). Jenže každý `npm install` i `npm ci` node_modules
// přeinstaluje a tím je smaže – a protože se `Podfile.lock` nemění,
// `expo run:ios` to nepozná a build spadne na 63 chybách
// „cannot find 'exsqlite3_open' in scope“.
//
// Skript se pouští z `postinstall` v kořeni, takže je strom po instalaci vždy
// v použitelném stavu. Když expo-sqlite chybí (samostatný checkout webu),
// tiše skončí.
import { copyFileSync, existsSync, mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const pkg = join(root, 'node_modules', 'expo-sqlite')
if (!existsSync(pkg)) process.exit(0)

const target = join(pkg, 'ios')
mkdirSync(target, { recursive: true })

for (const file of ['sqlite3.c', 'sqlite3.h']) {
  const from = join(pkg, 'vendor', 'sqlite3', file)
  const to = join(target, file)
  if (!existsSync(from) || existsSync(to)) continue
  copyFileSync(from, to)
  console.log(`expo-sqlite: obnoveno ios/${file}`)
}
