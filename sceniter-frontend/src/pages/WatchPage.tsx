import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { canResume, episodeLabel } from 'sceniter-shared'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { useEpisode, useMovie } from '@/api/hooks'
import { VideoPlayer } from '@/player/VideoPlayer'

/**
 * Celoobrazovkové přehrávání filmu nebo dílu: /watch/movie/:id,
 * /watch/episode/:id. ?from=0 začne od začátku místo od uložené pozice.
 */
export function WatchPage() {
  const { kind, id = '' } = useParams()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const fromStart = params.get('from') === '0'

  if (kind === 'episode') return <WatchEpisode key={id} id={id} fromStart={fromStart} onBack={(to) => navigate(to)} />
  return <WatchMovie key={id} id={id} fromStart={fromStart} onBack={(to) => navigate(to)} />
}

/**
 * Pozice, od které se pokračuje, musí být čerstvá ze serveru – v cache může
 * být stará (díl mezitím rozkoukaný na jiném zařízení). Dokud nedorazí data
 * načtená po otevření přehrávače, čeká se.
 */
function useFresh(dataUpdatedAt: number): boolean {
  const [openedAt] = useState(Date.now)
  return dataUpdatedAt >= openedAt
}

function Loading() {
  const { t } = useTranslation()
  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-black text-white/70">{t('video.player.loading')}</div>
}

function WatchMovie({ id, fromStart, onBack }: { id: string; fromStart: boolean; onBack: (to: string) => void }) {
  const { t } = useTranslation()
  const movie = useMovie(id, { fresh: true })
  const fresh = useFresh(movie.dataUpdatedAt)
  if (movie.isError) return <ErrorState error={movie.error} onRetry={() => void movie.refetch()} />
  const m = movie.data
  if (!m || !fresh) return <Loading />
  if (!m.media_id) return <ErrorState error={new Error(t('video.detail.noFile'))} />
  const p = m.user.progress
  return (
    <VideoPlayer
      mediaId={m.media_id}
      movieId={m.id}
      title={m.title}
      subtitle={m.year ? String(m.year) : undefined}
      startAt={!fromStart && canResume(p) ? p.position_sec : undefined}
      onBack={() => onBack(`/movies/${m.id}`)}
    />
  )
}

function WatchEpisode({ id, fromStart, onBack }: { id: string; fromStart: boolean; onBack: (to: string) => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const episode = useEpisode(id, { fresh: true })
  const fresh = useFresh(episode.dataUpdatedAt)
  if (episode.isError) return <ErrorState error={episode.error} onRetry={() => void episode.refetch()} />
  const d = episode.data
  if (!d || !fresh) return <Loading />
  const file = d.files[0]
  if (!file) return <ErrorState error={new Error(t('video.detail.noFile'))} />
  const p = d.progress ?? undefined
  const next = d.next
  return (
    <VideoPlayer
      mediaId={file.id}
      episodeId={d.episode.id}
      showId={d.show.id}
      title={d.show.title}
      subtitle={episodeLabel(d.episode)}
      startAt={!fromStart && canResume(p) ? p.position_sec : undefined}
      onBack={() => onBack(`/shows/${d.show.id}`)}
      next={next ? { label: episodeLabel(next), onPlay: () => navigate(`/watch/episode/${next.id}`, { replace: true }) } : undefined}
    />
  )
}
