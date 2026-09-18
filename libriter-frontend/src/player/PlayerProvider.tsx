import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { API_PREFIX, ApiError, apiFetch } from '@/api/client'
import { chaptersQuery, queryKeys, useBooks } from '@/api/hooks'
import type {
  BookProgress,
  Chapter,
  CreateSessionRequest,
  PlaySession,
  SessionItemsRequest,
  SessionPositionRequest,
  StreamToken,
} from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { coverUrl } from '@/components/BookCover'
import { authorsLabel } from '@/lib/format'
import {
  currentBookId,
  MAX_TIMEUPDATE_GAP_SECONDS,
  PlayerContext,
  REMOTE_SYNC_INTERVAL_MS,
  SAVE_INTERVAL_MS,
  sessionItem,
  STORAGE_KEY,
  VOLUME_KEY,
  type PlayerValue,
} from './playerContext'

/**
 * Uložená hlasitost. Bez záznamu, s poškozenou hodnotou i mimo rozsah hraje
 * přehrávač naplno – zticha startovat nemá.
 *
 * Prázdná hodnota se musí odchytit dřív, než se převede na číslo: `Number(null)`
 * je nula a ta by prošla kontrolou rozsahu jako platné ztlumení.
 */
function readVolume(): number {
  try {
    const stored = localStorage.getItem(VOLUME_KEY)
    if (stored === null || stored.trim() === '') return 1

    const value = Number(stored)
    if (Number.isFinite(value) && value >= 0 && value <= 1) return value
  } catch {
    // Zakázané úložiště nesmí přehrávač shodit.
  }
  return 1
}

/**
 * Token pro adresu audio souboru. Platí 24 hodin, takže ho stačí načíst
 * jednou za relaci; delší poslech ho obnoví přes vyprázdnění cache.
 */
const streamTokenQuery = {
  queryKey: ['auth', 'stream-token'] as const,
  queryFn: () => apiFetch<StreamToken>('/auth/stream-token'),
  staleTime: 12 * 60 * 60 * 1000,
}

/** Rozdíl pozice, od kterého se přebírá stav z jiného zařízení. */
const REMOTE_DRIFT_SECONDS = 3

interface Track {
  bookId: string
  chapterId: string
}

/**
 * Přehrávač žije nad celou aplikací, aby poslech nepřerušila změna stránky.
 * Nepřihlášenému uživateli nemá co nabídnout, takže se vůbec nesestavuje;
 * klíč podle účtu zajistí, že po přepnutí účtu nezůstane cizí rozposlouchaný
 * poslech (stejně jako u ColorSchemeProvider).
 */
export function PlayerProvider({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated } = useAuth()
  // Uložená relace s prošlým tokenem má uživatele, ale ne přístup k API;
  // přehrávač by pak na přihlašovací stránce marně sahal na server.
  if (!user || !isAuthenticated) return <>{children}</>
  return <ActivePlayer key={user.id}>{children}</ActivePlayer>
}

