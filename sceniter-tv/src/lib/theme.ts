import * as SecureStore from 'expo-secure-store'
import { DEFAULT_COLOR_SCHEME, parseScheme, parseSession, type ColorScheme } from 'sceniter-shared'

// Téma je samostatný klíč, ne součást sceniter.prefs: čte se synchronně
// ještě před vykreslením (barvy jsou ve StyleSheet.create při načtení modulů).
const THEME_KEY = 'sceniter.theme'
const SESSION_KEY = 'sceniter.session'

/**
 * Téma pro start aplikace: volba z Nastavení televize, jinak téma
 * spárovaného účtu (z profilu při párování), jinak výchozí.
 */
export function readThemeSync(): ColorScheme {
  try {
    const stored = SecureStore.getItem(THEME_KEY)
    if (stored) return parseScheme(stored)
    const user = parseSession(SecureStore.getItem(SESSION_KEY))?.user
    return user ? parseScheme(user.color_scheme) : DEFAULT_COLOR_SCHEME
  } catch {
    return DEFAULT_COLOR_SCHEME
  }
}

/** Uloží téma; projeví se po novém načtení aplikace. */
export async function saveTheme(scheme: ColorScheme): Promise<void> {
  await SecureStore.setItemAsync(THEME_KEY, scheme)
}
