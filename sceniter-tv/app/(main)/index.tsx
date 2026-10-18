import { useEffect, useRef } from 'react'
import { ScrollView, StyleSheet, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import {
  canResume,
  progressRatio,
  remainingLabel,
  t as tr,
  type ContinueItem,
  type Movie,
  type Show,
  type WatchlistItem,
} from 'sceniter-shared'
import { useDebouncedPrefetch, useHome } from '@/api/queries'
import { Backdrop } from '@/components/Backdrop'
import { PosterCard, WideCard } from '@/components/Cards'
import { Hero } from '@/components/Hero'
import { Row } from '@/components/Row'
import { NAV_COLLAPSED } from '@/components/SideNav'
import { Empty, ErrorView, Loading } from '@/components/States'
import { createFocusStore, useSettled } from '@/lib/focusStore'
import { openMovie, openShow, play } from '@/lib/nav'
import { episodeHero, episodeShort, movieHero, posterImage, showHero, wideImage, type HeroInfo } from '@/lib/titles'
import { card, px } from '@/theme'

const INSET = NAV_COLLAPSED + px(32)

function continueHero(c: ContinueItem): HeroInfo | undefined {
  if (c.kind === 'movie' && c.movie) return movieHero(c.movie)
  if (c.episode) return episodeHero(c.episode, c.show)
  return undefined
}

function playContinue(c: ContinueItem) {
  const start = !c.next && canResume(c.progress) ? c.progress.position_sec : 0
  if (c.kind === 'movie' && c.movie) {
    if (!c.movie.media_id) return openMovie(c.movie.id)
    play({ mediaId: c.movie.media_id, movieId: c.movie.id, title: c.movie.title, start })
  } else if (c.episode) {
    if (!c.episode.media_id) return openShow(c.episode.show_id)
    play({
      mediaId: c.episode.media_id,
      episodeId: c.episode.id,
      title: c.show?.title ?? c.episode.show_title ?? '',
      subtitle: episodeShort(c.episode),
      start,
    })
  }
}

function HeroArea({ store }: { store: ReturnType<typeof createFocusStore<HeroInfo>> }) {
  const info = store.use()
  const backdrop = useSettled(info?.backdrop, 220)
  return (
    <>
      <Backdrop uri={backdrop} />
      <Hero info={info} inset={INSET} />
    </>
  )
}

export default function HomeScreen() {
  const { t } = useTranslation()
  const home = useHome()
  const prefetch = useDebouncedPrefetch()
  const store = useRef(createFocusStore<HeroInfo>()).current
  // Výchozí fokus jen do prvního zaměření – obnova dat ze serveru (po datech
  // z cache) jinak vrátí fokus na první kartu uprostřed listování.
  const focusedOnce = useRef(false)

  const focus = (info: HeroInfo | undefined, prefetchTitle?: [kind: 'movie' | 'show', id: string]) => {
    focusedOnce.current = true
    if (info) store.set(info)
    if (prefetchTitle) prefetch(...prefetchTitle)
  }

  const data = home.data
  // Výchozí hero, než uživatel na něco najede.
  useEffect(() => {
    if (!data || store.get()) return
    const first =
      (data.continue[0] && continueHero(data.continue[0])) ||
      data.libraries.flatMap((l) => [...l.movies.map(movieHero), ...l.shows.map(showHero)]).find((h) => h.backdrop)
    if (first) store.set(first)
  }, [data, store])

  if (!data) {
    if (home.isError) return <ErrorView onRetry={() => void home.refetch()} />
    return <Loading />
  }
  if (data.libraries.length === 0 && data.continue.length === 0) {
    return <Empty title={t('video.home.noLibrariesTitle')} description={t('video.home.noLibrariesDescription')} />
  }

  let preferred = !focusedOnce.current
  const takePreferred = () => {
    const p = preferred
    preferred = false
    return p
  }

  return (
    <View style={styles.root}>
      <HeroArea store={store} />
      <ScrollView style={styles.rows} contentContainerStyle={styles.rowsContent} showsVerticalScrollIndicator={false}>
        <Row<ContinueItem>
          title={t('video.home.continue')}
          items={data.continue}
          inset={INSET}
          itemWidth={card.wideW}
          keyOf={(c) => `${c.kind}${c.movie?.id ?? c.episode?.id}`}
          render={(c, i) => {
            const isMovie = c.kind === 'movie'
            const ep = c.episode
            const image = isMovie
              ? wideImage(c.movie?.backdrop_path, c.movie?.updated_at)
              : wideImage(ep?.still_path, ep?.created_at) ?? wideImage(c.show?.backdrop_path, c.show?.updated_at)
            const duration = isMovie ? c.movie?.duration_sec : ep?.duration_sec
            return (
              <WideCard
                title={isMovie ? c.movie?.title ?? '' : c.show?.title ?? ep?.show_title ?? ''}
                subtitle={
                  isMovie
                    ? remainingLabel(c.progress, duration)
                    : [ep ? episodeShort(ep) : '', c.next ? tr('video.home.nextEpisode') : remainingLabel(c.progress, duration)]
                        .filter(Boolean)
                        .join(' · ')
                }
                image={image}
                progress={c.next ? 0 : progressRatio(c.progress, duration)}
                hasTVPreferredFocus={i === 0 && takePreferred()}
                onFocus={() => focus(continueHero(c), isMovie && c.movie ? ['movie', c.movie.id] : ep ? ['show', ep.show_id] : undefined)}
                onPress={() => playContinue(c)}
              />
            )
          }}
        />

        {data.libraries.map(({ library, movies, shows }) =>
          library.kind === 'movie' ? (
            <Row<Movie>
              key={library.id}
              title={t('video.home.inLibrary', { name: library.name })}
              items={movies}
              inset={INSET}
              itemWidth={card.posterW}
              keyOf={(m) => m.id}
              render={(m, i) => (
                <PosterCard
                  title={m.title}
                  subtitle={m.year ? String(m.year) : undefined}
                  image={posterImage(m)}
                  hasTVPreferredFocus={i === 0 && takePreferred()}
                  onFocus={() => focus(movieHero(m), ['movie', m.id])}
                  onPress={() => openMovie(m.id)}
                />
              )}
            />
          ) : (
            <Row<Show>
              key={library.id}
              title={t('video.home.inLibrary', { name: library.name })}
              items={shows}
              inset={INSET}
              itemWidth={card.posterW}
              keyOf={(s) => s.id}
              render={(s, i) => (
                <PosterCard
                  title={s.title}
                  subtitle={t('video.shows.episodes', { count: s.episode_count })}
                  image={posterImage(s)}
                  hasTVPreferredFocus={i === 0 && takePreferred()}
                  onFocus={() => focus(showHero(s), ['show', s.id])}
                  onPress={() => openShow(s.id)}
                />
              )}
            />
          ),
        )}

        <Row<WatchlistItem>
          title={t('video.home.watchlist')}
          items={data.watchlist}
          inset={INSET}
          itemWidth={card.posterW}
          keyOf={(w) => `${w.kind}${w.movie?.id ?? w.show?.id}`}
          render={(w) => {
            const title = w.movie ?? w.show!
            return (
              <PosterCard
                title={title.title}
                subtitle={title.year ? String(title.year) : undefined}
                image={posterImage(title)}
                onFocus={() => focus(w.movie ? movieHero(w.movie) : showHero(w.show!), [w.kind, title.id])}
                onPress={() => (w.movie ? openMovie(w.movie.id) : openShow(w.show!.id))}
              />
            )
          }}
        />
      </ScrollView>
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  rows: { flex: 1 },
  rowsContent: { paddingBottom: px(120) },
})
