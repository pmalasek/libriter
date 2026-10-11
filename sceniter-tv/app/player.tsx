import { useCallback, useEffect, useRef, useState } from 'react'
import { ActivityIndicator, AppState, BackHandler, StyleSheet, View, useTVEventHandler, type HWEvent } from 'react-native'
import { router, useLocalSearchParams } from 'expo-router'
import { useEventListener } from 'expo'
import { VideoView, useVideoPlayer } from 'expo-video'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowLeftIcon, RotateCcwIcon } from 'lucide-react-native'
import {
  escalateMode,
  languageCode,
  saveProgress,
  shouldSeekTo,
  NEXT_COUNTDOWN,
  SAVE_INTERVAL_MS,
  WatchMeter,
  type Playback,
  type PlaybackMode,
} from 'sceniter-shared'
import { useEpisode, usePrefetchEpisode, usePrefetchPlayback } from '@/api/queries'
import { Button } from '@/components/Button'
import { T } from '@/components/T'
import { usePrefs } from '@/lib/prefs'
import { deviceId } from '@/lib/storage'
import { episodeShort } from '@/lib/titles'
import { Chrome, NextUp, TracksPanel } from '@/player/Overlay'
import { playbackQueryOptions } from '@/player/playbackQuery'
import { StreamSession, TranscodeBusyError } from '@/player/session'
import { SubtitleOverlay, useCues } from '@/player/Subtitles'
import { colors, px } from '@/theme'

/** Ovládání zmizí po chvíli bez stisku, pokud video hraje. */
const CHROME_MS = 4000
/** Přetáčení šipkami se potvrdí samo po chvíli bez dalšího stisku. */
const SCRUB_COMMIT_MS = 900

type Phase = 'loading' | 'resuming' | 'playing'

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

