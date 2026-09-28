import * as BackgroundTask from 'expo-background-task'
import * as TaskManager from 'expo-task-manager'
import { setAuthToken, setBaseUrl } from 'libriter-shared'

import { loadSession } from '@/auth/storage'
import { getSetting } from '@/db/settings'
import { syncEngine } from './syncEngine'

/**
 * Odeslání fronty pozic, když aplikace spí.
 *
 * Běžící aplikace posílá sama (NetInfo, návrat do popředí). Tohle je pojistka
 * pro případ, kdy telefon chytí síť s aplikací uspanou v pauze: iOS ani
 * Android ji kvůli síti neprobudí, ale systém čas od času spustí tenhle úkol
 * (Android jen se sítí, nejdřív po 15 minutách; iOS podle svého uvážení).
 *
 * Modul se načítá z index.js ještě před routerem – úkol musí být definovaný
 * i při startu na pozadí, kdy žádná obrazovka ani React neběží.
 */
export const BACKGROUND_SYNC_TASK = 'libriter-background-sync'

TaskManager.defineTask(BACKGROUND_SYNC_TASK, async () => {
  try {
    // Při startu na pozadí nenastavil adresu ani token nikdo – AuthProvider
    // se nevykreslil. Načíst se to dá vždy, v běžící aplikaci jde o tytéž hodnoty.
    const [session, url] = await Promise.all([loadSession(), getSetting('server_url')])
    if (!session || !url) return BackgroundTask.BackgroundTaskResult.Success
    setBaseUrl(url)
    setAuthToken(session.token)

    await syncEngine.pushPending()
    return BackgroundTask.BackgroundTaskResult.Success
  } catch {
    return BackgroundTask.BackgroundTaskResult.Failed
  }
})

export async function registerBackgroundSync(): Promise<void> {
  if (await TaskManager.isTaskRegisteredAsync(BACKGROUND_SYNC_TASK)) return
  await BackgroundTask.registerTaskAsync(BACKGROUND_SYNC_TASK, { minimumInterval: 15 })
}

export async function unregisterBackgroundSync(): Promise<void> {
  if (!(await TaskManager.isTaskRegisteredAsync(BACKGROUND_SYNC_TASK))) return
  await BackgroundTask.unregisterTaskAsync(BACKGROUND_SYNC_TASK)
}
