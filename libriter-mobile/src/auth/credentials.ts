import * as SecureStore from 'expo-secure-store'
import { t } from 'libriter-shared'

import { getSetting, setSetting } from '@/db/settings'

// Login a heslo pro předvyplnění formuláře po odhlášení. Leží v Keychainu /
// Keystore a čtou se jen po ověření biometrikou (Face ID / otisk). Záznam
// neopustí zařízení: THIS_DEVICE_ONLY vylučuje zálohu do iCloudu i přenos
// na nový telefon. Položky s requireAuthentication musí mít vlastní
// keychainService, jinak se na iOS perou se session tokenem (storage.ts).
const CREDENTIALS_KEY = 'libriter.credentials'

export interface Credentials {
  login: string
  password: string
}

function options(): SecureStore.SecureStoreOptions {
  return {
    keychainService: CREDENTIALS_KEY,
    keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
    requireAuthentication: true,
    authenticationPrompt: t('mobile.login.biometricPrompt'),
  }
}

/** Bez zapsané biometrie nejde údaje uložit s ověřením – volbu pak nenabízet. */
export function biometricsAvailable(): boolean {
  try {
    return SecureStore.canUseBiometricAuthentication()
  } catch {
    return false
  }
}

/**
 * Nešifrovaný příznak v SQLite, že v úložišti něco je. Díky němu se
 * biometrický dotaz nevyvolá naprázdno; samotné údaje v něm nejsou.
 */
export async function hasSavedCredentials(): Promise<boolean> {
  return (await getSetting('saved_credentials')) === '1'
}

export async function saveCredentials(credentials: Credentials): Promise<void> {
  await SecureStore.setItemAsync(CREDENTIALS_KEY, JSON.stringify(credentials), options())
  await setSetting('saved_credentials', '1')
}

/** null = zrušené ověření, zneplatněný klíč (změna biometrie) nebo nic uloženého. */
export async function loadCredentials(): Promise<Credentials | null> {
  try {
    const raw = await SecureStore.getItemAsync(CREDENTIALS_KEY, options())
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (
      typeof parsed === 'object' && parsed !== null &&
      typeof (parsed as Credentials).login === 'string' &&
      typeof (parsed as Credentials).password === 'string'
    ) {
      const { login, password } = parsed as Credentials
      return { login, password }
    }
    return null
  } catch {
    return null
  }
}

export async function clearCredentials(): Promise<void> {
  await setSetting('saved_credentials', '')
  try {
    await SecureStore.deleteItemAsync(CREDENTIALS_KEY, { keychainService: CREDENTIALS_KEY })
  } catch {
    // Chybějící záznam není chyba – výsledek je stejný.
  }
}