export default function PlayerScreen() {
  const params = useLocalSearchParams<{
    mediaId: string
    movieId?: string
    episodeId?: string
    title: string
    subtitle?: string
    start?: string
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
  const pendingSeek = useRef<number | undefined>(undefined)
  const [phase, setPhase] = useState<Phase>('loading')
  const phaseRef = useRef(phase)
  phaseRef.current = phase
  const [failure, setFailure] = useState<string | null>(null)

  // Poslední známá poloha – ukládání při odchodu už na uvolněný přehrávač sahat nesmí.
  const timeRef = useRef(0)
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
      if (!current) return Promise.resolve()
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
        recorded_at: new Date().toISOString(),
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
    const session = new StreamSession(pb, restart)
    setFailure(null)
    setPhase('loading')
    void (async () => {
      try {
        await session.open()
      } catch (e) {
        if (cancelled) return
        if (e instanceof TranscodeBusyError) setFailure(t('video.player.busy'))
        else escalate()
        return
      }
      const at = seekTo.current
      const resume = shouldSeekTo(at, pb.duration_sec)
      if (resume) {
        setPhase('resuming')
        await session.waitForPosition(at, () => cancelled)
      } else {
        seekTo.current = undefined
      }
      if (cancelled) return
      pendingSeek.current = resume ? at : undefined
      endedRef.current = false
      player.replace({ uri: session.url, contentType: pb.hls ? 'hls' : 'progressive' })
      player.play()
    })()
    return () => {
      cancelled = true
      session.close()
    }
  }, [pb, player, restart, escalate, t])

  useEventListener(player, 'statusChange', ({ status }) => {
    if (status === 'readyToPlay') {
      if (pendingSeek.current !== undefined) {
        player.currentTime = pendingSeek.current
        pendingSeek.current = undefined
      }
      seekTo.current = undefined
      setPhase('playing')
    } else if (status === 'error') {
      escalate()
    }
  })

  // ---- čas, pauza, konec ----

  const [time, setTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [paused, setPaused] = useState(false)
  const [countdown, setCountdown] = useState<number | null>(null)
  const [ended, setEnded] = useState(false)
  // Android hlásí konec opakovaně (při každé změně stavu po dohrání).
  const endedRef = useRef(false)

  useEventListener(player, 'timeUpdate', ({ currentTime }) => {
    timeRef.current = currentTime
    setTime(currentTime)
    // Rostoucí playlist (přebalení) hlásí jen hotovou část – platí délka ze serveru.
    const total = pbRef.current?.duration_sec || player.duration
    if (total > 0) {
      durationRef.current = total
      setDuration(total)
    }
    meter.current.tick(currentTime, player.playing)
    if (Date.now() - lastSave.current > SAVE_INTERVAL_MS) void save()
  })

  useEventListener(player, 'playingChange', ({ isPlaying }) => {
    setPaused(!isPlaying)
    if (!isPlaying) {
      void save()
      setChrome(true)
    }
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
    setCountdown(null)
    setEnded(false)
    seekTo.current = undefined
    pendingSeek.current = undefined
    timeRef.current = 0
    durationRef.current = 0
    lastSave.current = 0
    meter.current.reset()
    setTime(0)
    setDuration(0)
    setPaused(false)
    setFailure(null)
    setPhase('loading')
    setScrubTo(undefined)
    setPanel(false)
    setChrome(true)
    setSubtitleKey(null)
    setAudio(undefined)
    setMode(undefined)
    setAttempt(0)
    // Nové ID → nový dotaz na rozhodnutí → efekt výš zavře session dohraného
    // dílu a otevře další.
    setCurrent({ mediaId: next.media_id, episodeId: next.id, movieId: undefined, subtitle: episodeShort(next) })
  }, [])

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

  const [chrome, setChrome] = useState(true)
  const [panel, setPanel] = useState(false)
  const [scrubTo, setScrubTo] = useState<number | undefined>()
  const scrubTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => {
    if (!chrome || paused || panel || scrubTo !== undefined) return
    const timer = setTimeout(() => setChrome(false), CHROME_MS)
    return () => clearTimeout(timer)
  }, [chrome, paused, panel, scrubTo, time])

  const commitScrub = useCallback(
    (target: number | undefined) => {
      clearTimeout(scrubTimer.current)
      setScrubTo(undefined)
      if (target === undefined) return
      meter.current.reset()
      player.currentTime = target
    },
    [player],
  )

  const scrubBy = useCallback(
    (delta: number) => {
      setChrome(true)
      setScrubTo((prev) => {
        const max = durationRef.current > 0 ? durationRef.current - 1 : Infinity
        const next = Math.max(0, Math.min(max, (prev ?? timeRef.current) + delta))
        clearTimeout(scrubTimer.current)
        scrubTimer.current = setTimeout(() => commitScrub(next), SCRUB_COMMIT_MS)
        return next
      })
    },
    [commitScrub],
  )

  // Opakované stisky stejným směrem zrychlují (10 s → 30 s → 60 s).
  const repeat = useRef({ dir: 0, count: 0, at: 0 })
  const step = (dir: 1 | -1) => {
    const now = Date.now()
    const r = repeat.current
    r.count = r.dir === dir && now - r.at < 700 ? r.count + 1 : 0
    r.dir = dir
    r.at = now
    return dir * (r.count >= 8 ? 60 : r.count >= 4 ? 30 : 10)
  }

  const handler = useRef<(e: HWEvent) => void>(() => undefined)
  handler.current = (e: HWEvent) => {
    // Android posílá stisk i uvolnění klávesy; reaguje se na stisk.
    if (e.eventKeyAction === 1) return
    if (countdown !== null || panel || failure || phase !== 'playing') return
    switch (e.eventType) {
      case 'select':
      case 'playPause':
        setChrome(true)
        if (scrubTo !== undefined) commitScrub(scrubTo)
        else if (player.playing) player.pause()
        else player.play()
        break
      case 'left':
      case 'right':
        scrubBy(step(e.eventType === 'right' ? 1 : -1))
        break
      case 'longLeft':
      case 'longRight':
        scrubBy(e.eventType === 'longRight' ? 60 : -60)
        break
      case 'fastForward':
      case 'rewind':
        scrubBy(e.eventType === 'fastForward' ? 30 : -30)
        break
      case 'down':
        if (pb) setPanel(true)
        break
      case 'up':
        setChrome(true)
        break
    }
  }
  useTVEventHandler((e) => handler.current(e))

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
      return false
    })
    return () => sub.remove()
  }, [commitScrub, panel, scrubTo])

  const changeAudio = (index: number) => {
    setPanel(false)
    seekTo.current = position()
    setAudio(index)
  }

  // ---- vykreslení ----

  const error = failure ?? (playback.isError ? playback.error.message : null)
  const showChrome = (chrome || paused || scrubTo !== undefined) && phase === 'playing' && !panel && countdown === null

  return (
    <View style={styles.root}>
      <VideoView player={player} style={StyleSheet.absoluteFill} nativeControls={false} contentFit="contain" />

      <SubtitleOverlay cues={cues} time={time} size={prefs.subtitleSize} raised={showChrome || panel} />

      {showChrome ? (
        <Chrome title={params.title} subtitle={current.subtitle} time={time} duration={duration || pb?.duration_sec || 0} scrubTo={scrubTo} paused={paused} />
      ) : null}

      {panel && pb ? (
        <TracksPanel
          pb={pb}
          subtitleKey={subtitleKey ?? ''}
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
            {t(phase === 'resuming' ? 'video.player.resuming' : 'video.player.loading')}
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
