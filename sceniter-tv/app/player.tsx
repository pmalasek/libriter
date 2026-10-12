import { useCallback, useEffect, useRef, useState } from 'react'
import { ActivityIndicator, AppState, BackHandler, Platform, StyleSheet, View, useTVEventHandler, useWindowDimensions, type HWEvent, type ViewStyle } from 'react-native'
import { router, useLocalSearchParams, useNavigation } from 'expo-router'
import { useEventListener } from 'expo'
import { VideoView, useVideoPlayer } from 'expo-video'
import { useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowLeftIcon, RotateCcwIcon } from 'lucide-react-native'
import {
  canResume,
  escalateMode,
  languageCode,
  saveProgress,
  setAspectRatio,
  shouldSeekTo,
  NEXT_COUNTDOWN,
  SAVE_INTERVAL_MS,
  WatchMeter,
  type AspectRatio,
  type Playback,
  type PlaybackMode,
} from 'sceniter-shared'
import { episodeQuery, movieQuery, useEpisode, usePrefetchEpisode, usePrefetchPlayback } from '@/api/queries'
import { Button } from '@/components/Button'
import { T } from '@/components/T'
import { goToShowEpisode } from '@/lib/nav'
import { usePrefs } from '@/lib/prefs'
import { deviceId } from '@/lib/storage'
import { episodeShort } from '@/lib/titles'
import { Chrome, NextUp, TracksPanel, UpNextPill } from '@/player/Overlay'
import { playbackQueryOptions } from '@/player/playbackQuery'
import { StreamSession, TranscodeBusyError } from '@/player/session'
import { SubtitleOverlay, useCues } from '@/player/Subtitles'
import { colors, px } from '@/theme'

/** Ovládání zmizí po chvíli bez stisku, pokud video hraje. */
const CHROME_MS = 4000
/** Na tlačítkách lišty (čtení popisků, volba) zůstává lišta déle. */
const CHROME_BUTTONS_MS = 8000
/** Jeden stisk šipky přetočí vždy o stejný kus (s). */
const SEEK_STEP = 10
/** Jak často se při držené šipce posune posuvník. */
const HOLD_TICK_MS = 200
/** Přetáčení šipkami se potvrdí samo po chvíli bez dalšího stisku. */
const SCRUB_COMMIT_MS = 600
/** Jak dlouho po skoku nevěřit hlášené poloze, dokud se přehrávač nesrovná. */
const SETTLE_MS = 4000

/** Krok při držení (s za tik): zrychluje jen s délkou držení, ne se stisky. */
function holdStep(heldMs: number): number {
  return heldMs < 2000 ? 10 : heldMs < 4000 ? 30 : 60
}

type Phase = 'loading' | 'resuming' | 'seeking' | 'playing'

/**
 * Výchozí titulky: vynucené v jazyce rozhraní vždy (překlad cizojazyčných
 * pasáží), běžné v jazyce rozhraní jen při volbě „V mém jazyce“.
 */
function defaultSubtitle(pb: Playback, uiLang: string, mode: 'off' | 'language'): string {
  const mine = pb.subtitles.filter((s) => languageCode(s.language) === uiLang)
  const forced = mine.find((s) => s.forced)
  if (mode === 'language') return (mine.find((s) => !s.forced) ?? forced)?.key ?? ''
  return forced?.key ?? ''
}

/** Jak dlouho nejvýš čekat na pozici ze serveru, než se pustí ta z obrazovky. */
const RESUME_CHECK_MS = 3000

/**
 * Plocha videa s vynuceným poměrem stran: co největší obdélník daného poměru
 * uprostřed obrazovky. Obraz se do ní roztáhne (contentFit="fill") – tak se
 * opraví soubor, který má poměr stran zapsaný špatně.
 */
function aspectBox(win: { width: number; height: number }, ratio: AspectRatio): ViewStyle {
  const [w, h] = ratio.split(':').map(Number)
  const r = w / h
  const width = win.width / win.height > r ? win.height * r : win.width
  const height = width / r
  return { position: 'absolute', width, height, left: (win.width - width) / 2, top: (win.height - height) / 2 }
}

