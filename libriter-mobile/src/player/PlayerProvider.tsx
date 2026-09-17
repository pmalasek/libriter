import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { AppState } from 'react-native'
import { useQueryClient } from '@tanstack/react-query'
import TrackPlayer, { Event, State, useTrackPlayerEvents, type Track } from 'react-native-track-player'
import {
  ApiError,
  apiUrl,
  currentBookId,
  MAX_TIMEUPDATE_GAP_SECONDS,
  queryKeys,
  SAVE_INTERVAL_MS,
  sessionItem,
  type Book,
  type Chapter,
  type PlaySession,
} from 'libriter-shared'

import { fetchStreamToken } from '@/api/stream'
import { toast } from '@/components/Toast'
import { withSource } from '@/data/sources'
import { bookChapterFiles } from '@/db/downloads'
import { getSessionMirror } from '@/db/library'
import { resetFingerprint, savePosition } from './positionSaver'
import { addSessionItems, deleteSession, startSession } from './sessions'
import { ensurePlayer } from './setup'

/** Výběr knih a sérií pro seznam nebo přidání do poslechu – jako na webu. */
interface Selection {
  bookIds?: string[]
  seriesIds?: string[]
}

interface PlayerValue {
  session: PlaySession | null
  book: Book | null
  chapter: Chapter | null
  chapters: Chapter[]
  position: number
  duration: number
  playing: boolean
  loading: boolean
  speed: number
  /** Hraje se ze staženého souboru, ne ze streamu? */
  offline: boolean

  /** Přehraje knihu; volitelně rovnou konkrétní kapitolu od začátku. */
  playBook: (bookId: string, chapterId?: string) => Promise<void>
  playSeries: (seriesId: string) => Promise<void>
  playList: (input: Selection & { title?: string }) => Promise<void>
  /** Přidá knihy nebo série na konec otevřeného poslechu. */
  addToSession: (input: Selection) => Promise<void>
  /** Přepne na jiný rozposlouchaný poslech a načte jeho pozici. */
  switchSession: (sessionId: string) => Promise<void>
  removeSession: (sessionId: string) => Promise<void>
  /** Přepne knihu uvnitř otevřeného poslechu. */
  playItem: (bookId: string) => Promise<void>

  toggle: () => Promise<void>
  seek: (seconds: number) => Promise<void>
  skip: (delta: number) => Promise<void>
  nextChapter: () => Promise<void>
  prevChapter: () => Promise<void>
  setSpeed: (speed: number) => Promise<void>
  /** Zavře přehrávač; poslech zůstane v seznamu i s pozicí. */
  close: () => Promise<void>
}

const PlayerContext = createContext<PlayerValue | null>(null)

export function usePlayer(): PlayerValue {
  const ctx = useContext(PlayerContext)
  if (!ctx) throw new Error('usePlayer musí být uvnitř PlayerProvider')
  return ctx
}

