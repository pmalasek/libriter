import { useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon, AudioLinesIcon, CaptionsIcon, InfoIcon, LoaderIcon, SkipForwardIcon } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { languageName, trackLabel, type Playback, type PlaybackMode } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from 'core-web/src/components/ui/dropdown-menu'
import { cn } from 'core-web/src/lib/utils'
import { saveProgress, usePlayback } from '@/api/hooks'
import { detectProfile, deviceId, hasNativeHls } from './profile'

/** Jak často se během přehrávání ukládá pozice. */
const SAVE_INTERVAL_MS = 10_000
/** Větší skok v čase mezi dvěma timeupdate je přetočení, ne sledování. */
const MAX_TIME_GAP = 3
/** Odpočet před automatickým spuštěním dalšího dílu. */
const NEXT_COUNTDOWN = 10
/** Pořadí režimů: když přehrání selže, zkusí se dražší. */
const ESCALATION: PlaybackMode[] = ['direct', 'remux', 'transcode']

type HlsInstance = import('hls.js').default

export interface NextUp {
  label: string
  onPlay: () => void
}

export function VideoPlayer({
  mediaId,
  movieId,
  episodeId,
  title,
  subtitle,
  startAt,
  onBack,
  next,
}: {
  mediaId: string
  movieId?: string
  episodeId?: string
  title: string
  subtitle?: string
  /** Pozice, od které pokračovat (s). */
  startAt?: number
  onBack: () => void
  next?: NextUp
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [profile] = useState(detectProfile)
  const [audio, setAudio] = useState<number | undefined>()
  const [mode, setMode] = useState<PlaybackMode | undefined>()
  const [subtitleKey, setSubtitleKey] = useState('')
  const [failure, setFailure] = useState<string | null>(null)
  const [waiting, setWaiting] = useState(true)
  const [chrome, setChrome] = useState(true)
  const [countdown, setCountdown] = useState<number | null>(null)

  const playback = usePlayback(mediaId, profile, audio, mode)
  const pb = playback.data

  const videoRef = useRef<HTMLVideoElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const seekTo = useRef<number | undefined>(startAt)
  const lastTime = useRef<number | null>(null)
  const watched = useRef(0)
  const lastSave = useRef(0)
  // Aktuální rozhodnutí pro obsluhy událostí (ukládání pozice, eskalace).
  const pbRef = useRef<Playback | undefined>(pb)
  useEffect(() => {
    pbRef.current = pb
  }, [pb])

  // ---- ukládání pozice ----

  const save = useCallback(
    (opts: { keepalive?: boolean; completed?: boolean } = {}) => {
      const video = videoRef.current
      const current = pbRef.current
      if (!video || !current || !Number.isFinite(video.currentTime)) return
      const secs = Math.floor(watched.current)
      watched.current -= secs
      lastSave.current = Date.now()
      const duration = current.duration_sec || (Number.isFinite(video.duration) ? video.duration : undefined)
      void saveProgress(
        {
          movie_id: movieId,
          episode_id: episodeId,
          media_id: mediaId,
          position_sec: Math.max(0, video.currentTime),
          duration_sec: duration,
          watched_seconds: secs,
          completed: opts.completed,
          device_id: deviceId(),
          recorded_at: new Date().toISOString(),
        },
        opts.keepalive,
      ).catch(() => {
        // Pozice se uloží při dalším pokusu; přehrávání kvůli tomu nepřerušujeme.
      })
    },
    [episodeId, mediaId, movieId],
  )

  useEffect(() => {
    const onHide = () => {
      if (document.visibilityState === 'hidden') save({ keepalive: true })
    }
    document.addEventListener('visibilitychange', onHide)
    window.addEventListener('pagehide', onHide)
    return () => {
      document.removeEventListener('visibilitychange', onHide)
      window.removeEventListener('pagehide', onHide)
      save({ keepalive: true })
      void qc.invalidateQueries({ queryKey: ['sceniter'] })
    }
  }, [qc, save])

  // ---- napojení zdroje (přímo / nativní HLS / hls.js) ----

  const escalate = useCallback(
    (from: PlaybackMode) => {
      const i = ESCALATION.indexOf(from === 'transcode_audio' ? 'remux' : from)
      const nextMode = ESCALATION[i + 1]
      if (!nextMode || !pbRef.current?.transcoding_available) {
        setFailure(t('video.player.failed'))
        return
      }
      seekTo.current = videoRef.current?.currentTime || seekTo.current
      setMode(nextMode)
    },
    [t],
  )

  useEffect(() => {
    const video = videoRef.current
    if (!video || !pb) return
    let hls: HlsInstance | undefined
    let cancelled = false
    let stopUrl: string | undefined
    setFailure(null)
    setWaiting(true)

    const onMeta = () => {
      const at = seekTo.current
      if (at && at > 0 && at < (pb.duration_sec ?? Infinity) - 5) video.currentTime = at
      seekTo.current = undefined
      void video.play().catch(() => {
        // Automatické přehrání může prohlížeč zakázat – uživatel klikne sám.
      })
    }
    const onError = () => {
      if (cancelled || hls) return
      escalate(pb.mode)
    }
    video.addEventListener('loadedmetadata', onMeta, { once: true })
    video.addEventListener('error', onError)

    if (pb.hls && !(profile === 'safari' && hasNativeHls(video))) {
      void import('hls.js').then(({ default: Hls }) => {
        if (cancelled) return
        if (!Hls.isSupported()) {
          video.src = pb.url
          return
        }
        const h = new Hls({ maxBufferLength: 30, backBufferLength: 90, manifestLoadingMaxRetry: 2 })
        hls = h
        let mediaRecovered = false
        h.on(Hls.Events.MANIFEST_PARSED, () => {
          stopUrl = h.levels[0]?.uri?.replace(/\/index\.m3u8\?/, '/stop?')
        })
        h.on(Hls.Events.ERROR, (_e, data) => {
          if (!data.fatal) return
          const status = data.response?.code
          if (status === 503 && data.details === Hls.ErrorDetails.MANIFEST_LOAD_ERROR) {
            setFailure(t('video.player.busy'))
            return
          }
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !mediaRecovered) {
            mediaRecovered = true
            h.recoverMediaError()
            return
          }
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
            escalate(pb.mode)
            return
          }
          setFailure(t('video.player.failed'))
        })
        h.loadSource(pb.url)
        h.attachMedia(video)
      })
    } else {
      video.src = pb.url
    }

    return () => {
      cancelled = true
      video.removeEventListener('loadedmetadata', onMeta)
      video.removeEventListener('error', onError)
      hls?.destroy()
      // Převod na serveru skončí hned, ne až po době nečinnosti.
      if (stopUrl) navigator.sendBeacon?.(stopUrl)
      video.removeAttribute('src')
      video.load()
    }
    // Objekt rozhodnutí se mění jen s novým zdrojem (jiná stopa, režim nebo
    // nový token po „Zkusit znovu“) – pak se video napojí znovu.
  }, [pb, profile, escalate, t])

  // ---- titulky ----

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    for (const track of Array.from(video.textTracks)) {
      track.mode = track.id === subtitleKey ? 'showing' : 'disabled'
    }
  }, [subtitleKey, pb?.url])

  // ---- ovládání a stav ----

  useEffect(() => {
    if (!chrome) return
    const timer = window.setTimeout(() => {
      if (videoRef.current && !videoRef.current.paused) setChrome(false)
    }, 3000)
    return () => window.clearTimeout(timer)
  }, [chrome])

  useEffect(() => {
    if (countdown === null) return
    const timer = window.setTimeout(() => {
      if (countdown <= 1) {
        setCountdown(null)
        next?.onPlay()
      } else {
        setCountdown(countdown - 1)
      }
    }, 1000)
    return () => window.clearTimeout(timer)
  }, [countdown, next])

  function onTimeUpdate() {
    const video = videoRef.current
    if (!video) return
    const now = video.currentTime
    if (lastTime.current !== null && !video.paused) {
      const delta = now - lastTime.current
      if (delta > 0 && delta < MAX_TIME_GAP) watched.current += delta
    }
    lastTime.current = now
    if (Date.now() - lastSave.current > SAVE_INTERVAL_MS) save()
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const video = videoRef.current
    if (!video || e.target instanceof HTMLInputElement) return
    switch (e.key) {
      case ' ':
      case 'k':
        e.preventDefault()
        if (video.paused) void video.play()
        else video.pause()
        break
      case 'ArrowLeft':
        video.currentTime = Math.max(0, video.currentTime - 10)
        break
      case 'ArrowRight':
        video.currentTime = video.currentTime + 10
        break
      case 'f':
        if (document.fullscreenElement) void document.exitFullscreen()
        else void containerRef.current?.requestFullscreen()
        break
      case 'Escape':
        if (!document.fullscreenElement) onBack()
        break
    }
    setChrome(true)
  }

  const audioTracks = pb?.audio ?? []
  const error = failure ?? (playback.isError ? playback.error.message : null)

  return (
    <div
      ref={containerRef}
      tabIndex={-1}
      onKeyDown={onKeyDown}
      onMouseMove={() => setChrome(true)}
      onTouchStart={() => setChrome(true)}
      className="fixed inset-0 z-50 bg-black text-white outline-none"
      // Fokus hned po otevření, ať fungují klávesové zkratky.
      autoFocus
    >
      <video
        ref={videoRef}
        className="sceniter-video size-full"
        controls
        playsInline
        crossOrigin="anonymous"
        onTimeUpdate={onTimeUpdate}
        onSeeking={() => {
          lastTime.current = null
        }}
        onPause={() => {
          save()
          setChrome(true)
        }}
        onPlaying={() => setWaiting(false)}
        onWaiting={() => setWaiting(true)}
        onCanPlay={() => setWaiting(false)}
        onEnded={() => {
          save({ completed: true })
          if (next) setCountdown(NEXT_COUNTDOWN)
        }}
      >
        {(pb?.subtitles ?? []).map((s) => (
          <track
            key={s.key + pb!.url}
            id={s.key}
            kind="subtitles"
            src={s.url}
            srcLang={s.language ?? undefined}
            label={[languageName(s.language) ?? s.key, s.title, s.forced ? t('video.detail.forced') : null].filter(Boolean).join(' · ')}
          />
        ))}
      </video>

      {/* Horní lišta: zpět, název, stopy. Schová se během přehrávání. */}
      <div
        className={cn(
          'pointer-events-none absolute inset-x-0 top-0 flex items-start gap-3 bg-gradient-to-b from-black/80 to-transparent p-4 pb-16 transition-opacity duration-300',
          chrome ? 'opacity-100' : 'opacity-0',
        )}
      >
        <Button variant="ghost" size="icon-lg" onClick={onBack} className="pointer-events-auto text-white hover:bg-white/15 hover:text-white" aria-label={t('video.player.back')}>
          <ArrowLeftIcon className="size-5" />
        </Button>
        <div className="min-w-0 flex-1 pt-1">
          <p className="font-heading truncate text-lg font-semibold">{title}</p>
          {subtitle ? <p className="truncate text-sm text-white/70">{subtitle}</p> : null}
        </div>

        <div className="pointer-events-auto flex gap-1">
          {audioTracks.length > 1 ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-lg" className="text-white hover:bg-white/15 hover:text-white" aria-label={t('video.player.audio')} title={t('video.player.audio')}>
                  <AudioLinesIcon className="size-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{t('video.player.audio')}</DropdownMenuLabel>
                <DropdownMenuRadioGroup
                  value={String(pb?.audio_index ?? '')}
                  onValueChange={(v) => {
                    seekTo.current = videoRef.current?.currentTime
                    setAudio(Number(v))
                  }}
                >
                  {audioTracks.map((s) => (
                    <DropdownMenuRadioItem key={s.index} value={String(s.index)}>
                      {trackLabel(s)}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}

          {(pb?.subtitles ?? []).length > 0 ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-lg" className={cn('text-white hover:bg-white/15 hover:text-white', subtitleKey && 'bg-white/15')} aria-label={t('video.player.subtitles')} title={t('video.player.subtitles')}>
                  <CaptionsIcon className="size-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{t('video.player.subtitles')}</DropdownMenuLabel>
                <DropdownMenuRadioGroup value={subtitleKey} onValueChange={setSubtitleKey}>
                  <DropdownMenuRadioItem value="">{t('video.player.subtitlesOff')}</DropdownMenuRadioItem>
                  {pb!.subtitles.map((s) => (
                    <DropdownMenuRadioItem key={s.key} value={s.key}>
                      {[languageName(s.language) ?? t('video.detail.unknownTrack', { index: s.key }), s.title, s.forced ? t('video.detail.forced') : null]
                        .filter(Boolean)
                        .join(' · ')}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
                {pb!.unsupported_subtitles.length > 0 ? (
                  <p className="max-w-56 px-2 py-1.5 text-xs text-muted-foreground">{t('video.player.unsupportedSubtitles')}</p>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}

          {pb ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-lg" className="text-white hover:bg-white/15 hover:text-white" aria-label={t('video.player.info')} title={t('video.player.info')}>
                  <InfoIcon className="size-5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="max-w-72">
                <DropdownMenuLabel>{t(`video.player.mode.${pb.mode}`)}</DropdownMenuLabel>
                {pb.reasons.map((r) => (
                  <p key={r} className="px-2 pb-1 text-xs text-muted-foreground">
                    {r}
                  </p>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </div>
      </div>

      {waiting && !error ? (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3">
          <LoaderIcon className="size-10 animate-spin opacity-80" />
          {!pb ? <p className="text-sm text-white/70">{t('video.player.loading')}</p> : null}
        </div>
      ) : null}

      {error ? (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-black/70 p-6 text-center">
          <p className="max-w-md text-lg">{error}</p>
          <div className="flex gap-2">
            <Button
              onClick={() => {
                setFailure(null)
                seekTo.current = videoRef.current?.currentTime || seekTo.current
                void playback.refetch()
              }}
            >
              {t('video.player.retry')}
            </Button>
            <Button variant="outline" onClick={onBack} className="text-foreground">
              {t('video.player.back')}
            </Button>
          </div>
        </div>
      ) : null}

      {countdown !== null && next ? (
        <div className="glass-strong absolute right-6 bottom-24 flex items-center gap-3 rounded-2xl p-3 text-foreground shadow-glass-lg ring-1 ring-glass-edge">
          <div>
            <p className="text-xs text-muted-foreground">{t('video.player.nextIn', { seconds: countdown })}</p>
            <p className="max-w-64 truncate font-medium">{next.label}</p>
          </div>
          <Button size="lg" onClick={() => next.onPlay()}>
            <SkipForwardIcon />
            {t('video.player.nextEpisode')}
          </Button>
          <Button size="lg" variant="outline" onClick={() => setCountdown(null)}>
            {t('video.player.cancel')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