function ActivePlayer({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient()
  const books = useBooks()

  const [session, setSession] = useState<PlaySession | null>(null)
  const [track, setTrack] = useState<Track | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [loading, setLoading] = useState(false)
  const [speed, setSpeedState] = useState(1)
  const [volume, setVolumeState] = useState(readVolume)
  // Stažení posuvníku na nulu je ztlumení, i když ho přežilo načtení stránky –
  // ikona pak sedí a jedno kliknutí zvuk vrátí.
  const [muted, setMuted] = useState(() => volume === 0)

  // Zvuk musí přežít překreslení, proto element i vše, co se čte v jeho
  // událostech, drží ref – z posluchače by uzávěr viděl starý stav.
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const sessionRef = useRef<PlaySession | null>(null)
  const trackRef = useRef<Track | null>(null)
  const chaptersRef = useRef<Chapter[]>([])
  const speedRef = useRef(1)
  const pendingSeekRef = useRef<number | null>(null)
  const lastSavedRef = useRef<string>('')
  const tokenRetryRef = useRef(false)
  // Odposlouchané sekundy od posledního odeslání a čas, proti kterému se
  // měří přírůstek. Drží se v refu, aby je nepřekreslovaly events.
  const listenedRef = useRef(0)
  const lastTimeRef = useRef(0)

  function audioElement() {
    if (!audioRef.current) {
      audioRef.current = new Audio()
      audioRef.current.preload = 'metadata'
    }
    return audioRef.current
  }

  const book = useMemo(() => {
    if (!track) return null
    return (books.data ?? []).find((b) => b.id === track.bookId) ?? null
  }, [books.data, track])

  const chapter = useMemo(() => {
    if (!track) return null
    return chapters.find((c) => c.id === track.chapterId) ?? null
  }, [chapters, track])

  // --- ukládání pozice ---

  /** Zapíše čerstvý stav poslechu do cache i do stavu, bez dotazu na server. */
  const cacheSession = useCallback(
    (updated: PlaySession) => {
      if (sessionRef.current?.id === updated.id) {
        sessionRef.current = updated
        setSession(updated)
      }
      queryClient.setQueryData(queryKeys.session(updated.id), updated)
      queryClient.setQueryData<PlaySession[]>(queryKeys.sessions, (old) => {
        if (!old) return old
        const without = old.filter((s) => s.id !== updated.id)
        return [updated, ...without]
      })
    },
    [queryClient],
  )

  /**
   * Uloží pozici na server. Volá se každých pár sekund poslechu a při každé
   * změně, aby se dalo pokračovat na jiném zařízení; tělo se skládá synchronně,
   * takže i odchod ze stránky odešle to, co v tu chvíli hrálo.
   */
  const savePosition = useCallback(
    (
      options: {
        finished?: boolean
        bookFinished?: boolean
        keepalive?: boolean
        force?: boolean
      } = {},
    ) => {
      const openSession = sessionRef.current
      const openTrack = trackRef.current
      const audio = audioRef.current
      if (!openSession || !openTrack || !audio) return

      // Mezi nastavením souboru a doskočením na uloženou pozici hlásí element
      // nulu. Zápis v tu chvíli by rozposlouchané místo přepsal začátkem.
      if (pendingSeekRef.current != null) return

      const listened = Math.round(listenedRef.current)
      const body: SessionPositionRequest = {
        book_id: openTrack.bookId,
        chapter_id: openTrack.chapterId,
        position_seconds: Math.max(0, Math.round(audio.currentTime)),
        playback_speed: speedRef.current,
        finished: options.finished,
        listened_seconds: listened,
        book_finished: options.bookFinished,
      }

      // Pauza a přepínání stránek umí zavolat uložení několikrát za sebou;
      // beze změny není co posílat. Otisk se počítá bez odposlouchaných
      // sekund – nenulový přírůstek do deníku se zahodit nesmí.
      const fingerprint = JSON.stringify({ ...body, listened_seconds: undefined })
      if (!options.force && listened === 0 && fingerprint === lastSavedRef.current) return
      lastSavedRef.current = fingerprint
      // Neúspěšný zápis přijde o nejvýš jeden interval poslechu; držet
      // sekundy do potvrzení by je při rychlém přepínání knih počítalo dvakrát.
      listenedRef.current = 0

      const sessionId = openSession.id
      apiFetch<PlaySession>(`/sessions/${sessionId}/position`, {
        method: 'PUT',
        json: body,
        keepalive: options.keepalive,
      })
        .then((updated) => {
          cacheSession(updated)
          // Stav knihy se mění jen na začátku a na konci, ne každých 10 s.
          const known = queryClient.getQueryData<BookProgress[]>(queryKeys.bookProgress)
          const tracked = known?.some((p) => p.book_id === body.book_id)
          if (options.bookFinished || options.finished || !tracked) {
            void queryClient.invalidateQueries({ queryKey: queryKeys.bookProgress })
          }
        })
        .catch((error: unknown) => {
          // Výpadek sítě nemá přerušit poslech; příští zápis to dožene.
          if (error instanceof ApiError && error.status >= 500) return
          if (error instanceof ApiError && error.status === 404) return
        })
    },
    [cacheSession, queryClient],
  )

  // --- načtení kapitoly do přehrávače ---

  const load = useCallback(
    async (input: { bookId: string; chapterId?: string; position: number; autoplay: boolean }) => {
      setLoading(true)
      try {
        const list = await queryClient.fetchQuery(chaptersQuery(input.bookId))
        chaptersRef.current = list
        setChapters(list)

        const next = list.find((c) => c.id === input.chapterId) ?? list[0]
        if (!next) {
          toast.error('Kniha nemá žádné kapitoly k přehrání.')
          return
        }

        const { token } = await queryClient.fetchQuery(streamTokenQuery)
        const audio = audioElement()
        const target: Track = { bookId: input.bookId, chapterId: next.id }
        trackRef.current = target
        setTrack(target)

        // Pozici nelze nastavit dřív, než prohlížeč zná délku souboru; podle
        // ní se také ořízne. Délka z databáze na to nestačí – u souboru, kterému
        // scanner délku nezjistil, je uložená jen zástupná jedna sekunda.
        pendingSeekRef.current = Math.max(0, input.position)
        setCurrentTime(pendingSeekRef.current)
        setDuration(next.duration_seconds)

        audio.src = `${API_PREFIX}/chapters/${next.id}/audio?t=${encodeURIComponent(token)}`
        audio.playbackRate = speedRef.current
        audio.load()

        if (input.autoplay) {
          try {
            await audio.play()
          } catch {
            // Prohlížeč umí přehrání odmítnout, dokud uživatel neklikne –
            // lišta pak zůstane připravená v pauze.
          }
        }
      } catch (error) {
        toast.error(error instanceof Error ? error.message : 'Kapitolu se nepodařilo načíst.')
      } finally {
        setLoading(false)
      }
    },
    [queryClient],
  )

  /** Otevře poslech v liště a začne u jeho rozposlouchaného místa. */
  const openSession = useCallback(
    (next: PlaySession, options: { bookId?: string; chapterId?: string; autoplay: boolean }) => {
      sessionRef.current = next
      setSession(next)
      cacheSession(next)
      try {
        localStorage.setItem(STORAGE_KEY, next.id)
      } catch {
        // Bez místní cache se poslech po reloadu neobnoví, jinak nevadí.
      }

      speedRef.current = next.playback_speed
      setSpeedState(next.playback_speed)

      const bookId = options.bookId ?? currentBookId(next)
      if (!bookId) {
        toast.error('Poslech nemá žádné knihy.')
        return
      }
      const item = sessionItem(next, bookId)
      // Kliknutí na konkrétní kapitolu znamená začít ji od začátku.
      const chapterId = options.chapterId ?? item?.chapter_id
      const position = options.chapterId ? 0 : (item?.position_seconds ?? 0)

      void load({ bookId, chapterId, position, autoplay: options.autoplay })
    },
    [cacheSession, load],
  )

  // --- akce uživatele ---

  const createSession = useMutation({
    mutationFn: (body: CreateSessionRequest) =>
      apiFetch<PlaySession>('/sessions', { method: 'POST', json: body }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
    },
    onError: (error: Error) => toast.error(`Poslech se nepodařilo spustit: ${error.message}`),
  })

  const start = useCallback(
    (body: CreateSessionRequest, options: { chapterId?: string; bookId?: string } = {}) => {
      // Rozehraný poslech se nesmí zapomenout jen proto, že uživatel klikl jinam.
      savePosition()
      createSession.mutate(body, {
        onSuccess: (next) => openSession(next, { ...options, autoplay: true }),
      })
    },
    [createSession, openSession, savePosition],
  )

  const playBook = useCallback(
    (bookId: string, chapterId?: string) => {
      const open = sessionRef.current
      // Kniha z právě otevřeného poslechu nemusí přes server – stačí přepnout.
      if (open && sessionItem(open, bookId)) {
        savePosition()
        openSession(open, { bookId, chapterId, autoplay: true })
        return
      }
      start({ kind: 'book', book_id: bookId }, { bookId, chapterId })
    },
    [openSession, savePosition, start],
  )

  const playSeries = useCallback(
    (seriesId: string) => start({ kind: 'series', series_id: seriesId }),
    [start],
  )

  const playList = useCallback(
    (input: { bookIds?: string[]; seriesIds?: string[]; title?: string }) =>
      start({
        kind: 'list',
        title: input.title,
        book_ids: input.bookIds,
        series_ids: input.seriesIds,
      }),
    [start],
  )

  const addItems = useMutation({
    mutationFn: ({ sessionId, body }: { sessionId: string; body: SessionItemsRequest }) =>
      apiFetch<PlaySession>(`/sessions/${sessionId}/items`, { method: 'POST', json: body }),
    onSuccess: (updated) => {
      cacheSession(updated)
      void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
      toast.success('Přidáno do poslechu.')
    },
    onError: (error: Error) => toast.error(`Do poslechu se nepodařilo přidat: ${error.message}`),
  })

  const addToSession = useCallback(
    (input: { bookIds?: string[]; seriesIds?: string[] }) => {
      const open = sessionRef.current
      if (!open) return
      addItems.mutate({
        sessionId: open.id,
        body: { book_ids: input.bookIds, series_ids: input.seriesIds },
      })
    },
    [addItems],
  )

  const switchSession = useCallback(
    (sessionId: string) => {
      savePosition()
      // Pozice mohla mezitím povyrůst na jiném zařízení, proto čerstvě ze serveru.
      apiFetch<PlaySession>(`/sessions/${sessionId}`)
        .then((next) => openSession(next, { autoplay: true }))
        .catch((error: Error) => toast.error(`Poslech se nepodařilo otevřít: ${error.message}`))
    },
    [openSession, savePosition],
  )

  const playItem = useCallback(
    (bookId: string) => {
      const open = sessionRef.current
      if (!open) return
      savePosition()
      openSession(open, { bookId, autoplay: true })
    },
    [openSession, savePosition],
  )

  const close = useCallback(() => {
    audioRef.current?.pause()
    savePosition()
    sessionRef.current = null
    trackRef.current = null
    setSession(null)
    setTrack(null)
    setChapters([])
    setPlaying(false)
    try {
      localStorage.removeItem(STORAGE_KEY)
    } catch {
      // Zavření lišty nesmí spadnout na zakázaném úložišti.
    }
  }, [savePosition])

  const removeSession = useCallback(
    (sessionId: string) => {
      if (sessionRef.current?.id === sessionId) close()
      apiFetch<void>(`/sessions/${sessionId}`, { method: 'DELETE' })
        .then(() => {
          queryClient.removeQueries({ queryKey: queryKeys.session(sessionId) })
          void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
        })
        .catch((error: Error) => toast.error(`Poslech se nepodařilo smazat: ${error.message}`))
    },
    [close, queryClient],
  )

  const toggle = useCallback(() => {
    const audio = audioRef.current
    if (!audio || !trackRef.current) return
    if (audio.paused) {
      void audio.play().catch(() => toast.error('Přehrávání se nepodařilo spustit.'))
    } else {
      audio.pause()
    }
  }, [])

  const seek = useCallback((seconds: number) => {
    const audio = audioRef.current
    if (!audio || !trackRef.current) return
    const limit = Number.isFinite(audio.duration) ? audio.duration : seconds
    audio.currentTime = Math.min(Math.max(0, seconds), limit)
    setCurrentTime(audio.currentTime)
  }, [])

  const skip = useCallback(
    (delta: number) => seek((audioRef.current?.currentTime ?? 0) + delta),
    [seek],
  )

  /**
   * Kam vede sousední krok: na další kapitolu knihy, za její hranicí na další
   * knihu poslechu, a null znamená konec. Konec poslechu se musí poznat i bez
   * přepnutí – podle toho se zapisuje příznak doposlechnuto.
   */
  const stepTarget = useCallback((delta: number) => {
    const openTrack = trackRef.current
    const open = sessionRef.current
    if (!openTrack || !open) return null

    const index = chaptersRef.current.findIndex((c) => c.id === openTrack.chapterId)
    const next = index >= 0 ? chaptersRef.current[index + delta] : undefined
    if (next) return { bookId: openTrack.bookId, chapterId: next.id, position: 0 }

    const items = open.items
    const itemIndex = items.findIndex((i) => i.book_id === openTrack.bookId)
    const nextItem = items[itemIndex + delta]
    if (!nextItem) return null

    return {
      bookId: nextItem.book_id,
      chapterId: nextItem.chapter_id,
      position: nextItem.position_seconds,
    }
  }, [])

  /** Přepne na sousední kapitolu; za poslední pokračuje další knihou poslechu. */
  const step = useCallback(
    (delta: number, autoplay = true) => {
      const target = stepTarget(delta)
      if (!target) return false

      savePosition()
      void load({ ...target, autoplay })
      return true
    },
    [load, savePosition, stepTarget],
  )

  const nextChapter = useCallback(() => void step(1), [step])
  const prevChapter = useCallback(() => void step(-1), [step])

  const setSpeed = useCallback(
    (value: number) => {
      speedRef.current = value
      setSpeedState(value)
      if (audioRef.current) audioRef.current.playbackRate = value
      savePosition({ force: true })
    },
    [savePosition],
  )

  const setVolume = useCallback((value: number) => {
    const next = Math.min(1, Math.max(0, value))
    setVolumeState(next)
    // Tažení posuvníku na nulu a zpátky je přirozenější než hledat tlačítko.
    setMuted(next === 0)
    try {
      localStorage.setItem(VOLUME_KEY, String(next))
    } catch {
      // Bez uložení se hlasitost příště vrátí na výchozí, jinak nevadí.
    }
  }, [])

  const toggleMute = useCallback(() => {
    setMuted((current) => {
      // Ztlumení z nuly nemá co vracet, tak zvedne zvuk na polovinu.
      if (current && volume === 0) setVolume(0.5)
      return !current
    })
  }, [setVolume, volume])

  // Hlasitost se nastavuje i po výměně souboru – nový zdroj ji nepřebírá sám.
  useEffect(() => {
    const audio = audioElement()
    audio.volume = volume
    audio.muted = muted
  }, [muted, track, volume])

  // --- události přehrávače ---

  useEffect(() => {
    const audio = audioElement()

    const onLoaded = () => {
      const known = Number.isFinite(audio.duration) ? audio.duration : 0
      if (known > 0) setDuration(known)

      const pending = pendingSeekRef.current
      if (pending != null) {
        pendingSeekRef.current = null
        // Uložená pozice za koncem souboru (přeuložená kapitola) by přehrávání
        // rovnou ukončila, proto se ořízne kousek před konec.
        const target = known > 0 ? Math.min(pending, Math.max(0, known - 1)) : pending
        if (target > 0) audio.currentTime = target
      }
      // Nový soubor začíná jinde než skončil předchozí; ten skok není poslech.
      lastTimeRef.current = audio.currentTime
      tokenRetryRef.current = false
    }
    const onTime = () => {
      const now = audio.currentTime
      const diff = now - lastTimeRef.current
      // Do deníku jde jen plynulý posun. Převíjení i výměna souboru udělají
      // skok, a ten se nepočítá.
      if (!audio.paused && diff > 0 && diff < MAX_TIMEUPDATE_GAP_SECONDS * audio.playbackRate) {
        listenedRef.current += diff
      }
      lastTimeRef.current = now
      setCurrentTime(now)
    }
    const onPlay = () => setPlaying(true)
    const onPause = () => {
      setPlaying(false)
      savePosition()
    }
    const onSeeked = () => {
      lastTimeRef.current = audio.currentTime
      savePosition()
    }
    const onEnded = () => {
      // Konec poslední kapitoly je koncem knihy a bez další knihy i koncem
      // celého poslechu. Oba příznaky odejdou jedním zápisem: dva souběžné
      // požadavky doběhnou v libovolném pořadí a „doposlechnuto“ si přepíšou.
      // Zapsat se musí dřív, než step přepne na další knihu – jinak by se
      // příznak svezl k nesprávné.
      const openTrack = trackRef.current
      const index = openTrack
        ? chaptersRef.current.findIndex((c) => c.id === openTrack.chapterId)
        : -1
      const lastChapter = index >= 0 && index === chaptersRef.current.length - 1
      const hasNext = stepTarget(1) !== null
      if (lastChapter || !hasNext) {
        savePosition({ bookFinished: lastChapter, finished: !hasNext, force: true })
      }

      step(1)
    }
    const onError = async () => {
      if (tokenRetryRef.current || !trackRef.current) return
      tokenRetryRef.current = true
      // Nejčastější příčina je vypršelý token v adrese – zkusíme nový.
      const position = audio.currentTime
      await queryClient.invalidateQueries({ queryKey: streamTokenQuery.queryKey })
      const openTrack = trackRef.current
      void load({
        bookId: openTrack.bookId,
        chapterId: openTrack.chapterId,
        position,
        autoplay: !audio.paused,
      })
    }

    audio.addEventListener('loadedmetadata', onLoaded)
    audio.addEventListener('durationchange', onLoaded)
    audio.addEventListener('timeupdate', onTime)
    audio.addEventListener('play', onPlay)
    audio.addEventListener('pause', onPause)
    audio.addEventListener('seeked', onSeeked)
    audio.addEventListener('ended', onEnded)
    audio.addEventListener('error', onError)
    return () => {
      audio.removeEventListener('loadedmetadata', onLoaded)
      audio.removeEventListener('durationchange', onLoaded)
      audio.removeEventListener('timeupdate', onTime)
      audio.removeEventListener('play', onPlay)
      audio.removeEventListener('pause', onPause)
      audio.removeEventListener('seeked', onSeeked)
      audio.removeEventListener('ended', onEnded)
      audio.removeEventListener('error', onError)
    }
  }, [load, queryClient, savePosition, step, stepTarget])

  // Pravidelný zápis během poslechu; v pauze není co ukládat.
  useEffect(() => {
    if (!playing) return
    const timer = setInterval(() => savePosition(), SAVE_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [playing, savePosition])

  /**
   * Doptá se serveru, kde poslech je, a srovná podle něj lištu. Volá se jen
   * tehdy, když tady nic nehraje – jinak by cizí zápis přebil vlastní pozici.
   */
  const syncFromServer = useCallback(() => {
    // Bez zvukového prvku se tady nehraje – to je důvod ptát se, ne mlčet.
    if (audioRef.current && !audioRef.current.paused) return
    const open = sessionRef.current
    if (!open) return
    apiFetch<PlaySession>(`/sessions/${open.id}`)
      .then((fresh) => {
        if (sessionRef.current?.id !== fresh.id) return
        if (fresh.updated_at <= open.updated_at) return
        // Mezitím se tady mohlo spustit přehrávání; pak má přednost.
        if (audioRef.current && !audioRef.current.paused) return
        cacheSession(fresh)

        const bookId = currentBookId(fresh)
        const item = sessionItem(fresh, bookId)
        const openTrack = trackRef.current
        if (!bookId || !item || !openTrack) return

        const movedElsewhere = bookId !== openTrack.bookId || item.chapter_id !== openTrack.chapterId
        if (movedElsewhere) {
          void load({
            bookId,
            chapterId: item.chapter_id,
            position: item.position_seconds,
            autoplay: false,
          })
          return
        }
        const audio = audioRef.current
        if (audio && Math.abs(audio.currentTime - item.position_seconds) > REMOTE_DRIFT_SECONDS) {
          audio.currentTime = item.position_seconds
          setCurrentTime(item.position_seconds)
        }
      })
      .catch(() => {
        // Nedostupný server nemá důvod rušit rozehraný poslech.
      })
  }, [cacheSession, load])

  // Odchod ze stránky: uložit poslední pozici tak, aby požadavek doběhl.
  useEffect(() => {
    const onHide = () => savePosition({ keepalive: true })
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        savePosition({ keepalive: true })
        return
      }
      // Návrat k záložce: mezitím se mohlo poslouchat jinde.
      syncFromServer()
    }

    window.addEventListener('pagehide', onHide)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      window.removeEventListener('pagehide', onHide)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [savePosition, syncFromServer])

  // Poslech běžící na jiném zařízení: dokud je tahle karta vidět a mlčí,
  // ptá se občas serveru, ať čas a kapitola na druhé obrazovce nezamrznou.
  useEffect(() => {
    if (!session || playing) return
    const timer = setInterval(() => {
      if (document.visibilityState !== 'visible') return
      syncFromServer()
    }, REMOTE_SYNC_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [session, playing, syncFromServer])

  // Obnovení po načtení stránky: poslech se otevře v pauze tam, kde skončil.
  useEffect(() => {
    let stored: string | null = null
    try {
      stored = localStorage.getItem(STORAGE_KEY)
    } catch {
      return
    }
    if (!stored) return

    let cancelled = false
    apiFetch<PlaySession>(`/sessions/${stored}`)
      .then((restored) => {
        if (!cancelled) openSession(restored, { autoplay: false })
      })
      .catch((error: unknown) => {
        // Smazaný poslech nemá smysl zkoušet znovu.
        if (error instanceof ApiError && error.status === 404) {
          try {
            localStorage.removeItem(STORAGE_KEY)
          } catch {
            // nevadí
          }
        }
      })
    return () => {
      cancelled = true
    }
    // Záměrně jen při prvním sestavení – dál poslech řídí uživatel.
  }, [openSession])

  // Ovládání ze sluchátek, zamčené obrazovky a lišty systému.
  useEffect(() => {
    if (!('mediaSession' in navigator)) return
    const media = navigator.mediaSession

    if (book && chapter) {
      media.metadata = new MediaMetadata({
        title: chapter.title,
        artist: authorsLabel(book.authors),
        album: book.title,
        artwork: book.cover_path ? [{ src: coverUrl(book), sizes: '512x512' }] : undefined,
      })
    } else {
      media.metadata = null
    }
    media.playbackState = playing ? 'playing' : 'paused'

    const handlers: [MediaSessionAction, MediaSessionActionHandler][] = [
      ['play', () => toggle()],
      ['pause', () => toggle()],
      ['seekbackward', () => skip(-15)],
      ['seekforward', () => skip(30)],
      ['previoustrack', () => prevChapter()],
      ['nexttrack', () => nextChapter()],
      ['seekto', (details) => details.seekTime != null && seek(details.seekTime)],
    ]
    for (const [action, handler] of handlers) {
      try {
        media.setActionHandler(action, handler)
      } catch {
        // Starší prohlížeč nemusí akci znát; zbytek ovládání funguje dál.
      }
    }
    return () => {
      for (const [action] of handlers) {
        try {
          media.setActionHandler(action, null)
        } catch {
          // nevadí
        }
      }
    }
  }, [book, chapter, nextChapter, playing, prevChapter, seek, skip, toggle])

  // Odchod z aplikace (odhlášení, přepnutí účtu) nesmí nechat hrát zvuk.
  useEffect(() => {
    const audio = audioElement()
    return () => {
      audio.pause()
      audio.removeAttribute('src')
      audio.load()
    }
  }, [])

  const value: PlayerValue = {
    session,
    book,
    chapter,
    chapters,
    currentTime,
    duration,
    playing,
    loading: loading || createSession.isPending,
    speed,
    volume,
    muted,
    playBook,
    playSeries,
    playList,
    addToSession,
    switchSession,
    removeSession,
    playItem,
    toggle,
    seek,
    skip,
    nextChapter,
    prevChapter,
    setSpeed,
    setVolume,
    toggleMute,
    close,
  }

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>
}