export function PlayerProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [session, setSession] = useState<PlaySession | null>(null)
  const [book, setBook] = useState<Book | null>(null)
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [chapter, setChapter] = useState<Chapter | null>(null)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [loading, setLoading] = useState(false)
  const [speed, setSpeedState] = useState(1)
  const [offline, setOffline] = useState(false)

  // Hodnoty, které potřebují posluchači přehrávače (běží mimo render).
  const sessionRef = useRef<PlaySession | null>(null)
  const bookRef = useRef<Book | null>(null)
  const chaptersRef = useRef<Chapter[]>([])
  const chapterRef = useRef<Chapter | null>(null)
  const speedRef = useRef(1)
  const playingRef = useRef(false)
  /** Odposlouchané sekundy od posledního zápisu; jdou do deníku poslechu. */
  const listenedRef = useRef(0)
  const lastPositionRef = useRef(0)
  /** Mezi nastavením fronty a doskočením na pozici se nesmí ukládat. */
  const seekingRef = useRef(false)

  sessionRef.current = session
  bookRef.current = book
  chaptersRef.current = chapters
  chapterRef.current = chapter

  const invalidateSessions = useCallback(
    () => void queryClient.invalidateQueries({ queryKey: queryKeys.sessions }),
    [queryClient],
  )

  const store = useCallback(
    async (options: { finished?: boolean; bookFinished?: boolean; force?: boolean } = {}) => {
      const openSession = sessionRef.current
      const openBook = bookRef.current
      if (!openSession || !openBook || seekingRef.current) return

      const listened = listenedRef.current
      listenedRef.current = 0

      const saved = await savePosition({
        sessionId: openSession.id,
        bookId: openBook.id,
        chapterId: chapterRef.current?.id,
        positionSeconds: lastPositionRef.current,
        playbackSpeed: speedRef.current,
        listenedSeconds: listened,
        finished: options.finished,
        bookFinished: options.bookFinished,
        force: options.force,
      })
      if (!saved) return

      const fresh = await getSessionMirror(openSession.id)
      if (fresh && sessionRef.current?.id === fresh.id) setSession(fresh)
    },
    [],
  )

  /**
   * Naplní frontu přehrávače kapitolami knihy a doskočí na uloženou pozici.
   *
   * Kniha a kapitoly jdou podle režimu ze serveru nebo z telefonu; zdroj
   * zvuku se volí podle toho, co je stažené: lokální soubor vyhrává nad
   * streamem, takže rozehraná kniha přežije i vypnutou síť.
   */
  const load = useCallback(
    async (input: {
      session: PlaySession
      bookId: string
      chapterId?: string
      positionSeconds: number
      autoplay: boolean
    }) => {
      setLoading(true)
      seekingRef.current = true
      try {
        await ensurePlayer()

        const [nextBook, nextChapters, files] = await Promise.all([
          withSource((s) => s.book(input.bookId)),
          withSource((s) => s.chapters(input.bookId)),
          bookChapterFiles(input.bookId),
        ])
        if (!nextBook || nextChapters.length === 0) {
          toast.error('Kniha nemá žádné kapitoly k přehrání.')
          return
        }

        // Token se vyžádá jen tehdy, když aspoň jedna kapitola chybí na disku.
        const needsStream = nextChapters.some((item) => !files.has(item.id))
        const token = needsStream ? await streamTokenOrNull() : null

        const tracks: Track[] = nextChapters.map((item) => {
          const file = files.get(item.id)
          return {
            id: item.id,
            url: file ? file.path : `${apiUrl(`/chapters/${item.id}/audio`)}?t=${token ?? ''}`,
            title: item.title,
            artist: nextBook.authors.map((author) => author.name).join(', ') || 'Libriter',
            album: nextBook.title,
            duration: item.duration_seconds,
            artwork: apiUrl(`/books/${nextBook.id}/cover`),
          }
        })

        const index = Math.max(
          0,
          nextChapters.findIndex((item) => item.id === input.chapterId),
        )

        await TrackPlayer.setQueue(tracks)
        await TrackPlayer.skip(index)
        // Pozice za koncem souboru (přeuložená kapitola) by přehrávání rovnou
        // ukončila, proto se ořízne kousek před konec.
        const limit = Math.max(0, nextChapters[index].duration_seconds - 1)
        const target = Math.min(Math.max(0, input.positionSeconds), limit || input.positionSeconds)
        if (target > 0) await TrackPlayer.seekTo(target)
        speedRef.current = input.session.playback_speed || speedRef.current
        setSpeedState(speedRef.current)
        await TrackPlayer.setRate(speedRef.current)

        setSession(input.session)
        setBook(nextBook)
        setChapters(nextChapters)
        setChapter(nextChapters[index])
        setPosition(target)
        setDuration(nextChapters[index].duration_seconds)
        setOffline(files.has(nextChapters[index].id))
        lastPositionRef.current = target
        listenedRef.current = 0
        resetFingerprint()

        if (input.autoplay) await TrackPlayer.play()
      } catch (error: unknown) {
        toast.error(error instanceof Error ? error.message : 'Přehrávání se nepodařilo spustit')
      } finally {
        seekingRef.current = false
        setLoading(false)
      }
    },
    [],
  )

  /** Otevře poslech na jeho rozehrané knize. */
  const openSession = useCallback(
    async (target: PlaySession, bookId?: string, chapterId?: string, fromStart = false) => {
      const id = bookId ?? currentBookId(target)
      if (!id) return
      const item = sessionItem(target, id)
      await load({
        session: target,
        bookId: id,
        chapterId: chapterId ?? item?.chapter_id,
        positionSeconds: fromStart ? 0 : (item?.position_seconds ?? 0),
        autoplay: true,
      })
    },
    [load],
  )

  const playBook = useCallback(
    async (bookId: string, chapterId?: string) => {
      try {
        // Otevřený poslech s touhle knihou se jen přepne – zakládat nový by
        // zahodil pozici, kterou přehrávač drží.
        const open = sessionRef.current
        if (open && open.items.some((item) => item.book_id === bookId)) {
          await store({ force: true })
          await openSession(open, bookId, chapterId, Boolean(chapterId))
          return
        }
        const target = await startSession({ kind: 'book', book_id: bookId })
        invalidateSessions()
        // Kliknutí na konkrétní kapitolu ji spustí od začátku; „Přehrát“
        // u knihy pokračuje tam, kde poslech skončil.
        await openSession(target, bookId, chapterId, Boolean(chapterId))
      } catch (error: unknown) {
        toast.error(describe(error, 'Poslech se nepodařilo založit'))
      }
    },
    [invalidateSessions, openSession, store],
  )

  const playSeries = useCallback(
    async (seriesId: string) => {
      try {
        const target = await startSession({ kind: 'series', series_id: seriesId })
        invalidateSessions()
        await openSession(target)
      } catch (error: unknown) {
        toast.error(describe(error, 'Poslech série se nepodařilo založit'))
      }
    },
    [invalidateSessions, openSession],
  )

  const playList = useCallback(
    async (input: Selection & { title?: string }) => {
      try {
        const target = await startSession({
          kind: 'list',
          title: input.title,
          book_ids: input.bookIds,
          series_ids: input.seriesIds,
        })
        invalidateSessions()
        await openSession(target)
      } catch (error: unknown) {
        toast.error(describe(error, 'Seznam se nepodařilo založit'))
      }
    },
    [invalidateSessions, openSession],
  )

  const addToSession = useCallback(
    async (input: Selection) => {
      const open = sessionRef.current
      if (!open) return
      try {
        const updated = await addSessionItems(open.id, {
          book_ids: input.bookIds,
          series_ids: input.seriesIds,
        })
        setSession(updated)
        invalidateSessions()
        toast.success('Přidáno do poslechu.')
      } catch (error: unknown) {
        toast.error(describe(error, 'Do poslechu se nepodařilo přidat'))
      }
    },
    [invalidateSessions],
  )

  const switchSession = useCallback(
    async (sessionId: string) => {
      try {
        await store({ force: true })
        // Ze serveru, pokud je po ruce – pozice z jiného zařízení je novější
        // než zrcadlo; bez sítě se vezme zrcadlo.
        const target = (await withSource((s) => s.session(sessionId))) ?? (await getSessionMirror(sessionId))
        if (!target) {
          toast.error('Poslech už neexistuje.')
          return
        }
        await openSession(target)
      } catch (error: unknown) {
        toast.error(describe(error, 'Poslech se nepodařilo otevřít'))
      }
    },
    [openSession, store],
  )

  const close = useCallback(async () => {
    await store({ force: true })
    await TrackPlayer.pause()
    await TrackPlayer.reset()
    setSession(null)
    setBook(null)
    setChapter(null)
    setChapters([])
    setPlaying(false)
  }, [store])

  const removeSession = useCallback(
    async (sessionId: string) => {
      try {
        if (sessionRef.current?.id === sessionId) await close()
        await deleteSession(sessionId)
        invalidateSessions()
      } catch (error: unknown) {
        toast.error(describe(error, 'Poslech se nepodařilo smazat'))
      }
    },
    [close, invalidateSessions],
  )

  const playItem = useCallback(
    async (bookId: string) => {
      const open = sessionRef.current
      if (!open) return
      await store({ force: true })
      await openSession(open, bookId)
    },
    [openSession, store],
  )

  const toggle = useCallback(async () => {
    const state = await TrackPlayer.getPlaybackState()
    if (state.state === State.Playing) await TrackPlayer.pause()
    else await TrackPlayer.play()
  }, [])

  const seek = useCallback(async (seconds: number) => {
    const target = Math.max(0, seconds)
    await TrackPlayer.seekTo(target)
    lastPositionRef.current = target
    setPosition(target)
  }, [])

  const skip = useCallback(async (delta: number) => {
    await TrackPlayer.seekBy(delta)
  }, [])

  const step = useCallback(
    async (delta: number) => {
      const list = chaptersRef.current
      const index = list.findIndex((item) => item.id === chapterRef.current?.id)
      const next = index + delta
      if (index < 0 || next < 0 || next >= list.length) return

      await store({ force: true })
      await TrackPlayer.skip(next)
      await TrackPlayer.play()
    },
    [store],
  )

  const nextChapter = useCallback(() => step(1), [step])
  const prevChapter = useCallback(() => step(-1), [step])

  const setSpeed = useCallback(
    async (value: number) => {
      speedRef.current = value
      setSpeedState(value)
      await TrackPlayer.setRate(value)
      await store({ force: true })
    },
    [store],
  )

  // --- události přehrávače ---

  useTrackPlayerEvents(
    [Event.PlaybackProgressUpdated, Event.PlaybackActiveTrackChanged, Event.PlaybackState, Event.PlaybackQueueEnded],
    async (event) => {
      if (event.type === Event.PlaybackProgressUpdated) {
        const now = event.position
        const diff = now - lastPositionRef.current
        // Do deníku jde jen plynulý posun. Převíjení i výměna kapitoly udělají
        // skok, a ten se nepočítá.
        if (playingRef.current && diff > 0 && diff < MAX_TIMEUPDATE_GAP_SECONDS * speedRef.current) {
          listenedRef.current += diff
        }
        lastPositionRef.current = now
        setPosition(now)
        if (event.duration > 0) setDuration(event.duration)
        return
      }

      if (event.type === Event.PlaybackState) {
        const isPlaying = event.state === State.Playing
        playingRef.current = isPlaying
        setPlaying(isPlaying)
        // Pauza je přirozený okamžik k zápisu: uživatel odložil telefon.
        if (!isPlaying) await store()
        return
      }

      if (event.type === Event.PlaybackActiveTrackChanged) {
        const list = chaptersRef.current
        const previous = event.lastIndex != null ? list[event.lastIndex] : undefined
        const next = event.index != null ? list[event.index] : undefined

        // Konec poslední kapitoly je koncem knihy; zapsat se musí dřív, než
        // se příznak sveze k nesprávné.
        if (previous && list[list.length - 1]?.id === previous.id && next === undefined) {
          await store({ bookFinished: true, force: true })
        }
        if (!next) return

        chapterRef.current = next
        setChapter(next)
        setDuration(next.duration_seconds)
        lastPositionRef.current = 0
        setPosition(0)
        const files = bookRef.current ? await bookChapterFiles(bookRef.current.id) : new Map()
        setOffline(files.has(next.id))
        await store({ force: true })
        return
      }

      if (event.type === Event.PlaybackQueueEnded) {
        // Poslední kapitola knihy: u vícedílného poslechu se jde na další
        // knihu, u jednodílného je poslech doposlechnutý.
        const open = sessionRef.current
        const current = bookRef.current
        const index = open && current ? open.items.findIndex((item) => item.book_id === current.id) : -1
        const nextItem = index >= 0 && open ? open.items[index + 1] : undefined
        if (open && nextItem) {
          await store({ bookFinished: true, force: true })
          await openSession(open, nextItem.book_id, undefined, true)
          return
        }
        await store({ bookFinished: true, finished: true, force: true })
      }
    },
  )

  // Pravidelný zápis během poslechu; v pauze není co ukládat.
  useEffect(() => {
    if (!playing) return
    const timer = setInterval(() => void store(), SAVE_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [playing, store])

  // Odchod do pozadí: uložit, než systém aplikaci uspí.
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (status) => {
      if (status !== 'active') void store({ force: true })
    })
    return () => subscription.remove()
  }, [store])

  const value = useMemo<PlayerValue>(
    () => ({
      session,
      book,
      chapter,
      chapters,
      position,
      duration,
      playing,
      loading,
      speed,
      offline,
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
      close,
    }),
    [
      session,
      book,
      chapter,
      chapters,
      position,
      duration,
      playing,
      loading,
      speed,
      offline,
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
      close,
    ],
  )

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>
}

/** Token do adresy audia; bez spojení se vrátí null a hraje se z disku. */
async function streamTokenOrNull(): Promise<string | null> {
  try {
    return (await fetchStreamToken()).token
  } catch {
    return null
  }
}

function describe(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.status === 0) return 'Bez připojení k serveru'
  return error instanceof Error && error.message ? error.message : fallback
}
