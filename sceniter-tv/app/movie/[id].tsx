import { ActivityIndicator } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { PlayIcon, RotateCcwIcon } from 'lucide-react-native'
import { canResume, creditsWithRole, formatClock, profileUrl, type Credit, type Movie } from 'sceniter-shared'
import { usePrefetchPlayback, useMovie, useSimilar } from '@/api/queries'
import { Button } from '@/components/Button'
import { PersonCard, PosterCard } from '@/components/Cards'
import { DetailLayout } from '@/components/DetailLayout'
import { Row } from '@/components/Row'
import { ErrorView, Loading } from '@/components/States'
import { openMovie, openPerson, play } from '@/lib/nav'
import { useCachedTitle } from '@/lib/summaryCache'
import { backdropImage, metaLine, posterImage } from '@/lib/titles'
import { card, colors, px } from '@/theme'

/** Hlavní soubor filmu – nejvyšší rozlišení (stejně volí server media_id). */
function techLine(m: NonNullable<ReturnType<typeof useMovie>['data']>): string | undefined {
  const f = m.files.find((x) => x.id === m.media_id) ?? m.files[0]
  if (!f) return undefined
  const res = f.height ? (f.height >= 2000 ? '4K' : f.height >= 1000 ? '1080p' : f.height >= 700 ? '720p' : `${f.height}p`) : undefined
  const hdr = f.pix_fmt?.includes('10') && f.video_codec === 'hevc' ? 'HDR' : undefined
  const audio = f.streams.filter((s) => s.kind === 'audio').length
  return [res, hdr, audio > 1 ? `${audio}× audio` : undefined].filter(Boolean).join(' · ') || undefined
}

export default function MovieScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const { t } = useTranslation()
  const movie = useMovie(id)
  const similar = useSimilar('movie', id)
  const cached = useCachedTitle('movie', id)
  const prefetchPlayback = usePrefetchPlayback()

  const m = movie.data
  // Než dojde detail, stačí na pozadí a název to, co známe ze seznamu.
  const head = m ?? cached
  if (!head) return movie.isError ? <ErrorView onRetry={() => void movie.refetch()} /> : <Loading />
  if (!m && movie.isError) return <ErrorView onRetry={() => void movie.refetch()} />

  const progress = m?.user.progress
  const resume = canResume(progress)
  const cast = m ? [...creditsWithRole(m.credits, 'director'), ...creditsWithRole(m.credits, 'cast')].slice(0, 20) : []
  const start = (at: number) => m?.media_id && play({ mediaId: m.media_id, movieId: m.id, title: m.title, start: at })
  const warm = () => prefetchPlayback(m?.media_id)

  return (
    <DetailLayout
      backdrop={backdropImage(head)}
      title={head.title}
      eyebrow={m?.tagline}
      meta={metaLine({ ...head, extra: m ? techLine(m) : undefined })}
      overview={m?.overview ?? cached?.overview}
      actions={
        !m ? (
          <ActivityIndicator size="large" color={colors.textDim} />
        ) : m.media_id ? (
          <>
            <Button
              primary
              hasTVPreferredFocus
              icon={PlayIcon}
              label={resume ? t('video.detail.resume', { time: formatClock(progress.position_sec) }) : t('video.detail.play')}
              onFocus={warm}
              onPress={() => start(resume ? progress.position_sec : 0)}
            />
            {resume ? <Button icon={RotateCcwIcon} label={t('video.detail.restart')} onFocus={warm} onPress={() => start(0)} /> : null}
          </>
        ) : (
          <Button label={t('video.detail.noFile')} disabled hasTVPreferredFocus />
        )
      }
    >
      <Row<Credit>
        title={t('video.detail.castSection')}
        items={cast}
        itemWidth={px(200)}
        keyOf={(c) => c.person_id + c.role}
        render={(c) => (
          <PersonCard
            title={c.name}
            subtitle={c.role === 'cast' ? c.character : t('video.detail.director')}
            image={profileUrl(c, px(170) * 2)}
            onPress={() => openPerson(c.person_id)}
          />
        )}
      />
      <Row<Movie>
        title={t('video.detail.similar')}
        // Až s detailem – jinak by fokus skočil do řady a pak zpět na Přehrát.
        items={m ? similar.data?.movies ?? [] : []}
        itemWidth={card.posterW}
        keyOf={(x) => x.id}
        render={(x) => (
          <PosterCard title={x.title} subtitle={x.year ? String(x.year) : undefined} image={posterImage(x)} onPress={() => openMovie(x.id)} />
        )}
      />
    </DetailLayout>
  )
}