/**
 * Pozice, od které pokračovat, přímo ze serveru – data obrazovky, ze které se
 * přišlo, můžou být stará (díl mezitím rozkoukaný na jiné TV, domovská
 * obrazovka z disku, aplikace dlouho na pozadí). 0 = od začátku,
 * null = nezjištěno (platí pozice z obrazovky).
 */
function checkResume(qc: QueryClient, ids: { movieId?: string; episodeId?: string }): Promise<number | null> {
  const request = ids.episodeId
    ? qc.fetchQuery({ ...episodeQuery(ids.episodeId), staleTime: 0 }).then((d) => d.progress)
    : ids.movieId
      ? qc.fetchQuery({ ...movieQuery(ids.movieId), staleTime: 0 }).then((d) => d.user.progress)
      : undefined
  if (!request) return Promise.resolve(null)
  const fresh = request.then((p) => (canResume(p) ? p.position_sec : 0)).catch(() => null)
  const timeout = new Promise<null>((resolve) => setTimeout(() => resolve(null), RESUME_CHECK_MS))
  return Promise.race([fresh, timeout])
}

export default function PlayerScreen() {
  const params = useLocalSearchParams<{
    mediaId: string
    movieId?: string
    episodeId?: string
    title: string
    subtitle?: string
    start?: string
    resume?: string
  }>()
  const { t, i18n } = useTranslation()
  const qc = useQueryClient()
  const { prefs } = usePrefs()

  // Co se právě přehrává. Další díl se přepne tady, bez navigace – nová
  // obrazovka přehrávače by při odchodu staré přišla o své rozhodnutí
  // o přehrávání a převod na serveru by se zastavil.
  const [current, setCurrent] = useState(() => ({
    mediaId: params.mediaId,
    movieId: params.movieId,
    episodeId: params.episodeId,
    subtitle: params.subtitle,
  }))
  const ids = useRef(current)
  ids.current = current
  const { mediaId, episodeId } = current

  const episode = useEpisode(episodeId)
  const nextEpisode = episode.data?.next ?? undefined
  // Detail tohoto seriálu je přímo pod přehrávačem (přišlo se z něj) –
  // tlačítko Seriál se na něj vrátí místo otevírání nového. Stav zásobníku
  // (kořenový Stack) se čte až při stisku; useRootNavigationState vrací
  // obal nad navigátorem, ne jeho cesty.
  const navigation = useNavigation()
  const openShow = () => {
    const e = episode.data?.episode
    if (!e) return
    const routes = navigation.getState()?.routes ?? []
    const below = routes.length >= 2 ? routes[routes.length - 2] : undefined
    goToShowEpisode(e, below?.name === 'show/[id]' && (below.params as { id?: string } | undefined)?.id === e.show_id)
  }
  const nextRef = useRef(nextEpisode)
  nextRef.current = nextEpisode
  const prefetchEpisode = usePrefetchEpisode()
  const prefetchPlayback = usePrefetchPlayback(0)

  // ---- rozhodnutí o přehrávání ----

  const [audio, setAudio] = useState<number | undefined>()
  const [mode, setMode] = useState<PlaybackMode | undefined>()
  const [attempt, setAttempt] = useState(0)
  const playback = useQuery(playbackQueryOptions(mediaId, { audio, mode, attempt }))
  const pb = playback.data
  const pbRef = useRef(pb)
  pbRef.current = pb

  const player = useVideoPlayer(null, (p) => {
    p.timeUpdateEventInterval = 0.25
    p.keepScreenOnWhilePlaying = true
  })

  const seekTo = useRef<number | undefined>(Number(params.start) > 0 ? Number(params.start) : undefined)
  // Ověření pozice na serveru běží souběžně s otevřením proudu; spotřebuje se
  // při prvním úspěšném otevření (undefined = ještě nespuštěno).
  const resumeCheck = useRef<Promise<number | null> | null | undefined>(undefined)
  if (resumeCheck.current === undefined) resumeCheck.current = params.resume ? checkResume(qc, current) : null
  // Dokud se poprvé nerozehrálo, neukládat: pozice by byla ta převzatá
  // (třeba stará z jiné TV) s čerstvým časem a přepsala by novější na serveru.
  const started = useRef(false)
  const pendingSeek = useRef<number | undefined>(undefined)
  const [phase, setPhase] = useState<Phase>('loading')
  const phaseRef = useRef(phase)
  phaseRef.current = phase
  const [failure, setFailure] = useState<string | null>(null)

  // Poslední známá poloha – ukládání při odchodu už na uvolněný přehrávač sahat nesmí.
  const timeRef = useRef(0)
  // Přebalení spuštěné od místa skoku začíná v čase `offset` videa: poloha
  // přehrávače 0 = offset. Čas v aplikaci (lišta, titulky, ukládání) je vždy
  // čas videa, s přehrávačem se mluví v jeho čase.
  const offset = useRef(0)
  // Skok mimo hotovou část přebalení: nový převod na serveru od cíle.
  const [seekGen, setSeekGen] = useState(0)
  const seeking = useRef(false)
  // Než se přehrávači předá nový zdroj, hlásí ještě ten starý (i chybu,
  // když se jeho převod na serveru zastaví) – jeho události se ignorují.
  const switching = useRef(false)
  // Po skoku přehrávač chvíli hlásí ještě starou polohu.
  const settle = useRef<{ target: number; until: number } | undefined>(undefined)
  const position = useCallback(() => (timeRef.current > 0 ? timeRef.current : seekTo.current ?? 0), [])

  // ---- ukládání pozice ----

  const meter = useRef(new WatchMeter())
  const durationRef = useRef(0)
  const lastSave = useRef(0)
  const device = useRef<string | undefined>(undefined)
  useEffect(() => {
    void deviceId().then((id) => (device.current = id))
  }, [])

  const save = useCallback(
    (completed?: boolean) => {
      const current = pbRef.current
      if (!current || (!started.current && !completed)) return Promise.resolve()
      lastSave.current = Date.now()
      const { movieId, episodeId, mediaId } = ids.current
      return saveProgress({
        movie_id: movieId,
        episode_id: episodeId,
        media_id: mediaId,
        // Dokud se na uloženou pozici neskočilo, stojí video na začátku.
        position_sec: Math.max(0, seekTo.current ?? timeRef.current),
        duration_sec: current.duration_sec || durationRef.current || undefined,
        watched_seconds: meter.current.take(),
        completed,
        device_id: device.current,
      }).catch(() => {
        // Uloží se při dalším pokusu; přehrávání kvůli tomu nepřerušujeme.
      })
    },
    [],
  )

  useEffect(() => {
    const sub = AppState.addEventListener('change', (state) => {
      if (state !== 'active') {
        player.pause()
        void save()
      }
    })
    return () => {
      sub.remove()
      // Až po uložení, ať detail a domovská obrazovka ukážou novou pozici.
      // Jen dotazy s průběhem – výpisy a rozhodnutí o přehrávání zůstanou.
      void save().finally(() =>
        qc.invalidateQueries({
          predicate: (q) => q.queryKey[0] === 'sceniter' && ['home', 'movie', 'show', 'episode'].includes(String(q.queryKey[1])),
        }),
      )
    }
  }, [player, qc, save])

  // ---- napojení zdroje, převod na serveru, pokračování ----

  const escalate = useCallback(() => {
    const current = pbRef.current
    const nextMode = current && escalateMode(current.mode, current.transcoding_available)
    if (!nextMode) {
      setFailure(t('video.player.failed'))
      return
    }
    seekTo.current = position()
    setMode(nextMode)
  }, [position, t])

  const restart = useCallback(() => {
    seekTo.current = position()
    setAttempt((a) => a + 1)
  }, [position])

  useEffect(() => {
    if (!pb) return
    let cancelled = false
    let session: StreamSession | undefined
    switching.current = true
    setFailure(null)
    setPhase(seeking.current ? 'seeking' : 'loading')
    void (async () => {
      // Místo pokračování se zjistí před otevřením proudu: přebalení se na
      // serveru spustí rovnou od něj.
      const check = resumeCheck.current
      if (check) {
        const fresh = await check
        if (cancelled) return
        resumeCheck.current = null
        if (fresh !== null) seekTo.current = fresh > 0 ? fresh : undefined
      }
      const at = seekTo.current
      const resume = shouldSeekTo(at, pb.duration_sec)
      if (!resume) seekTo.current = undefined
      session = new StreamSession(pb, restart, resume ? at : undefined)
      try {
        await session.open()
      } catch (e) {
        if (cancelled) return
        if (e instanceof TranscodeBusyError) setFailure(t('video.player.busy'))
        else escalate()
        return
      }
      if (cancelled) return
      if (resume) {
        if (!seeking.current) setPhase('resuming')
        await session.waitForPosition(at, () => cancelled)
      }
      if (cancelled) return
      offset.current = session.start
      pendingSeek.current = resume ? Math.max(0, at - session.start) : undefined
      endedRef.current = false
      switching.current = false
      player.replace({ uri: session.url, contentType: pb.hls ? 'hls' : 'progressive' })
      player.play()
    })()
    return () => {
      cancelled = true
      session?.close()
    }
  }, [pb, seekGen, player, restart, escalate, t])

  useEventListener(player, 'statusChange', ({ status }) => {
    if (switching.current) return
    if (status === 'readyToPlay') {
      if (pendingSeek.current !== undefined) {
        player.currentTime = pendingSeek.current
        settle.current = { target: offset.current + pendingSeek.current, until: Date.now() + SETTLE_MS }
        pendingSeek.current = undefined
      }
      seekTo.current = undefined
      seeking.current = false
      started.current = true
      setPhase('playing')
    } else if (status === 'error') {
      escalate()
    }
  })

  // ---- čas, pauza, konec ----

  const [time, setTime] = useState(0)
  const [duration, setDuration] = useState(0)
  // Hotová část rostoucího playlistu (přebalení) – dál přehrávač skočit neumí.
  const [available, setAvailable] = useState<number | undefined>()
  const [readyFrom, setReadyFrom] = useState(0)
  const availableRef = useRef<number | undefined>(undefined)
  const [paused, setPaused] = useState(false)
  const [countdown, setCountdown] = useState<number | null>(null)
  const [ended, setEnded] = useState(false)
  // Android hlásí konec opakovaně (při každé změně stavu po dohrání).
  const endedRef = useRef(false)

  useEventListener(player, 'timeUpdate', ({ currentTime }) => {
    // Při načítání (i nového převodu po skoku) hlásí přehrávač nesmysly.
    if (switching.current || phaseRef.current !== 'playing') return
    const at = offset.current + currentTime
    const s = settle.current
    if (s) {
      if (Math.abs(at - s.target) > 2 && Date.now() < s.until) return
      settle.current = undefined
    }
    timeRef.current = at
    setTime(at)
    // Rostoucí playlist (přebalení) hlásí jen hotovou část – platí délka ze serveru.
    const total = pbRef.current?.duration_sec || player.duration
    if (total > 0) {
      durationRef.current = total
      setDuration(total)
    }
    const growing = pbRef.current?.hls && pbRef.current.mode !== 'transcode'
    const ready = growing && Number.isFinite(player.duration) && player.duration > 0 ? offset.current + player.duration : undefined
    availableRef.current = ready
    setAvailable(ready)
    setReadyFrom(offset.current)
    meter.current.tick(at, player.playing)
    if (Date.now() - lastSave.current > SAVE_INTERVAL_MS) void save()
  })

  // Po skoku přehrávač chvíli bufferuje a hlásí „nehraje“ – to není pauza
  // (ikona by blikala play/pause). Pauza je jen „nehraje a nenačítá“.
  const pauseCheck = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(pauseCheck.current), [])
  useEventListener(player, 'playingChange', ({ isPlaying }) => {
    clearTimeout(pauseCheck.current)
    if (isPlaying) {
      setPaused(false)
      return
    }
    pauseCheck.current = setTimeout(() => {
      if (player.playing || player.status === 'loading') return
      setPaused(true)
      void save()
      poke()
    }, 250)
  })

  useEventListener(player, 'playToEnd', () => {
    if (endedRef.current || phaseRef.current !== 'playing') return
    endedRef.current = true
    void save(true)
    setEnded(true)
  })

  // Co po dohrání: odpočet k dalšímu dílu, jinak zpět. Dokud se detail dílu
  // načítá, nerozhoduje se – jinak by se odešlo, i když další díl existuje.
  const episodeSettled = !episodeId || episode.isSuccess || episode.isError
  useEffect(() => {
    if (!ended || countdown !== null) return
    if (nextEpisode?.media_id) {
      setCountdown(NEXT_COUNTDOWN)
      prefetchEpisode(nextEpisode.id)
      prefetchPlayback(nextEpisode.media_id)
    } else if (episodeSettled) router.back()
  }, [countdown, ended, episodeSettled, nextEpisode, prefetchEpisode, prefetchPlayback])

  const playNext = useCallback(() => {
    const next = nextRef.current
    if (!next?.media_id) return
    // Rozkoukaný díl uložit (v poslední minutě jako dokoukaný) a zastavit,
    // ať nehraje, než se načte další.
    if (!endedRef.current) {
      const left = durationRef.current - timeRef.current
      void save(durationRef.current > 0 && left <= 60 ? true : undefined)
      player.pause()
    }
    setCountdown(null)
    setEnded(false)
    seekTo.current = undefined
    started.current = false
    resumeCheck.current = checkResume(qc, { episodeId: next.id })
    pendingSeek.current = undefined
    timeRef.current = 0
    offset.current = 0
    seeking.current = false
    settle.current = undefined
    durationRef.current = 0
    lastSave.current = 0
    meter.current.reset()
    setTime(0)
    setDuration(0)
    setPaused(false)
    setFailure(null)
    setPhase('loading')
    clearTimeout(scrubTimer.current)
    scrubRef.current = undefined
    setScrubTo(undefined)
    setPanel(false)
    setChromeAt(Date.now())
    setUpNextDismissed(false)
    upNextWarmed.current = false
    availableRef.current = undefined
    setAvailable(undefined)
    setReadyFrom(0)
    setSubtitleKey(null)
    setAspectChoice(undefined)
    setAudio(undefined)
    setMode(undefined)
    setAttempt(0)
    // Nové ID → nový dotaz na rozhodnutí → efekt výš zavře session dohraného
    // dílu a otevře další.
    setCurrent({ mediaId: next.media_id, episodeId: next.id, movieId: undefined, subtitle: episodeShort(next) })
  }, [player, qc, save])

  useEffect(() => {
    if (countdown === null) return
    const timer = setTimeout(() => {
      if (countdown <= 1) playNext()
      else setCountdown(countdown - 1)
    }, 1000)
    return () => clearTimeout(timer)
  }, [countdown, playNext])

  // ---- titulky ----

  const [subtitleKey, setSubtitleKey] = useState<string | null>(null)
  useEffect(() => {
    if (pb && subtitleKey === null) setSubtitleKey(defaultSubtitle(pb, i18n.language, prefs.subtitles))
  }, [i18n.language, pb, prefs.subtitles, subtitleKey])
  const cues = useCues(pb?.subtitles.find((s) => s.key === subtitleKey)?.url)

  // ---- ovládání ovladačem ----

  // Lišta je vidět od posledního stisku (0 = skrytá); po CHROME_MS zmizí.
  const [chromeAt, setChromeAt] = useState(() => Date.now())
  const poke = useCallback(() => setChromeAt(Date.now()), [])
  const [panel, setPanel] = useState(false)
  // Cíl přetáčení, dokud se nepotvrdí; ref pro obsluhu kláves (stav se
  // mezi rychlými stisky nestihne překreslit).
  const [scrubTo, setScrubTo] = useState<number | undefined>()
  const scrubRef = useRef<number | undefined>(undefined)
  const scrubTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const clearScrub = useCallback(() => {
    clearTimeout(scrubTimer.current)
    scrubRef.current = undefined
    setScrubTo(undefined)
  }, [])
  const [upNextDismissed, setUpNextDismissed] = useState(false)
  // Šipka dolů při skryté liště: ukázat ji a rovnou zaměřit tlačítka.
  const [focusButtons, setFocusButtons] = useState(false)
  const upNextWarmed = useRef(false)

  // Kde je fokus: posuvník, tlačítka lišty, nebo nabídka dalšího dílu.
  // Bez zaměřeného prvku by Android šipky do aplikace vůbec nedoručil.
  const zone = useRef<'seek' | 'buttons' | 'upnext' | null>(null)
  const seekRef = useRef<View>(null)
  const focusSeek = useCallback(() => seekRef.current?.requestTVFocus(), [])

  const chromeVisible = (chromeAt > 0 || paused || scrubTo !== undefined) && phase === 'playing' && !panel && countdown === null
  const chromeVisibleRef = useRef(chromeVisible)
  chromeVisibleRef.current = chromeVisible

  const hideChrome = useCallback(() => {
    // Tlačítka se se skrytou lištou odmontují – fokus musí zpět na posuvník.
    if (zone.current === 'buttons') focusSeek()
    setFocusButtons(false)
    setChromeAt(0)
  }, [focusSeek])

  useEffect(() => {
    if (!chromeAt || paused || panel || scrubTo !== undefined) return
    const keep = zone.current === 'buttons' ? CHROME_BUTTONS_MS : CHROME_MS
    const timer = setTimeout(hideChrome, Math.max(0, chromeAt + keep - Date.now()))
    return () => clearTimeout(timer)
  }, [chromeAt, hideChrome, paused, panel, scrubTo])

  // Po načtení, zavření panelu a zrušení odpočtu patří fokus posuvníku.
  useEffect(() => {
    if (phase === 'playing' && !panel && countdown === null) focusSeek()
  }, [countdown, focusSeek, panel, phase])
  // Na začátku přehrávání ukázat na chvíli lištu (název, délka).
  useEffect(() => {
    if (phase === 'playing') poke()
  }, [phase, poke])

  const commitScrub = useCallback(
    (target: number | undefined) => {
      clearScrub()
      if (target === undefined) return
      meter.current.reset()
      // Další stisk musí navázat na cíl, ne na polohu před skokem.
      timeRef.current = target
      setTime(target)
      const cur = pbRef.current
      const growing = cur?.hls && cur.mode !== 'transcode'
      const ready = availableRef.current
      if (growing && (target < offset.current || ready === undefined || target > ready - 2)) {
        // Mimo hotovou část přebalení: převod na serveru znovu od cíle.
        seekTo.current = target
        seeking.current = true
        player.pause()
        setSeekGen((g) => g + 1)
        return
      }
      settle.current = { target, until: Date.now() + SETTLE_MS }
      player.currentTime = target - offset.current
    },
    [clearScrub, player],
  )

  const scrubBy = useCallback(
    (delta: number) => {
      poke()
      // Přetáčet jde po celé délce – co ještě není převedené, převede server.
      const end = durationRef.current > 0 ? durationRef.current - 1 : Infinity
      const next = Math.max(0, Math.min(end, (scrubRef.current ?? timeRef.current) + delta))
      scrubRef.current = next
      setScrubTo(next)
      // Při držení odkládají potvrzení další tiky; potvrdí ho až puštění.
      clearTimeout(scrubTimer.current)
      scrubTimer.current = setTimeout(() => commitScrub(scrubRef.current), SCRUB_COMMIT_MS)
    },
    [commitScrub, poke],
  )

  const togglePlay = () => {
    poke()
    if (scrubRef.current !== undefined) commitScrub(scrubRef.current)
    else if (player.playing) player.pause()
    else player.play()
  }

  const restartEpisode = () => {
    poke()
    commitScrub(0)
    // Při novém převodu od začátku spustí přehrávání až nový zdroj.
    if (!seeking.current) player.play()
  }

  // Plynulé přetáčení při držené šipce (do puštění klávesy).
  const hold = useRef<ReturnType<typeof setInterval>>(undefined)
  const stopHold = () => {
    clearInterval(hold.current)
    hold.current = undefined
  }
  useEffect(() => () => clearInterval(hold.current), [])

  const handler = useRef<(e: HWEvent) => void>(() => undefined)
  handler.current = (e: HWEvent) => {
    // Jakákoli jiná klávesa (nebo puštění) držení ukončí.
    if (hold.current && !e.eventType.startsWith('long')) stopHold()
    const type = e.eventType
    // react-native-tvos na Androidu posílá jen uvolnění klávesy (1); při
    // držení opakovaně long* se stiskem (0). Stisk běžné klávesy se ignoruje,
    // jinak by se na tvOS (posílá obojí) reagovalo dvakrát.
    if (e.eventKeyAction === 0 && !type.startsWith('long')) return
    if (countdown !== null || panel || failure || phase !== 'playing') return

    // Mediální klávesy fungují vždy. Přehrát/Pauzu na Androidu vyřídí media
    // session přehrávače sama – tady by se přepnulo podruhé.
    switch (type) {
      case 'playPause':
        return Platform.OS === 'android' ? poke() : togglePlay()
      case 'play':
        poke()
        return Platform.OS === 'android' ? undefined : player.play()
      case 'pause':
        poke()
        return Platform.OS === 'android' ? undefined : player.pause()
      case 'fastForward':
      case 'rewind':
        return scrubBy(type === 'fastForward' ? 30 : -30)
      case 'next':
        return playNext()
      case 'previous':
        return restartEpisode()
    }

    // Tlačítka lišty ovládá nativní fokus; stisk jen prodlouží lištu.
    if (zone.current === 'buttons' && chromeVisibleRef.current) return poke()

    // Z nabídky dalšího dílu šipky vrací na posuvník (OK ji spustí samo).
    if (zone.current === 'upnext') {
      if (!['left', 'right', 'up', 'longLeft', 'longRight'].includes(type)) return
      focusSeek()
    }

    switch (type) {
      case 'left':
      case 'right':
        // Každý stisk přetočí o stejný kus; skočí se po chvíli bez stisku.
        return scrubBy(type === 'right' ? SEEK_STEP : -SEEK_STEP)
      case 'longLeft':
      case 'longRight': {
        // Držení: react-native-tvos pošle long* jen jednou při začátku (0,
        // asi po 300 ms) a při puštění (1); opakování dělá přehrávač sám.
        // Posuvník jede plynule a zrychluje s délkou držení, video skočí
        // hned po puštění.
        stopHold()
        if (e.eventKeyAction === 1) return commitScrub(scrubRef.current)
        const dir = type === 'longRight' ? 1 : -1
        const started = Date.now()
        scrubBy(dir * holdStep(0))
        hold.current = setInterval(() => {
          // Pojistka: kdyby se puštění klávesy ztratilo, nepřetáčet donekonečna
          // (potvrzení pak přijde samo po SCRUB_COMMIT_MS).
          if (Date.now() - started > 30_000) return stopHold()
          scrubBy(dir * holdStep(Date.now() - started))
        }, HOLD_TICK_MS)
        return
      }
      case 'select':
        // Skrytá lišta: první OK ji jen ukáže.
        return chromeVisibleRef.current ? togglePlay() : poke()
      case 'down':
        if (!chromeVisibleRef.current) setFocusButtons(true)
        return poke()
      case 'up':
        return poke()
    }
  }
  useTVEventHandler((e) => handler.current(e))

  // Nabídka dalšího dílu v poslední minutě (když lišta nepřekáží).
  const showUpNext =
    !!nextEpisode?.media_id &&
    phase === 'playing' &&
    duration > 0 &&
    duration - time <= 60 &&
    !chromeVisible &&
    !upNextDismissed &&
    !ended &&
    !panel &&
    countdown === null &&
    scrubTo === undefined
  useEffect(() => {
    if (showUpNext) {
      if (!upNextWarmed.current && nextEpisode?.media_id) {
        upNextWarmed.current = true
        prefetchEpisode(nextEpisode.id)
        prefetchPlayback(nextEpisode.media_id)
      }
    } else if (zone.current === 'upnext') {
      // Nabídka zmizela i s fokusem – vrátit ho, jinak by ovladač „oněměl“.
      focusSeek()
    }
  }, [focusSeek, nextEpisode, prefetchEpisode, prefetchPlayback, showUpNext])

  useEffect(() => {
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      if (panel) {
        setPanel(false)
        return true
      }
      if (scrubTo !== undefined) {
        commitScrub(undefined)
        return true
      }
      if (zone.current === 'upnext') {
        setUpNextDismissed(true)
        focusSeek()
        return true
      }
      // Viditelná lišta se nejdřív skryje; při pauze je vidět pořád, tam Zpět odchází.
      if (chromeAt > 0 && !paused) {
        hideChrome()
        return true
      }
      return false
    })
    return () => sub.remove()
  }, [chromeAt, commitScrub, focusSeek, hideChrome, panel, paused, scrubTo])

  const changeAudio = (index: number) => {
    setPanel(false)
    seekTo.current = position()
    setAudio(index)
  }

  // ---- poměr stran ----

  // Ruční volba platí hned; undefined = podle rozhodnutí ze serveru.
  const [aspectChoice, setAspectChoice] = useState<AspectRatio | null | undefined>()
  const aspect = aspectChoice !== undefined ? aspectChoice : (pb?.aspect_ratio ?? null)
  const win = useWindowDimensions()
  const changeAspect = (ratio: AspectRatio | null) => {
    setPanel(false)
    setAspectChoice(ratio)
    // Patří celému titulu: filmu, nebo seriálu (i jeho dalším dílům).
    const showId = episode.data?.episode.show_id
    const target = current.movieId ? (['movie', current.movieId] as const) : showId ? (['show', showId] as const) : undefined
    if (!target) return
    void setAspectRatio(target[0], target[1], ratio)
      .then(() => {
        // Předem načtená rozhodnutí jiných dílů nesou starý poměr. Aktuální
        // se nesmí obnovit – nový objekt by znovu otevřel stream.
        qc.removeQueries({ queryKey: ['sceniter', 'playback'], predicate: (q) => q.queryKey[2] !== mediaId })
      })
      // Neuložilo se – pro toto přehrání volba platí dál.
      .catch(() => undefined)
  }

  // ---- vykreslení ----

  const error = failure ?? (playback.isError ? playback.error.message : null)

  return (
    <View style={styles.root}>
      <VideoView
        player={player}
        style={aspect ? aspectBox(win, aspect) : StyleSheet.absoluteFill}
        nativeControls={false}
        contentFit={aspect ? 'fill' : 'contain'}
      />

      <SubtitleOverlay cues={cues} time={time} size={prefs.subtitleSize} raised={chromeVisible || panel} />

      {phase === 'playing' && !error ? (
        <Chrome
          ref={seekRef}
          title={params.title}
          subtitle={current.subtitle}
          time={time}
          duration={duration || pb?.duration_sec || 0}
          available={available}
          readyFrom={readyFrom}
          scrubTo={scrubTo}
          paused={paused}
          visible={chromeVisible}
          nextLabel={nextEpisode?.media_id ? episodeShort(nextEpisode) : undefined}
          focusButtons={focusButtons}
          onSeekFocus={() => (zone.current = 'seek')}
          onButtonFocus={() => {
            zone.current = 'buttons'
            setFocusButtons(false)
            poke()
          }}
          onTracks={() => setPanel(true)}
          onRestart={restartEpisode}
          onNext={playNext}
          onShow={episode.data ? openShow : undefined}
        />
      ) : null}

      {showUpNext && nextEpisode ? (
        <UpNextPill label={episodeShort(nextEpisode)} onPlay={playNext} onFocus={() => (zone.current = 'upnext')} />
      ) : null}

      {panel && pb ? (
        <TracksPanel
          pb={pb}
          subtitleKey={subtitleKey ?? ''}
          aspect={aspect}
          onAspect={changeAspect}
          onAudio={changeAudio}
          onSubtitle={(key) => {
            setSubtitleKey(key)
            setPanel(false)
          }}
        />
      ) : null}

      {phase !== 'playing' && !error ? (
        <View style={styles.center}>
          <ActivityIndicator size="large" color={colors.text} />
          <T dim style={{ marginTop: px(24) }}>
            {t(phase === 'resuming' ? 'video.player.resuming' : phase === 'seeking' ? 'video.tv.player.seeking' : 'video.player.loading')}
          </T>
        </View>
      ) : null}

      {error ? (
        <View style={[styles.center, styles.scrim]}>
          <T variant="section" style={{ textAlign: 'center', maxWidth: px(1100) }}>
            {error}
          </T>
          <View style={styles.errorActions}>
            <Button
              primary
              hasTVPreferredFocus
              icon={RotateCcwIcon}
              label={t('video.player.retry')}
              onPress={() => {
                setFailure(null)
                restart()
              }}
            />
            <Button icon={ArrowLeftIcon} label={t('video.player.back')} onPress={() => router.back()} />
          </View>
        </View>
      ) : null}

      {countdown !== null && nextEpisode ? (
        <NextUp label={episodeShort(nextEpisode)} seconds={countdown} onPlay={playNext} onCancel={() => router.back()} />
      ) : null}
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#000' },
  center: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center' },
  scrim: { backgroundColor: 'rgba(0,0,0,0.75)' },
  errorActions: { flexDirection: 'row', gap: px(24), marginTop: px(40) },
})
