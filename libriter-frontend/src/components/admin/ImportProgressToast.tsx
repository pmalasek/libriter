import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { toast } from 'sonner'
import { adminKeys, useImportSession, useImportSessions } from '@/api/adminHooks'
import { queryKeys } from '@/api/hooks'
import { importManager, useImportManager } from '@/api/importManager'

const TOAST_ID = 'library-import'
const LIBRARY_PATH = '/admin/library'

/** Importy, na které se notifikace po načtení stránky sama napojí. */
const FOLLOW = new Set(['analyzing', 'ready', 'importing'])

/**
 * Průběh importu knih mimo záložku Knihovna: jedna notifikace, která se mění
 * podle fáze (nahrávání → rozpoznání → náhled → import → hotovo). Na stránce
 * Knihovna mizí – tam průběh ukazuje karta importu.
 */
export function ImportProgressToast() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const queryClient = useQueryClient()
  const { sessionId, upload } = useImportManager()
  const sessions = useImportSessions()
  const session = useImportSession(sessionId)
  const onLibrary = pathname.startsWith(LIBRARY_PATH)
  // Fáze („id:stav“), které už admin viděl – ty se znovu neoznamují.
  const seen = useRef(new Set<string>())

  // Po načtení stránky navázat na import, který na serveru ještě běží.
  useEffect(() => {
    if (sessionId !== null || upload !== null) return
    const open = sessions.data?.sessions.find(
      (s) => FOLLOW.has(s.state) && !importManager.isClosed(s.id),
    )
    if (open) importManager.track(open.id)
  }, [sessions.data, sessionId, upload])

  // Hotový import mění knihovnu – ať se seznamy načtou znovu, ať je admin kdekoli.
  const state = session.data?.state
  useEffect(() => {
    if (state !== 'done') return
    for (const key of [
      queryKeys.books,
      queryKeys.authors,
      queryKeys.series,
      adminKeys.stats,
      adminKeys.scanner,
    ]) {
      void queryClient.invalidateQueries({ queryKey: key })
    }
  }, [state, sessionId, queryClient])

  const data = session.data
  useEffect(() => {
    const phase = upload ? 'uploading' : data?.state
    const key = `${sessionId}:${phase}`

    if (onLibrary) {
      if (phase) seen.current.add(key)
      toast.dismiss(TOAST_ID)
      return
    }
    if (!sessionId || !phase) {
      toast.dismiss(TOAST_ID)
      return
    }

    const options = {
      id: TOAST_ID,
      duration: Infinity,
      action: { label: t('admin.import.toast.show'), onClick: () => navigate(LIBRARY_PATH) },
    }

    if (upload) {
      const percent = upload.size > 0 ? Math.floor((upload.loaded / upload.size) * 100) : 0
      toast.loading(
        t('admin.import.toast.uploading', { percent, done: upload.done, total: upload.total }),
        options,
      )
      return
    }
    if (!data) return

    switch (data.state) {
      case 'analyzing':
        toast.loading(t('admin.import.toast.analyzing', data.progress), options)
        return
      case 'importing':
        toast.loading(t('admin.import.toast.importing', data.progress), options)
        return
    }

    // Konečné fáze se oznámí jednou; zavřít je jde křížkem nebo přechodem na kartu.
    if (seen.current.has(key)) return
    seen.current.add(key)
    switch (data.state) {
      case 'ready':
        toast.info(t('admin.import.toast.ready'), options)
        return
      case 'done':
        toast.success(
          t('admin.import.toast.done', {
            ok: data.results.filter((r) => r.book_id).length,
            total: data.results.length,
          }),
          { ...options, duration: 10_000 },
        )
        return
      case 'failed':
        toast.error(t('admin.import.toast.failed'), options)
        return
      default:
        // Přerušené nahrávání – řeší ho karta, notifikace mlčí.
        toast.dismiss(TOAST_ID)
    }
  }, [onLibrary, sessionId, upload, data, t, navigate])

  return null
}
