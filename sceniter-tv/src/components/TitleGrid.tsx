import { useMemo, useState } from 'react'
import { Dimensions, FlatList, ScrollView, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import type { MovieSummary, ShowSummary, TitleKind } from 'sceniter-shared'
import { useLibraries, usePrefetchTitle, useTitles } from '@/api/queries'
import { openMovie, openShow } from '@/lib/nav'
import { posterImage } from '@/lib/titles'
import { card, px, safe } from '@/theme'
import { Button } from './Button'
import { PosterCard } from './Cards'
import { NAV_COLLAPSED } from './SideNav'
import { Empty, ErrorView, Loading } from './States'
import { T } from './T'

type Sort = 'added' | 'title' | 'year' | 'rating'
const SORTS: { key: Sort; label: string; dir: 'asc' | 'desc' }[] = [
  { key: 'added', label: 'video.list.sortAdded', dir: 'desc' },
  { key: 'title', label: 'video.list.sortTitle', dir: 'asc' },
  { key: 'year', label: 'video.list.sortYear', dir: 'desc' },
  { key: 'rating', label: 'video.list.sortRating', dir: 'desc' },
]

const usable = Dimensions.get('window').width - NAV_COLLAPSED - safe.x * 2
const COLUMNS = Math.max(3, Math.floor((usable + card.gap) / (card.posterW + card.gap)))
const ROW_H = card.posterH + px(120)

/** Mřížka všech filmů nebo seriálů s výběrem knihovny a řazením. */
export function TitleGrid({ kind }: { kind: TitleKind }) {
  const { t } = useTranslation()
  const libraries = (useLibraries().data ?? []).filter((l) => l.kind === kind)
  const [library, setLibrary] = useState<string | undefined>()
  const [sort, setSort] = useState<Sort>('added')
  const dir = SORTS.find((s) => s.key === sort)!.dir
  const titles = useTitles(kind, { library, sort, dir })
  const prefetch = usePrefetchTitle()
  const items = useMemo(() => titles.data?.pages.flatMap((p) => p.items) ?? [], [titles.data])
  const total = titles.data?.pages[0]?.total

  return (
    <View style={styles.root}>
      <View style={styles.header}>
        <T variant="title">{t(kind === 'movie' ? 'video.movies.title' : 'video.shows.title')}</T>
        {total !== undefined ? (
          <T variant="caption" faint style={{ marginLeft: px(20), marginBottom: px(6) }}>
            {total}
          </T>
        ) : null}
      </View>
      <TVFocusGuideView autoFocus>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.filters}>
          {libraries.length > 1 ? (
            <>
              <Button label={t('video.tv.allTitles')} selected={!library} onPress={() => setLibrary(undefined)} />
              {libraries.map((l) => (
                <Button key={l.id} label={l.name} selected={library === l.id} onPress={() => setLibrary(l.id)} />
              ))}
              <View style={styles.divider} />
            </>
          ) : null}
          {SORTS.map((s) => (
            <Button key={s.key} label={t(s.label as 'video.list.sortAdded')} selected={sort === s.key} onPress={() => setSort(s.key)} />
          ))}
        </ScrollView>
      </TVFocusGuideView>

      {titles.isError && items.length === 0 ? (
        <ErrorView onRetry={() => void titles.refetch()} />
      ) : titles.isPending ? (
        <Loading />
      ) : items.length === 0 ? (
        <Empty title={t(kind === 'movie' ? 'video.movies.empty' : 'video.shows.empty')} />
      ) : (
        <FlatList
          key={`${library}-${sort}`}
          data={items}
          numColumns={COLUMNS}
          keyExtractor={(x) => x.id}
          columnWrapperStyle={{ gap: card.gap }}
          contentContainerStyle={styles.grid}
          getItemLayout={(_, index) => ({ length: ROW_H, offset: ROW_H * Math.floor(index / COLUMNS), index })}
          onEndReached={() => titles.hasNextPage && !titles.isFetchingNextPage && void titles.fetchNextPage()}
          onEndReachedThreshold={1.5}
          initialNumToRender={COLUMNS * 3}
          windowSize={5}
          removeClippedSubviews
          renderItem={({ item, index }) => (
            <View style={{ height: ROW_H }}>
              <PosterCard
                title={item.title}
                subtitle={item.year ? String(item.year) : undefined}
                image={posterImage(item)}
                hasTVPreferredFocus={index === 0}
                onFocus={() => prefetch(kind, item.id)}
                onPress={() => (kind === 'movie' ? openMovie(item.id) : openShow(item.id))}
              />
            </View>
          )}
        />
      )}
    </View>
  )
}

export type GridItem = MovieSummary | ShowSummary

const styles = StyleSheet.create({
  root: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'flex-end', paddingHorizontal: safe.x, paddingTop: safe.y },
  filters: { gap: px(16), paddingHorizontal: safe.x, paddingVertical: px(24), alignItems: 'center' },
  divider: { width: px(2), height: px(48), backgroundColor: 'rgba(255,255,255,0.2)', marginHorizontal: px(12) },
  grid: { paddingHorizontal: safe.x, paddingTop: px(16), paddingBottom: px(120) },
})
