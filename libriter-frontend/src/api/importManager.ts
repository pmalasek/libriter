// Nahrávání knih do importu. Žije mimo komponenty, aby běželo dál, i když
// admin odejde z karty importu – karta i globální notifikace se k němu jen
// připojují (useImportManager).

import { useSyncExternalStore } from 'react'
import { t } from 'libriter-shared'
import { toast } from 'sonner'
import { apiFetch, apiUpload } from './client'
import type { ImportSession, ImportUploadedFile } from './types'
import { formatBytes } from '@/lib/format'

/** Přípony, které server přijme; ostatní soubory ze složky se vůbec neposílají. */
const ACCEPTED = /\.(mp3|m4a|m4b|aac|ogg|opus|flac|wav|zip|jpe?g|png|webp|gif|html?|pls)$/i
const JUNK = /(^|\/)(__MACOSX|\._[^/]*|thumbs\.db|\.ds_store|desktop\.ini)(\/|$)/i

/** Jak často nejvýš překreslovat průběh (onprogress chodí po desítkách ms). */
const PROGRESS_EVERY_MS = 250

/** Soubor k nahrání i s cestou ve vybrané složce. */
export interface PendingFile {
  path: string
  file: File
}

export interface UploadProgress {
  done: number
  total: number
  loaded: number
  size: number
}

export interface ImportManagerState {
  /** Import, který karta a notifikace sledují. */
  sessionId: string | null
  /** Průběh nahrávání; null = zrovna se nenahrává. */
  upload: UploadProgress | null
}

let state: ImportManagerState = { sessionId: null, upload: null }
const listeners = new Set<() => void>()
/** Importy zavřené uživatelem – zastaralé seznamy je nesmí znovu otevřít. */
const closed = new Set<string>()
let abort: AbortController | null = null

function set(patch: Partial<ImportManagerState>) {
  state = { ...state, ...patch }
  for (const listener of listeners) listener()
}

/** Zavření záložky nahrávání zabije – prohlížeč se na to zeptá. */
function warnOnUnload(event: BeforeUnloadEvent) {
  event.preventDefault()
  event.returnValue = ''
}

/** Cesta ve tvaru, v jakém ji ukládá server (viz importer.CleanRelPath). */
function normalizePath(path: string): string {
  return path
    .replace(/\\/g, '/')
    .normalize('NFC')
    .split('/')
    .map((part) => part.trim())
    .filter((part) => part && part !== '.')
    .join('/')
}

export const importManager = {
  subscribe(listener: () => void) {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },

  getSnapshot(): ImportManagerState {
    return state
  },

  /** Začne sledovat import (po načtení stránky navázání na rozpracovaný). */
  track(id: string) {
    if (closed.has(id) || state.sessionId === id) return
    set({ sessionId: id })
  },

  isClosed(id: string): boolean {
    return closed.has(id)
  },

  /** Přestane import sledovat; smazání na serveru řeší volající. */
  close(id: string) {
    closed.add(id)
    if (state.sessionId === id) set({ sessionId: null })
  },

  /** Přeruší běžící nahrávání; import se smaže. */
  cancel() {
    abort?.abort()
  },

  /**
   * Nahraje soubory a spustí rozpoznání knih. S `resumeId` pokračuje
   * v přerušeném importu a přeskočí soubory, které už server má.
   */
  async start(files: PendingFile[], resumeId?: string) {
    if (state.upload) return

    let accepted = files
      .map((f) => ({ ...f, path: normalizePath(f.path) }))
      .filter((f) => f.path && ACCEPTED.test(f.path) && !JUNK.test(f.path))
    if (accepted.length === 0) {
      toast.error(t('admin.import.noFiles'))
      return
    }

    const controller = new AbortController()
    abort = controller
    let sessionId = resumeId ?? null
    let uploaded = 0

    try {
      if (resumeId) {
        const present = await apiFetch<ImportUploadedFile[]>(`/admin/import/${resumeId}/files`)
        const sizes = new Map(present.map((f) => [f.path, f.size]))
        accepted = accepted.filter((f) => sizes.get(f.path) !== f.file.size)
      } else {
        const created = await apiFetch<ImportSession>('/admin/import', { method: 'POST' })
        sessionId = created.id
        const size = accepted.reduce((sum, f) => sum + f.file.size, 0)
        if (size > created.max_bytes) {
          toast.error(t('admin.import.tooLarge', { size: formatBytes(created.max_bytes) }))
          await apiFetch<void>(`/admin/import/${created.id}`, { method: 'DELETE' })
          return
        }
      }

      window.addEventListener('beforeunload', warnOnUnload)
      set({ sessionId })

      const size = accepted.reduce((sum, f) => sum + f.file.size, 0)
      const total = accepted.length
      let loadedBefore = 0
      let lastPaint = 0
      set({ upload: { done: 0, total, loaded: 0, size } })

      for (const [index, item] of accepted.entries()) {
        try {
          await apiUpload(
            `/admin/import/${sessionId}/files?path=${encodeURIComponent(item.path)}`,
            item.file,
            {
              signal: controller.signal,
              onProgress: (loaded) => {
                const now = Date.now()
                if (now - lastPaint < PROGRESS_EVERY_MS) return
                lastPaint = now
                set({ upload: { done: index, total, loaded: loadedBefore + loaded, size } })
              },
            },
          )
        } catch (error) {
          if (controller.signal.aborted) throw error
          throw new Error(
            t('admin.import.uploadFailed', {
              file: item.path,
              error: error instanceof Error ? error.message : String(error),
            }),
          )
        }
        loadedBefore += item.file.size
        uploaded += 1
        set({ upload: { done: index + 1, total, loaded: loadedBefore, size } })
      }

      await apiFetch<ImportSession>(`/admin/import/${sessionId}/analyze`, { method: 'POST' })
    } catch (error) {
      if (controller.signal.aborted) {
        if (sessionId) {
          closed.add(sessionId)
          void apiFetch<void>(`/admin/import/${sessionId}`, { method: 'DELETE' }).catch(() => {})
        }
        set({ sessionId: null })
        toast.info(t('admin.import.cancelled'))
      } else {
        // Nový import bez jediného souboru nemá cenu držet; rozpracovaný
        // (pokračování) zůstane, aby šel dokončit znovu.
        if (sessionId && !resumeId && uploaded === 0) {
          closed.add(sessionId)
          void apiFetch<void>(`/admin/import/${sessionId}`, { method: 'DELETE' }).catch(() => {})
          set({ sessionId: null })
        }
        toast.error(error instanceof Error ? error.message : String(error))
      }
    } finally {
      window.removeEventListener('beforeunload', warnOnUnload)
      abort = null
      set({ upload: null })
    }
  },
}

export function useImportManager(): ImportManagerState {
  return useSyncExternalStore(importManager.subscribe, importManager.getSnapshot)
}
