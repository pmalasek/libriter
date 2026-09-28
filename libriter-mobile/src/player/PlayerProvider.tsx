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
import { Alert, AppState, type AlertButton } from 'react-native'
import NetInfo from '@react-native-community/netinfo'
import { useQueryClient } from '@tanstack/react-query'
import TrackPlayer, { Event, State, useTrackPlayerEvents, type Track } from 'react-native-track-player'
import {
  ApiError,
  apiFetch,
  apiUrl,
  currentBookId,
  formatBytes,
  MAX_TIMEUPDATE_GAP_SECONDS,
  queryKeys,
  SAVE_INTERVAL_MS,
  sessionItem,
  t,
  type Book,
  type Chapter,
  type PlaySession,
} from 'libriter-shared'

import { fetchStreamToken } from '@/api/stream'
import { useAuth } from '@/auth/AuthProvider'
import { toast } from '@/components/Toast'
import { getMode } from '@/data/mode'
import { withSource } from '@/data/sources'
import { bookChapterFiles, getDownload } from '@/db/downloads'
import { downloadManager } from '@/downloads/downloadManager'
import { getSessionMirror } from '@/db/library'
import { syncEngine } from '@/sync/syncEngine'
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
  // Obnova stavu čte ze serveru, takže musí počkat na načtený token.
  const signedIn = Boolean(useAuth().session)
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
  /** Doposlechnuté stažené knihy, u kterých se čeká na dotaz na smazání. */
  const pendingDeleteRef = useRef(new Set<string>())
  /** Knihy, na které už se dotaz položil; konec knihy hlásí dvě události. */
  const promptedRef = useRef(new Set<string>())

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
  /** Promítne načtený poslech do stavu – po load() i po obnově. */
  const apply = useCallback(
    (loaded: {
      session: PlaySession
      book: Book
      chapters: Chapter[]
      index: number
      position: number
      speed: number
      offline: boolean
    }) => {
      speedRef.current = loaded.speed
      setSpeedState(loaded.speed)
      setSession(loaded.session)
      setBook(loaded.book)
      setChapters(loaded.chapters)
      setChapter(loaded.chapters[loaded.index])
      setPosition(loaded.position)
      setDuration(loaded.chapters[loaded.index].duration_seconds)
      setOffline(loaded.offline)
      lastPositionRef.current = loaded.position
      listenedRef.current = 0
      resetFingerprint()
    },
    [],
  )

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
      promptedRef.current.delete(input.bookId)
      try {
        await ensurePlayer()

        const [nextBook, nextChapters, files] = await Promise.all([
          withSource((s) => s.book(input.bookId)),
          withSource((s) => s.chapters(input.bookId)),
          bookChapterFiles(input.bookId),
        ])
        if (!nextBook || nextChapters.length === 0) {
          toast.error(t('mobile.player.errors.noChapters'))
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
            // Podle nich se stav obnoví, když React naběhne znovu a přehrávací
            // služba mezitím hraje dál (viz restore níže).
            sessionId: input.session.id,
            bookId: nextBook.id,
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
        await TrackPlayer.setRate(speedRef.current)

        apply({
          session: input.session,
          book: nextBook,
          chapters: nextChapters,
          index,
          position: target,
          speed: speedRef.current,
          offline: files.has(nextChapters[index].id),
        })

        if (input.autoplay) await TrackPlayer.play()
      } catch (error: unknown) {
        toast.error(error instanceof Error ? error.message : t('mobile.player.errors.playFailed'))
      } finally {
        seekingRef.current = false
        setLoading(false)
      }
    },
    [apply],
  )

  /**
   * Obnova po novém startu Reactu. Android umí zrušit Activity, zatímco
   * přehrávací služba hraje dál; klepnutí na notifikaci pak spustí React
   * znovu a přehrávač by se tvářil prázdný. Stav se proto vyčte z běžícího
   * přehrávače – bez setQueue a seekTo, aby přehrávání nezaškobrtlo.
   */
  useEffect(() => {
    if (!signedIn) return
    let cancelled = false
    void (async () => {
      try {
        await ensurePlayer()
        const track = await TrackPlayer.getActiveTrack()
        const storedSessionId = typeof track?.sessionId === 'string' ? track.sessionId : null
        const bookId = typeof track?.bookId === 'string' ? track.bookId : null
        if (!track || !storedSessionId || !bookId || sessionRef.current) return

        const sessionId = syncEngine.resolvedId(storedSessionId)
        const [restored, nextBook, nextChapters, files, index, progress, playback, rate] = await Promise.all([
          getSessionMirror(sessionId).then(
            (mirror) => mirror ?? apiFetch<PlaySession>(`/sessions/${sessionId}`),
          ),
          withSource((source) => source.book(bookId)),
          withSource((source) => source.chapters(bookId)),
          bookChapterFiles(bookId),
          TrackPlayer.getActiveTrackIndex(),
          TrackPlayer.getProgress(),
          TrackPlayer.getPlaybackState(),
          TrackPlayer.getRate(),
        ])
        // Uživatel mezitím mohl pustit něco jiného; to má přednost.
        if (cancelled || sessionRef.current || !nextBook || index == null) return
        const chapterIndex = nextChapters.findIndex((item) => item.id === track.id)
        if (chapterIndex < 0) return

        apply({
          session: restored,
          book: nextBook,
          chapters: nextChapters,
          index: chapterIndex,
          position: progress.position,
          speed: rate,
          offline: files.has(track.id),
        })
        const isPlaying = playback.state === State.Playing
        playingRef.current = isPlaying
        setPlaying(isPlaying)
      } catch (error: unknown) {
        // Bez obnovy se nic nerozbije – jen se ukáže prázdný přehrávač.
        console.warn('přehrávač: obnova stavu selhala', error)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [apply, signedIn])

  /**
   * Otevře poslech na jeho rozehrané knize. `check` říká, jak naložit
   * s nestaženou knihou bez sítě nebo v offline režimu: `ask` se zeptá,
   * `silent` (automatický přechod na další knihu, uživatel se nedívá) ji
   * bez sítě vynechá, `skip` – volající už se zeptal.
   */
  const openSession = useCallback(
    async (
      target: PlaySession,
      bookId?: string,
      chapterId?: string,
      fromStart = false,
      check: 'ask' | 'silent' | 'skip' = 'ask',
    ) => {
      const id = bookId ?? currentBookId(target)
      if (!id) return
      if (check !== 'skip' && !(await confirmPlayable(id, check === 'ask'))) return
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
        // Dotaz na nestaženou knihu padne dřív, než se založí poslech –
        // po „Zrušit“ by jinak v seznamu zůstal prázdný.
        if (!(await confirmPlayable(bookId, true))) return
        // Otevřený poslech s touhle knihou se jen přepne – zakládat nový by
        // zahodil pozici, kterou přehrávač drží.
        const open = sessionRef.current
        if (open && open.items.some((item) => item.book_id === bookId)) {
          await store({ force: true })
          await openSession(open, bookId, chapterId, Boolean(chapterId), 'skip')
          return
        }
        const target = await startSession({ kind: 'book', book_id: bookId })
        invalidateSessions()
        // Kliknutí na konkrétní kapitolu ji spustí od začátku; „Přehrát“
        // u knihy pokračuje tam, kde poslech skončil.
        await openSession(target, bookId, chapterId, Boolean(chapterId), 'skip')
      } catch (error: unknown) {
        toast.error(describe(error, t('mobile.player.errors.createFailed')))
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
        toast.error(describe(error, t('mobile.player.errors.createSeriesFailed')))
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
        toast.error(describe(error, t('mobile.player.errors.createListFailed')))
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
        toast.success(t('mobile.player.added'))
      } catch (error: unknown) {
        toast.error(describe(error, t('mobile.player.errors.addFailed')))
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
          toast.error(t('mobile.player.errors.sessionGone'))
          return
        }
        await openSession(target)
      } catch (error: unknown) {
        toast.error(describe(error, t('mobile.player.errors.openFailed')))
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

  /**
   * Zeptá se na smazání doposlechnutých stažených knih. Konec knihy často
   * přijde se zamčeným telefonem; dotaz pak počká na návrat do aplikace.
   */
  const askDeleteFinished = useCallback(async () => {
    if (AppState.currentState !== 'active') return
    for (const bookId of [...pendingDeleteRef.current]) {
      pendingDeleteRef.current.delete(bookId)
      if (promptedRef.current.has(bookId)) continue
      promptedRef.current.add(bookId)

      const download = await getDownload(bookId)
      if (download?.state !== 'complete') continue
      const title = (await withSource((s) => s.book(bookId)).catch(() => null))?.title ?? ''
      Alert.alert(
        t('mobile.player.finished.title'),
        t('mobile.player.finished.question', { title, size: formatBytes(download.bytesDone) }),
        [
          { text: t('mobile.player.finished.keep'), style: 'cancel' },
          {
            text: t('mobile.player.finished.delete'),
            style: 'destructive',
            onPress: () =>
              void (async () => {
                // Fronta přehrávače ukazuje na mazané soubory, dokud je kniha
                // otevřená (poslech jí skončil); zavřít ji musí dřív.
                if (bookRef.current?.id === bookId) await close()
                await downloadManager.remove(bookId)
                toast.success(t('mobile.player.finished.deleted'))
              })().catch((error: unknown) => toast.error(describe(error, t('mobile.player.errors.deleteFailed')))),
          },
        ],
      )
    }
  }, [close])

  const bookFinished = useCallback(
    (bookId: string) => {
      pendingDeleteRef.current.add(bookId)
      void askDeleteFinished()
    },
    [askDeleteFinished],
  )

  const removeSession = useCallback(
    async (sessionId: string) => {
      try {
        if (sessionRef.current?.id === sessionId) await close()
        await deleteSession(sessionId)
        invalidateSessions()
      } catch (error: unknown) {
        toast.error(describe(error, t('mobile.player.errors.deleteFailed')))
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
          if (bookRef.current) bookFinished(bookRef.current.id)
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
          if (current) bookFinished(current.id)
          await openSession(open, nextItem.book_id, undefined, true, 'silent')
          return
        }
        await store({ bookFinished: true, finished: true, force: true })
        if (current) bookFinished(current.id)
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
      else void askDeleteFinished()
    })
    return () => subscription.remove()
  }, [askDeleteFinished, store])

  // Návrat sítě: uložit pozici teď, ať server dostane aktuální, ne až 10 s starou.
  useEffect(() => syncEngine.onReconnect(() => store({ force: true })), [store])

  // Poslech založený bez sítě dostal serverové ID; další pozice patří pod něj.
  useEffect(
    () =>
      syncEngine.onSessionResolved((localId, fresh) => {
        if (sessionRef.current?.id !== localId) return
        sessionRef.current = fresh
        setSession(fresh)
      }),
    [],
  )

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

/**
 * Smí se kniha pustit? V offline režimu nebo bez sítě musí být stažená celá;
 * jinak se nabídne stažení, případně (je-li síť) přehrání ze serveru.
 * `interactive: false` je automatický přechod na další knihu – nikdo se
 * nedívá, takže se bez sítě jen oznámí, že kniha chybí.
 */
async function confirmPlayable(bookId: string, interactive: boolean): Promise<boolean> {
  const connected = (await NetInfo.fetch()).isConnected !== false
  if (getMode() !== 'offline' && connected) return true

  const [chapters, files] = await Promise.all([withSource((s) => s.chapters(bookId)), bookChapterFiles(bookId)])
  // Kniha bez kapitol se nechá projít – load() ohlásí vlastní chybu.
  if (chapters.every((item) => files.has(item.id))) return true
  if (connected && !interactive) return true

  const title = (await withSource((s) => s.book(bookId)).catch(() => null))?.title ?? ''
  if (!interactive) {
    toast.error(t('mobile.player.offline.nextUnavailable', { title }))
    return false
  }

  const download = await getDownload(bookId)
  const inProgress = download?.state === 'queued' || download?.state === 'downloading'
  const message = [
    t(connected ? 'mobile.player.offline.text' : 'mobile.player.offline.textNoNetwork', { title }),
    inProgress ? t('mobile.player.offline.downloading') : '',
  ]
    .filter(Boolean)
    .join('\n\n')

  return new Promise<boolean>((resolve) => {
    const buttons: AlertButton[] = [{ text: t('mobile.player.offline.cancel'), style: 'cancel', onPress: () => resolve(false) }]
    // Bez sítě by stahování jen čekalo ve frontě; nabízí se, jen když poběží.
    if (connected && !inProgress) {
      buttons.push({
        text: t('mobile.player.offline.download'),
        onPress: () => {
          resolve(false)
          downloadManager
            .enqueue(bookId)
            .then(() => toast.success(t('mobile.player.offline.downloadStarted')))
            .catch((error: unknown) => toast.error(error instanceof Error ? error.message : String(error)))
        },
      })
    }
    if (connected) buttons.push({ text: t('mobile.player.offline.playOnline'), onPress: () => resolve(true) })
    Alert.alert(t('mobile.player.offline.title'), message, buttons, { cancelable: true, onDismiss: () => resolve(false) })
  })
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
  if (error instanceof ApiError && error.status === 0) return t('mobile.player.errors.noConnection')
  return error instanceof Error && error.message ? error.message : fallback
}
