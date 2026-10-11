import { useEffect, useRef, useState } from 'react'
import { ActivityIndicator, Dimensions, FlatList, StyleSheet, TVFocusGuideView, View } from 'react-native'
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
import { usePrefetchPlayback, useSeason, useShow, useSimilar } from '@/api/queries'
import { Button } from '@/components/Button'
import { PersonCard, PosterCard, WideCard } from '@/components/Cards'
import { DetailLayout } from '@/components/DetailLayout'
import { Row } from '@/components/Row'
import { ErrorView, Loading } from '@/components/States'
import { T } from '@/components/T'
import { openPerson, openShow, play } from '@/lib/nav'
import { useCachedTitle } from '@/lib/summaryCache'
import { backdropImage, episodeShort, metaLine, posterImage, wideImage } from '@/lib/titles'
import { card, colors, px, safe } from '@/theme'

/** Mřížka dílů: čtyři sloupce širokých karet přes celou šířku mezi bezpečnými okraji. */
const EP_COLUMNS = 4
const EP_W = Math.floor((Dimensions.get('window').width - safe.x * 2 - card.gap * (EP_COLUMNS - 1)) / EP_COLUMNS)

function chunk<T>(xs: T[], n: number): T[][] {
  const rows: T[][] = []
  for (let i = 0; i < xs.length; i += n) rows.push(xs.slice(i, i + n))
  return rows
}

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
    fromStart,
  })
}

export default function ShowScreen() {
  // episode/season: z přehrávače – otevřít řadu a zaměřit hraný díl.
  const { id, episode: episodeParam, season: seasonParam } = useLocalSearchParams<{ id: string; episode?: string; season?: string }>()
  const { t } = useTranslation()
  const show = useShow(id)
  const similar = useSimilar('show', id)
  const cached = useCachedTitle('show', id)
  const prefetchPlayback = usePrefetchPlayback()
  const s = show.data
  const [season, setSeason] = useState<number | undefined>(() => (seasonParam ? Number(seasonParam) : undefined))
  // Díl, který má dostat fokus (jen jednou – po zaměření se zapomene).
  const [focusEpisode, setFocusEpisode] = useState(episodeParam)
  useEffect(() => {
    if (!episodeParam) return
    setFocusEpisode(episodeParam)
    if (seasonParam) setSeason(Number(seasonParam))
  }, [episodeParam, seasonParam])
  // Přejetí fokusem přes řady přepne až po chvíli klidu – jinak by každá
  // řada, přes kterou fokus jen projel, spustila načtení a překreslení.
  const seasonTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(seasonTimer.current), [])
  const pickSeason = (n: number, now = false) => {
    clearTimeout(seasonTimer.current)
    if (now) setSeason(n)
    else seasonTimer.current = setTimeout(() => setSeason(n), 250)
  }

  // Výchozí řada: ta, ve které je další díl, jinak první běžná (ne speciály).
  useEffect(() => {
    if (!s || season !== undefined) return
    setSeason(s.up_next?.season ?? s.seasons.find((x) => x.number > 0)?.number ?? s.seasons[0]?.number)
  }, [s, season])

  const episodes = useSeason(id, season)

  if (!s) {
    if (show.isError || !cached) return show.isError ? <ErrorView onRetry={() => void show.refetch()} /> : <Loading />
    // Než dojde detail: pozadí a název ze seznamu, ze kterého se přišlo.
    return (
      <DetailLayout
        backdrop={backdropImage(cached)}
        title={cached.title}
        meta={metaLine({ year: cached.year, rating: cached.rating })}
        overview={cached.overview}
        actions={<ActivityIndicator size="large" color={colors.textDim} />}
      />
    )
  }

  const next = s.up_next
  const resumeNext = next && canResume(next.progress)
  const cast = [...creditsWithRole(s.credits, 'creator'), ...creditsWithRole(s.credits, 'cast')].slice(0, 20)
  const upNextIndex = episodes.data?.episodes.findIndex((e) => e.id === next?.id) ?? -1
  const currentSeason = s.seasons.find((x) => x.number === season)

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
            onFocus={() => prefetchPlayback(next.media_id)}
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
                onFocus={() => pickSeason(item.number)}
                onPress={() => pickSeason(item.number, true)}
              />
            )}
          />
        </TVFocusGuideView>
      ) : null}

      <View style={[styles.episodes, { opacity: episodes.isPlaceholderData ? 0.6 : 1 }]}>
        {currentSeason ? (
          <T variant="section" style={styles.episodesTitle}>
            {`${seasonName(currentSeason)}  ·  ${t('video.shows.episodes', { count: currentSeason.episode_count })}`}
          </T>
        ) : null}
        {episodes.data ? (
          // Obyčejné řádky ve ScrollView detailu – seriál o stovce dílů se
          // posouvá svisle a šipky nahoru/dolů jdou po řádcích.
          <TVFocusGuideView autoFocus key={episodes.data.number}>
            {chunk(episodes.data.episodes, EP_COLUMNS).map((row, r) => (
              <View key={r} style={styles.epRow}>
                {row.map((e, c) => {
                  const i = r * EP_COLUMNS + c
                  const preferred = focusEpisode ? e.id === focusEpisode : !next?.media_id && i === Math.max(0, upNextIndex)
                  return (
                    <View key={e.id} style={{ width: EP_W }}>
                      <WideCard
                        width={EP_W}
                        title={e.title || t('video.detail.episode', { number: e.number })}
                        subtitle={[episodeCode(e.season, e.number), e.duration_sec ? formatDuration(e.duration_sec) : null, e.progress?.completed ? t('video.detail.watched') : null]
                          .filter(Boolean)
                          .join(' · ')}
                        image={wideImage(e.still_path, e.created_at) ?? wideImage(s.backdrop_path, s.updated_at)}
                        progress={e.progress?.completed ? 0 : progressRatio(e.progress, e.duration_sec)}
                        hasTVPreferredFocus={preferred}
                        onFocus={() => {
                          if (focusEpisode && e.id === focusEpisode) setFocusEpisode(undefined)
                          prefetchPlayback(e.media_id)
                        }}
                        onPress={() => playEpisode(s, e)}
                      />
                      {e.overview ? (
                        <T variant="small" faint numberOfLines={2} style={styles.epOverview}>
                          {e.overview}
                        </T>
                      ) : null}
                    </View>
                  )
                })}
              </View>
            ))}
          </TVFocusGuideView>
        ) : (
          <View style={{ minHeight: px(400) }}>
            <Loading />
          </View>
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
  episodes: { paddingHorizontal: safe.x, marginBottom: px(44) },
  episodesTitle: { marginBottom: px(18) },
  // Svislá mezera i na zvětšenou zaměřenou kartu.
  epRow: { flexDirection: 'row', gap: card.gap, paddingVertical: px(16) },
  epOverview: { marginTop: px(14) },
})
