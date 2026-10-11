import { useEffect, useState } from 'react'
import { FlatList, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { useTranslation } from 'react-i18next'
import { PlayIcon } from 'lucide-react-native'
import {
  canResume,
  creditsWithRole,
  episodeCode,
  formatDuration,
  profileUrl,
  progressRatio,
  t as tr,
  type Credit,
  type EpisodeWithProgress,
  type SeasonInfo,
  type Show,
  type ShowDetail,
} from 'sceniter-shared'
import { useSeason, useShow, useSimilar } from '@/api/queries'
import { Button } from '@/components/Button'
import { PersonCard, PosterCard, WideCard } from '@/components/Cards'
import { DetailLayout } from '@/components/DetailLayout'
import { Row } from '@/components/Row'
import { ErrorView, Loading } from '@/components/States'
import { T } from '@/components/T'
import { openPerson, openShow, play } from '@/lib/nav'
import { backdropImage, episodeShort, metaLine, posterImage, wideImage } from '@/lib/titles'
import { card, px, safe } from '@/theme'

function seasonName(s: SeasonInfo): string {
  return s.title || (s.number === 0 ? tr('video.detail.specials') : tr('video.detail.season', { number: s.number }))
}

function playEpisode(show: ShowDetail, e: EpisodeWithProgress, fromStart = false) {
  if (!e.media_id) return
  play({
    mediaId: e.media_id,
    episodeId: e.id,
    title: show.title,
    subtitle: episodeShort(e),
    start: !fromStart && canResume(e.progress) ? e.progress.position_sec : 0,
  })
}

export default function ShowScreen() {
  const { id } = useLocalSearchParams<{ id: string }>()
  const { t } = useTranslation()
  const show = useShow(id)
  const similar = useSimilar('show', id)
  const s = show.data
  const [season, setSeason] = useState<number | undefined>()

  // Výchozí řada: ta, ve které je další díl, jinak první běžná (ne speciály).
  useEffect(() => {
    if (!s || season !== undefined) return
    setSeason(s.up_next?.season ?? s.seasons.find((x) => x.number > 0)?.number ?? s.seasons[0]?.number)
  }, [s, season])

  const episodes = useSeason(id, season)

  if (!s) return show.isError ? <ErrorView onRetry={() => void show.refetch()} /> : <Loading />

  const next = s.up_next
  const resumeNext = next && canResume(next.progress)
  const cast = [...creditsWithRole(s.credits, 'creator'), ...creditsWithRole(s.credits, 'cast')].slice(0, 20)
  const upNextIndex = episodes.data?.episodes.findIndex((e) => e.id === next?.id) ?? -1

  return (
    <DetailLayout
      backdrop={backdropImage(s)}
      title={s.title}
      meta={metaLine({ year: s.year, genres: s.genres, rating: s.rating, extra: t('video.shows.seasons', { count: s.season_count }) })}
      overview={s.overview}
      actions={
        next?.media_id ? (
          <Button
            primary
            hasTVPreferredFocus
            icon={PlayIcon}
            label={
              resumeNext
                ? `${t('video.home.continue')} · ${episodeCode(next.season, next.number)}`
                : t('video.detail.playEpisode', { season: next.season, episode: next.number })
            }
            onPress={() => playEpisode(s, next)}
          />
        ) : null
      }
    >
      {s.seasons.length > 1 ? (
        <TVFocusGuideView autoFocus style={styles.seasons}>
          <FlatList
            horizontal
            data={s.seasons}
            keyExtractor={(x) => x.id}
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.seasonsContent}
            renderItem={({ item }) => (
              <Button
                label={seasonName(item)}
                selected={item.number === season}
                onFocus={() => setSeason(item.number)}
                onPress={() => setSeason(item.number)}
              />
            )}
          />
        </TVFocusGuideView>
      ) : null}

      <View style={{ minHeight: card.wideH + px(160) }}>
        {episodes.data ? (
          <Row<EpisodeWithProgress>
            key={season}
            items={episodes.data.episodes}
            itemWidth={card.wideW}
            keyOf={(e) => e.id}
            render={(e, i) => (
              <View>
                <WideCard
                  title={e.title || t('video.detail.episode', { number: e.number })}
                  subtitle={[episodeCode(e.season, e.number), e.duration_sec ? formatDuration(e.duration_sec) : null, e.progress?.completed ? t('video.detail.watched') : null]
                    .filter(Boolean)
                    .join(' · ')}
                  image={wideImage(e.still_path, e.created_at) ?? wideImage(s.backdrop_path, s.updated_at)}
                  progress={e.progress?.completed ? 0 : progressRatio(e.progress, e.duration_sec)}
                  hasTVPreferredFocus={!next?.media_id && i === Math.max(0, upNextIndex)}
                  onPress={() => playEpisode(s, e)}
                />
                {e.overview ? (
                  <T variant="small" faint numberOfLines={3} style={styles.epOverview}>
                    {e.overview}
                  </T>
                ) : null}
              </View>
            )}
          />
        ) : (
          <Loading />
        )}
      </View>

      <Row<Credit>
        title={t('video.detail.castSection')}
        items={cast}
        itemWidth={px(200)}
        keyOf={(c) => c.person_id + c.role}
        render={(c) => (
          <PersonCard
            title={c.name}
            subtitle={c.role === 'cast' ? c.character : t('video.detail.creator')}
            image={profileUrl(c, px(170) * 2)}
            onPress={() => openPerson(c.person_id)}
          />
        )}
      />
      <Row<Show>
        title={t('video.detail.similar')}
        items={similar.data?.shows ?? []}
        itemWidth={card.posterW}
        keyOf={(x) => x.id}
        render={(x) => (
          <PosterCard title={x.title} subtitle={x.year ? String(x.year) : undefined} image={posterImage(x)} onPress={() => openShow(x.id)} />
        )}
      />
    </DetailLayout>
  )
}

const styles = StyleSheet.create({
  seasons: { marginBottom: px(8) },
  seasonsContent: { paddingHorizontal: safe.x, gap: px(16), paddingVertical: px(12) },
  epOverview: { width: card.wideW, marginTop: px(14) },
})
