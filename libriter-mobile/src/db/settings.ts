import * as Crypto from 'expo-crypto'

import { openDb } from './schema'

/** Klíče v tabulce `settings`. */
export type SettingKey = 'device_id' | 'server_url' | 'wifi_only' | 'last_library_sync'

export async function getSetting(key: SettingKey): Promise<string | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ value: string }>(
    'SELECT value FROM settings WHERE key = ?',
    key,
  )
  return row?.value ?? null
}

export async function setSetting(key: SettingKey, value: string): Promise<void> {
  const db = await openDb()
  await db.runAsync(
    `INSERT INTO settings (key, value) VALUES (?, ?)
     ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
    key,
    value,
  )
}

/**
 * ID zařízení pro synchronizaci. Vzniká při prvním spuštění a přežije
 * odhlášení – server podle něj jen pozná, odkud dávka přišla, nic víc.
 */
export async function deviceId(): Promise<string> {
  const stored = await getSetting('device_id')
  if (stored) return stored

  const fresh = Crypto.randomUUID()
  await setSetting('device_id', fresh)
  return fresh
}

/** Stahovat jen na Wi-Fi? Výchozí ano – mobilní data bývají drahá. */
export async function wifiOnly(): Promise<boolean> {
  return (await getSetting('wifi_only')) !== 'false'
}

export async function setWifiOnly(value: boolean): Promise<void> {
  await setSetting('wifi_only', value ? 'true' : 'false')
}
