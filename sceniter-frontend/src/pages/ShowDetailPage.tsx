import { CheckCircle2Icon, CircleIcon, PencilIcon, PlayIcon, SparklesIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'
import {
  backdropUrl,
  canEdit,
  canResume,
  formatClock,
  imageUrl,
  posterUrl,
  progressRatio,
  type EpisodeWithProgress,
} from 'sceniter-shared'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Skeleton } from 'core-web/src/components/ui/skeleton'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from 'core-web/src/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from 'core-web/src/components/ui/select'
import { useSeason, useSetWatched, useShow } from '@/api/hooks'
import { Attribution } from '@/components/Attribution'
import { CastShelf, KeyCrew } from '@/components/CastShelf'
import { DetailSkeleton, Expandable, GenreChips, SimilarShelf } from '@/components/DetailParts'
import { EditTitleDialog } from '@/components/EditTitleDialog'
import { Hero, MetaLine } from '@/components/Hero'
import { Poster } from '@/components/Poster'
import { ProgressBar } from '@/components/ProgressBar'
import { TitleActions } from '@/components/TitleActions'
import { MetadataMatch } from '@/components/MetadataMatch'
import { ExternalLinks, SourceRating } from '@/components/MetaSource'

/** Největší šířka náhledu dílu v řádku (CSS px). */
const STILL_WIDTH = 192

function EpisodeRow({ e }: { e: EpisodeWithProgress }) {
  const { t } = useTranslation()
  const setWatched = useSetWatched()
  const done = Boolean(e.progress?.completed)
  return (
    <li className="glass inset-shadow-glass flex gap-4 rounded-2xl p-3 ring-1 ring-glass-edge">
      <Link to={`/watch/episode/${e.id}`} className="group relative w-36 shrink-0 sm:w-48" aria-label={t('video.detail.play')}>
        <Poster
          src={imageUrl(e.still_path, undefined, STILL_WIDTH) ?? imageUrl(e.show_backdrop_path, undefined, STILL_WIDTH)}
          title={t('video.detail.episode', { number: e.number })}
          aspect="still"
          className="rounded-xl"
        />
        <span className="absolute inset-0 flex items-center justify-center rounded-xl bg-black/0 text-white opacity-0 transition group-hover:bg-black/30 group-hover:opacity-100">
          <PlayIcon className="size-8" />
        </span>
        <ProgressBar value={done ? 0 : progressRatio(e.progress, e.duration_sec)} className="absolute inset-x-2 bottom-2" />
      </Link>
      <div className="min-w-0 flex-1">
        <p className="font-medium">
          <span className="text-muted-foreground">{e.number}. </span>
          {e.title || t('video.detail.episode', { number: e.number })}
        </p>
        <p className="text-xs text-muted-foreground">
          {[e.air_date, e.runtime_min ? t('video.detail.runtime', { minutes: e.runtime_min }) : e.duration_sec ? formatClock(e.duration_sec) : null]
            .filter(Boolean)
            .join(' · ')}
        </p>
        {e.overview ? <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{e.overview}</p> : null}
      </div>
      <Button
        variant="ghost"
        size="icon-lg"
        title={done ? t('video.detail.markUnwatched') : t('video.detail.markWatched')}
        aria-label={done ? t('video.detail.markUnwatched') : t('video.detail.markWatched')}
        onClick={() => setWatched.mutate({ kind: 'episode', id: e.id, watched: !done }, { onError: (err) => toast.error(err.message) })}
      >
        {done ? <CheckCircle2Icon className="text-primary" /> : <CircleIcon />}
      </Button>
    </li>
  )
}

