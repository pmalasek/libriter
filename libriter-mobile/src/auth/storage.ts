import * as SecureStore from 'expo-secure-store'
import { parseSession, serializeSession, type Session } from 'libriter-shared'

// Token je klíč k celé knihovně, proto SecureStore (Keychain / Keystore),
// ne AsyncStorage. Adresa serveru i ID zařízení jsou naopak obyčejná
// nastavení – leží v SQLite (viz src/db), aby šla číst i bez odemčení.
const SESSION_KEY = 'libriter.session'

export async function loadSession(): Promise<Session | null> {
  try {
    return parseSession(await SecureStore.getItemAsync(SESSION_KEY))
  } catch {
    // Poškozený nebo nečitelný záznam znamená přihlásit se znovu.
    return null
  }
}

export async function saveSession(session: Session): Promise<void> {
  await SecureStore.setItemAsync(SESSION_KEY, serializeSession(session))
}

export async function clearSession(): Promise<void> {
  try {
    await SecureStore.deleteItemAsync(SESSION_KEY)
  } catch {
    // Chybějící záznam není chyba – výsledek je stejný.
  }
}
