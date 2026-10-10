import { PencilIcon, PlayIcon, RotateCcwIcon, SparklesIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { backdropUrl, canEdit, canResume, formatClock, posterUrl } from 'sceniter-shared'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { Button } from 'core-web/src/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from 'core-web/src/components/ui/dialog'
import { useMovie } from '@/api/hooks'
import { Attribution } from '@/components/Attribution'
import { Credits } from '@/components/Credits'
import { EditTitleDialog } from '@/components/EditTitleDialog'
import { FileInfo } from '@/components/FileInfo'
import { Hero, MetaLine } from '@/components/Hero'
import { TitleActions } from '@/components/TitleActions'
import { MetadataMatch } from '@/components/MetadataMatch'
import { ExternalLinks, SourceRating } from '@/components/MetaSource'

export function MovieDetailPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const movie = useMovie(id)
  const [editing, setEditing] = useState(false)
  const [matching, setMatching] = useState(false)

  if (movie.isError) return <ErrorState error={movie.error} onRetry={() => void movie.refetch()} />
  const m = movie.data
  if (!m) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>

  const progress = m.user.progress
  const resume = canResume(progress)
  const playable = Boolean(m.media_id)

  return (
    <div>
      <Hero backdrop={backdropUrl(m)} poster={posterUrl(m)} title={m.title}>
        <h1 className="font-heading text-3xl font-bold tracking-tight text-balance sm:text-4xl">{m.title}</h1>
        {m.original_title && m.original_title !== m.title ? (
          <p className="mt-1 text-lg text-muted-foreground">{m.original_title}</p>
        ) : null}
        <MetaLine
          items={[
            m.year ? String(m.year) : null,
            m.runtime_min ? t('video.detail.runtime', { minutes: m.runtime_min }) : m.duration_sec ? formatClock(m.duration_sec) : null,
            m.genres.join(', '),
          ]}
        />
        <SourceRating rating={m.rating} provider={m.meta_provider} />

        <div className="mt-5 flex flex-wrap gap-2">
          {playable ? (
            <Button size="lg" onClick={() => navigate(`/watch/movie/${m.id}`)}>
              <PlayIcon />
              {resume ? t('video.detail.resume', { time: formatClock(progress.position_sec) }) : t('video.detail.play')}
            </Button>
          ) : null}
          {playable && resume ? (
            <Button size="lg" variant="outline" onClick={() => navigate(`/watch/movie/${m.id}?from=0`)}>
              <RotateCcwIcon />
              {t('video.detail.restart')}
            </Button>
          ) : null}
          <TitleActions kind="movie" id={m.id} user={m.user} watched={Boolean(progress?.completed)} />
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

      {m.match_state === 'unmatched' ? <p className="mt-6 text-sm text-muted-foreground">{t('video.detail.notMatched')}</p> : null}
      {m.tagline ? <p className="font-heading mt-6 text-lg italic">{m.tagline}</p> : null}
      {m.overview ? <p className="mt-4 max-w-3xl leading-relaxed">{m.overview}</p> : null}

      <Credits credits={m.credits} />
      <ExternalLinks kind="movie" ids={m.external_ids} />
      <FileInfo files={m.files} />
      <Attribution provider={m.meta_provider ?? null} />

      {editing ? <EditTitleDialog kind="movie" title={m} open={editing} onOpenChange={setEditing} /> : null}
      <Dialog open={matching} onOpenChange={setMatching}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t('video.detail.match')}</DialogTitle>
          </DialogHeader>
          <MetadataMatch
            kind="movie"
            id={m.id}
            initialQuery={m.original_title || m.title}
            initialYear={m.year}
            onMatched={(newId) => {
              setMatching(false)
              if (newId !== m.id) navigate(`/movies/${newId}`, { replace: true })
            }}
          />
        </DialogContent>
      </Dialog>
    </div>
  )
}
