import * as SecureStore from 'expo-secure-store'
import { parseSession, serializeSession, type Session } from 'libriter-shared'

// Token je klíč k celé knihovně, proto SecureStore (Keychain / Keystore),
// ne AsyncStorage. Adresa serveru i ID zařízení jsou naopak obyčejná
// nastavení – leží v SQLite (viz src/db), aby šla číst i bez odemčení.
const SESSION_KEY = 'libriter.session'

// Synchronizace na pozadí (src/sync/backgroundSync.ts) běží i na zamčeném
// telefonu; s výchozím WHEN_UNLOCKED by iOS token nevydal.
const OPTIONS: SecureStore.SecureStoreOptions = { keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK }

export async function loadSession(): Promise<Session | null> {
  try {
    return parseSession(await SecureStore.getItemAsync(SESSION_KEY))
  } catch {
    // Poškozený nebo nečitelný záznam znamená přihlásit se znovu.
    return null
  }
}

export async function saveSession(session: Session): Promise<void> {
  await SecureStore.setItemAsync(SESSION_KEY, serializeSession(session), OPTIONS)
}

export async function clearSession(): Promise<void> {
  try {
    await SecureStore.deleteItemAsync(SESSION_KEY)
  } catch {
    // Chybějící záznam není chyba – výsledek je stejný.
  }
}

/**
 * Přepíše uloženou session s aktuálními OPTIONS. Starší instalace ji mají
 * uloženou jako WHEN_UNLOCKED a přepis existující položky v Keychainu mění
 * jen data, ne přístupnost – proto smazat a uložit znovu. Volá se v popředí
 * (telefon je odemčený), jednou za start aplikace.
 */
export async function refreshSessionStorage(session: Session): Promise<void> {
  try {
    await SecureStore.deleteItemAsync(SESSION_KEY)
    await saveSession(session)
  } catch {
    // Nevyšlo-li to, zkusit aspoň obnovit původní záznam.
    await saveSession(session).catch(() => undefined)
  }
}
