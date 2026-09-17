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
import TrackPlayer, {
  Event,
  State,
  useTrackPlayerEvents,
  type Track,
} from 'react-native-track-player'
import {
  apiFetch,
  apiUrl,
  currentBookId,
  MAX_TIMEUPDATE_GAP_SECONDS,
  SAVE_INTERVAL_MS,
  sessionItem,
  type Book,
  type Chapter,
  type PlaySession,
} from 'libriter-shared'
import * as Crypto from 'expo-crypto'

import { fetchStreamToken } from '@/api/queries'
import { bookChapterFiles } from '@/db/downloads'
import {
  getBook,
  getSessionMirror,
  listChapters,
  listSessions,
  saveSessionMirror,
} from '@/db/library'
import { savePosition, resetFingerprint } from './positionSaver'
import { ensurePlayer } from './setup'

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

  /** Otevře knihu: pokračuje v rozposlouchaném poslechu, nebo založí nový. */
  playBook: (bookId: string, chapterId?: string) => Promise<void>
  /** Přepne na jiný rozposlouchaný poslech. */
  openSession: (sessionId: string) => Promise<void>
  toggle: () => Promise<void>
  seek: (seconds: number) => Promise<void>
  skip: (delta: number) => Promise<void>
  nextChapter: () => Promise<void>
  prevChapter: () => Promise<void>
  setSpeed: (speed: number) => Promise<void>
  close: () => Promise<void>
}

const PlayerContext = createContext<PlayerValue | null>(null)

export function usePlayer(): PlayerValue {
  const ctx = useContext(PlayerContext)
  if (!ctx) throw new Error('usePlayer musí být uvnitř PlayerProvider')
  return ctx
}

export function PlayerProvider({ children }: { children: ReactNode }) {
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
  /** Odposlouchané sekundy od posledního zápisu; jdou do deníku poslechu. */
  const listenedRef = useRef(0)
  const lastPositionRef = useRef(0)
  /** Mezi nastavením fronty a doskočením na pozici se nesmí ukládat. */
  const seekingRef = useRef(false)

  sessionRef.current = session
  bookRef.current = book
  chaptersRef.current = chapters
  chapterRef.current = chapter

  const store = useCallback(
    async (options: { finished?: boolean; bookFinished?: boolean; force?: boolean } = {}) => {
      const openSessionValue = sessionRef.current
      const openBook = bookRef.current
      if (!openSessionValue || !openBook || seekingRef.current) return

      const listened = listenedRef.current
      listenedRef.current = 0

      const saved = await savePosition({
        sessionId: openSessionValue.id,
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

      const fresh = await getSessionMirror(openSessionValue.id)
      if (fresh && sessionRef.current?.id === fresh.id) setSession(fresh)
    },
    [],
  )

  /**
   * Naplní frontu přehrávače kapitolami knihy a doskočí na uloženou pozici.
   *
   * Zdroj kapitoly volí podle toho, co je v telefonu: stažený soubor vyhrává
   * nad streamem, takže rozehraná kniha přežije i vypnutou síť.
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
          getBook(input.bookId),
          listChapters(input.bookId),
          bookChapterFiles(input.bookId),
        ])
        if (!nextBook || nextChapters.length === 0) return

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
      } finally {
        seekingRef.current = false
        setLoading(false)
      }
    },
    [],
  )

  const openSession = useCallback(
    async (sessionId: string) => {
      const mirror = await getSessionMirror(sessionId)
      if (!mirror) return
      const bookId = currentBookId(mirror)
      if (!bookId) return
      const item = sessionItem(mirror, bookId)
      await load({
        session: mirror,
        bookId,
        chapterId: item?.chapter_id,
        positionSeconds: item?.position_seconds ?? 0,
        autoplay: true,
      })
    },
    [load],
  )

  const playBook = useCallback(
    async (bookId: string, chapterId?: string) => {
      const target = await resolveSession(bookId)
      const item = sessionItem(target, bookId)
      await load({
        session: target,
        bookId,
        // Kliknutí na konkrétní kapitolu ji spustí od začátku; „Přehrát“
        // u knihy pokračuje tam, kde poslech skončil.
        chapterId: chapterId ?? item?.chapter_id,
        positionSeconds: chapterId ? 0 : (item?.position_seconds ?? 0),
        autoplay: true,
      })
    },
    [load],
  )

  const toggle = useCallback(async () => {
    const state = await TrackPlayer.getPlaybackState()
    if (state.state === State.Playing) await TrackPlayer.pause()
    else await TrackPlayer.play()
  }, [])

  const seek = useCallback(async (seconds: number) => {
    await TrackPlayer.seekTo(Math.max(0, seconds))
    lastPositionRef.current = Math.max(0, seconds)
    setPosition(Math.max(0, seconds))
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

  const close = useCallback(async () => {
    await store({ force: true })
    await TrackPlayer.pause()
    setSession(null)
    setBook(null)
    setChapter(null)
    setChapters([])
  }, [store])

  // --- události přehrávače ---

  useTrackPlayerEvents(
    [Event.PlaybackProgressUpdated, Event.PlaybackActiveTrackChanged, Event.PlaybackState, Event.PlaybackQueueEnded],
    async (event) => {
      if (event.type === Event.PlaybackProgressUpdated) {
        const now = event.position
        const diff = now - lastPositionRef.current
        // Do deníku jde jen plynulý posun. Převíjení i výměna kapitoly udělají
        // skok, a ten se nepočítá.
        if (playing && diff > 0 && diff < MAX_TIMEUPDATE_GAP_SECONDS * speedRef.current) {
          listenedRef.current += diff
        }
        lastPositionRef.current = now
        setPosition(now)
        if (event.duration > 0) setDuration(event.duration)
        return
      }

      if (event.type === Event.PlaybackState) {
        const isPlaying = event.state === State.Playing
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
        // Poslední kapitola poslední knihy poslechu = doposlechnuto.
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
      openSession,
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
      openSession,
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

/**
 * Najde poslech, do kterého kniha patří. Online se o něj řekne serveru (ten
 * pokračuje v rozposlouchaném místo zakládání duplicity); bez spojení vznikne
 * provizorní session s klientským UUID, kterou synchronizace vymění za
 * serverovou dřív, než odešle první pozici.
 */
async function resolveSession(bookId: string): Promise<PlaySession> {
  try {
    const created = await apiFetch<PlaySession>('/sessions', {
      method: 'POST',
      json: { kind: 'book', book_id: bookId },
    })
    await saveSessionMirror(created)
    return created
  } catch {
    // Offline. Rozposlouchaný poslech, který knihu obsahuje, se použije
    // znovu – jinak by každé zapnutí v letadle založilo další session
    // s nulovou pozicí.
    const known = await listSessions()
    const existing = known.find(
      (candidate) =>
        !candidate.finished_at && candidate.items.some((item) => item.book_id === bookId),
    )
    if (existing) return existing

    const now = new Date().toISOString()
    const local: PlaySession = {
      id: Crypto.randomUUID(),
      kind: 'book',
      source_id: bookId,
      current_book_id: bookId,
      playback_speed: 1,
      created_at: now,
      updated_at: now,
      items: [{ book_id: bookId, position: 1, position_seconds: 0 }],
    }
    await saveSessionMirror(local, true)
    return local
  }
}

/** Token do adresy audia; bez spojení se vrátí null a hraje se z disku. */
async function streamTokenOrNull(): Promise<string | null> {
  try {
    return (await fetchStreamToken()).token
  } catch {
    return null
  }
}
