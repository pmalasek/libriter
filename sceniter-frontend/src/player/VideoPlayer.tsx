import { useQueryClient } from '@tanstack/react-query'
import { ArrowLeftIcon, AudioLinesIcon, CaptionsIcon, InfoIcon, LoaderIcon, SkipForwardIcon } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  escalateMode,
  keepaliveUrl,
  languageName,
  shouldSeekTo,
  stopUrlFromVariant,
  trackLabel,
  WatchMeter,
  KEEPALIVE_MS,
  NEXT_COUNTDOWN,
  RESUME_AHEAD,
  SAVE_INTERVAL_MS,
  type Playback,
  type PlaybackMode,
} from 'sceniter-shared'
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

type HlsInstance = import('hls.js').default

/** Konec části videa, na kterou jde skočit (u rostoucího playlistu roste). */
function seekableEnd(video: HTMLVideoElement): number {
  const r = video.seekable
  if (r.length > 0) return r.end(r.length - 1)
  return Number.isFinite(video.duration) ? video.duration : 0
}

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
  // Čeká se, až bude místo, odkud se pokračuje, k dispozici (rostoucí
  // playlist přebalení). Nativní ovládání v tu chvíli točítko neukazuje.
  const [resuming, setResuming] = useState(false)
  const [chrome, setChrome] = useState(true)
  const [countdown, setCountdown] = useState<number | null>(null)

  const playback = usePlayback(mediaId, profile, audio, mode)
  const pb = playback.data

  const videoRef = useRef<HTMLVideoElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const seekTo = useRef<number | undefined>(startAt)
  const meter = useRef(new WatchMeter())
  const lastSave = useRef(0)
  // Aktuální rozhodnutí pro obsluhy událostí (ukládání pozice, eskalace).
  const pbRef = useRef<Playback | undefined>(pb)
  useEffect(() => {
    pbRef.current = pb
  }, [pb])

  // ---- ukládání pozice ----

  const save = useCallback(
    (opts: { keepalive?: boolean; completed?: boolean } = {}): Promise<void> => {
      const video = videoRef.current
      const current = pbRef.current
      if (!video || !current || !Number.isFinite(video.currentTime)) return Promise.resolve()
      const secs = meter.current.take()
      lastSave.current = Date.now()
      const duration = current.duration_sec || (Number.isFinite(video.duration) ? video.duration : undefined)
      return saveProgress(
        {
          movie_id: movieId,
          episode_id: episodeId,
          media_id: mediaId,
          // Dokud se na uloženou pozici neskočilo, video stojí na začátku –
          // uložila by se nula místo místa, kde divák skončil.
          position_sec: Math.max(0, seekTo.current ?? video.currentTime),
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
      if (document.visibilityState === 'hidden') void save({ keepalive: true })
    }
    document.addEventListener('visibilitychange', onHide)
    window.addEventListener('pagehide', onHide)
    return () => {
      document.removeEventListener('visibilitychange', onHide)
      window.removeEventListener('pagehide', onHide)
      // Až po uložení, jinak by detail mohl načíst ještě starou pozici.
      void save({ keepalive: true }).finally(() => qc.invalidateQueries({ queryKey: ['sceniter'] }))
    }
  }, [qc, save])

  // ---- napojení zdroje (přímo / nativní HLS / hls.js) ----

  // Převod na serveru zmizel (úklid, restart serveru): nový token a nový
  // převod, pokračuje se od aktuální pozice.
  const refetchPlayback = playback.refetch
  const restart = useCallback(() => {
    seekTo.current = videoRef.current?.currentTime || seekTo.current
    void refetchPlayback()
  }, [refetchPlayback])

  const escalate = useCallback(
    (from: PlaybackMode) => {
      const nextMode = escalateMode(from, pbRef.current?.transcoding_available ?? false)
      if (!nextMode) {
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
    let resumeTimer: number | undefined
    setFailure(null)
    setResuming(false)

    const play = () => {
      void video.play().catch(() => {
        // Automatické přehrání může prohlížeč zakázat – uživatel klikne sám.
      })
    }
    const onMeta = () => {
      const at = seekTo.current
      if (!shouldSeekTo(at, pb.duration_sec)) {
        seekTo.current = undefined
        play()
        return
      }
      // Přebalení a převod zvuku servírují playlist, který teprve roste:
      // skok za jeho konec by prohlížeč usekl na konec už hotové části.
      // Počká se proto, až převod k místu dojede (kopírování je rychlé).
      const seekWhenReady = () => {
        if (cancelled) return
        if (seekableEnd(video) < at + RESUME_AHEAD) {
          setResuming(true)
          resumeTimer = window.setTimeout(seekWhenReady, 500)
          return
        }
        video.currentTime = at
        seekTo.current = undefined
        setResuming(false)
        play()
      }
      seekWhenReady()
    }
    const onError = () => {
      if (cancelled || hls) return
      escalate(pb.mode)
    }
    video.addEventListener('loadedmetadata', onMeta, { once: true })
    video.addEventListener('error', onError)

    // S plným bufferem nebo v pauze přehrávač segmenty nežádá a server by
    // převod po době nečinnosti smazal – proto pravidelný keepalive.
    let keepalive: number | undefined
    if (pb.hls) {
      const keepUrl = keepaliveUrl(pb.url)
      keepalive = window.setInterval(() => {
        void fetch(keepUrl, { method: 'POST' })
          .then((res) => {
            if (res.status === 404 && !cancelled) restart()
          })
          .catch(() => {
            // Výpadek sítě – zkusí se při dalším tiku.
          })
      }, KEEPALIVE_MS)
    }

    if (pb.hls && !(profile === 'safari' && hasNativeHls(video))) {
      void import('hls.js').then(({ default: Hls }) => {
        if (cancelled) return
        if (!Hls.isSupported()) {
          video.src = pb.url
          return
        }
        // Rostoucí playlist (EVENT bez ENDLIST) bere hls.js jako živé vysílání
        // a začal by u jeho konce – startPosition 0 ho drží na začátku.
        const h = new Hls({ maxBufferLength: 30, backBufferLength: 90, manifestLoadingMaxRetry: 2, startPosition: 0 })
        hls = h
        let mediaRecovered = false
        let restarted = false
        h.on(Hls.Events.MANIFEST_PARSED, () => {
          stopUrl = stopUrlFromVariant(h.levels[0]?.uri)
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
          if (status === 404 && !restarted) {
            restarted = true
            restart()
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
      window.clearInterval(keepalive)
      window.clearTimeout(resumeTimer)
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
  }, [pb, profile, escalate, restart, t])

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
    meter.current.tick(video.currentTime, !video.paused)
    if (Date.now() - lastSave.current > SAVE_INTERVAL_MS) void save()
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
        onSeeking={() => meter.current.reset()}
        onPause={() => {
          void save()
          setChrome(true)
        }}
        onEnded={() => {
          void save({ completed: true })
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

      {/* Při načítání videa ukazuje točítko nativní ovládání; vlastní jen
          dokud video nemá zdroj nebo se čeká na místo pokračování. */}
      {(!pb || resuming) && !error ? (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3">
          <LoaderIcon className="size-10 animate-spin opacity-80" />
          <p className="text-sm text-white/70">{t(resuming ? 'video.player.resuming' : 'video.player.loading')}</p>
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
