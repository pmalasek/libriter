import { File, Paths } from 'expo-file-system'
import type { Home } from 'sceniter-shared'

// Poslední domovská obrazovka na disku: po startu je hned co ukázat a čerstvá
// data ze serveru ji jen tiše nahradí.
const file = () => new File(Paths.cache, 'home.json')

export function readHomeCache(): Home | undefined {
  try {
    const f = file()
    if (!f.exists) return undefined
    return JSON.parse(f.textSync()) as Home
  } catch {
    return undefined
  }
}

export function writeHomeCache(home: Home): void {
  try {
    const f = file()
    if (!f.exists) f.create()
    f.write(JSON.stringify(home))
  } catch {
    // Cache je jen zrychlení.
  }
}

export function clearHomeCache(): void {
  try {
    const f = file()
    if (f.exists) f.delete()
  } catch {
    // nic
  }
}
