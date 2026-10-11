import { useLocalSearchParams } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { PlayIcon, RotateCcwIcon } from 'lucide-react-native'
import { canResume, creditsWithRole, formatClock, profileUrl, type Credit, type Movie } from 'sceniter-shared'
import { useMovie, useSimilar } from '@/api/queries'
import { Button } from '@/components/Button'
import { PersonCard, PosterCard } from '@/components/Cards'
import { DetailLayout } from '@/components/DetailLayout'
import { Row } from '@/components/Row'
import { ErrorView, Loading } from '@/components/States'
import { openMovie, openPerson, play } from '@/lib/nav'
import { backdropImage, metaLine, posterImage } from '@/lib/titles'
import { card, px } from '@/theme'

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

  const m = movie.data
  if (!m) return movie.isError ? <ErrorView onRetry={() => void movie.refetch()} /> : <Loading />

  const progress = m.user.progress
  const resume = canResume(progress)
  const cast = [...creditsWithRole(m.credits, 'director'), ...creditsWithRole(m.credits, 'cast')].slice(0, 20)
  const start = (at: number) => m.media_id && play({ mediaId: m.media_id, movieId: m.id, title: m.title, start: at })

  return (
    <DetailLayout
      backdrop={backdropImage(m)}
      title={m.title}
      eyebrow={m.tagline}
      meta={metaLine({ ...m, extra: techLine(m) })}
      overview={m.overview}
      actions={
        m.media_id ? (
          <>
            <Button
              primary
              hasTVPreferredFocus
              icon={PlayIcon}
              label={resume ? t('video.detail.resume', { time: formatClock(progress.position_sec) }) : t('video.detail.play')}
              onPress={() => start(resume ? progress.position_sec : 0)}
            />
            {resume ? <Button icon={RotateCcwIcon} label={t('video.detail.restart')} onPress={() => start(0)} /> : null}
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
        items={similar.data?.movies ?? []}
        itemWidth={card.posterW}
        keyOf={(x) => x.id}
        render={(x) => (
          <PosterCard title={x.title} subtitle={x.year ? String(x.year) : undefined} image={posterImage(x)} onPress={() => openMovie(x.id)} />
        )}
      />
    </DetailLayout>
  )
}