export function ShowDetailPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const show = useShow(id)
  const [season, setSeason] = useState<number | null>(null)
  const [editing, setEditing] = useState(false)
  const [matching, setMatching] = useState(false)

  const s = show.data
  const up = s?.up_next
  const current = s
    ? (s.seasons.find((x) => x.number === (season ?? up?.season ?? s.seasons[0]?.number)) ?? s.seasons[0])
    : undefined
  // Díly se načítají po řadách – seriál o stovkách dílů nepřijde najednou.
  const episodes = useSeason(id, current?.number)

  if (show.isError) return <ErrorState error={show.error} onRetry={() => void show.refetch()} />
  if (!s) return <DetailSkeleton />
  const seasonLabel = (n: number) => (n === 0 ? t('video.detail.specials') : t('video.detail.season', { number: n }))

  return (
    <div>
      <Hero backdrop={backdropUrl(s)} poster={posterUrl(s)} title={s.title}>
        <h1 className="font-heading text-3xl font-bold tracking-tight text-balance sm:text-4xl">{s.title}</h1>
        {s.original_title && s.original_title !== s.title ? <p className="mt-1 text-lg text-muted-foreground">{s.original_title}</p> : null}
        <MetaLine
          items={[
            s.year ? String(s.year) : null,
            t('video.shows.seasons', { count: s.season_count }),
            t('video.shows.episodes', { count: s.episode_count }),
          ]}
        />
        <SourceRating rating={s.rating} provider={s.meta_provider} />
        <GenreChips genres={s.genres} libraryId={s.library_id} />
        <KeyCrew credits={s.credits} role="creator" />
        <div className="mt-5 flex flex-wrap gap-2">
          {up ? (
            <Button size="lg" onClick={() => navigate(`/watch/episode/${up.id}`)}>
              <PlayIcon />
              {canResume(up.progress)
                ? t('video.detail.resume', { time: formatClock(up.progress.position_sec) })
                : t('video.detail.playEpisode', { season: up.season, episode: up.number })}
            </Button>
          ) : null}
          <TitleActions kind="show" id={s.id} user={s.user} />
          {canEdit(user) ? (
            <>
              <Button size="lg" variant="ghost" onClick={() => setEditing(true)}>
                <PencilIcon />
                {t('video.detail.edit')}
              </Button>
              <Button size="lg" variant="ghost" onClick={() => setMatching(true)}>
                <SparklesIcon />
                {t('video.detail.match')}
              </Button>
            </>
          ) : null}
        </div>
      </Hero>

      {s.match_state === 'unmatched' ? <p className="mt-6 text-sm text-muted-foreground">{t('video.detail.notMatched')}</p> : null}
      {s.overview ? <Expandable key={s.id} text={s.overview} className="mt-6" /> : null}

      {current ? (
        <section className="mt-10">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-heading text-xl font-semibold tracking-tight">{seasonLabel(current.number)}</h2>
            {s.seasons.length > 1 ? (
              <Select value={String(current.number)} onValueChange={(v) => setSeason(Number(v))}>
                <SelectTrigger className="w-44" aria-label={t('video.detail.season', { number: current.number })}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {s.seasons.map((x) => (
                    <SelectItem key={x.id} value={String(x.number)}>
                      {seasonLabel(x.number)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : null}
          </div>
          {episodes.data?.overview ? (
            <p className="mb-4 max-w-3xl text-sm text-muted-foreground">{episodes.data.overview}</p>
          ) : null}
          {episodes.isError ? <ErrorState error={episodes.error} onRetry={() => void episodes.refetch()} /> : null}
          <ul className="grid gap-3">
            {episodes.data
              ? episodes.data.episodes.map((e) => <EpisodeRow key={e.id} e={e} />)
              : Array.from({ length: Math.min(current.episode_count, 6) }, (_, i) => (
                  <Skeleton key={i} className="h-28 rounded-2xl sm:h-32" />
                ))}
          </ul>
        </section>
      ) : null}

      <CastShelf credits={s.credits} />
      <SimilarShelf kind="show" id={s.id} />
      <ExternalLinks kind="show" ids={s.external_ids} />
      <section className="mt-8">
        <h2 className="font-heading text-lg font-semibold tracking-tight">{t('video.detail.folder')}</h2>
        {/* Díly ležící naplocho mají klíč „složka/@Název" – ukážeme jen složku. */}
        <p className="mt-1 font-mono text-xs break-all text-muted-foreground">
          {s.source_key.replace(/(^|\/)@[^/]*$/, '') || '/'}
        </p>
      </section>
      <Attribution provider={s.meta_provider ?? null} />

      {editing ? <EditTitleDialog kind="show" title={s} open={editing} onOpenChange={setEditing} /> : null}
      <Dialog open={matching} onOpenChange={setMatching}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t('video.detail.match')}</DialogTitle>
          </DialogHeader>
          <MetadataMatch kind="show" id={s.id} initialQuery={s.title} initialYear={s.year} onMatched={() => setMatching(false)} />
        </DialogContent>
      </Dialog>
    </div>
  )
}
