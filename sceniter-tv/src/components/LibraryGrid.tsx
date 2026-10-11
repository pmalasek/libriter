import { memo, useCallback, useMemo, useRef, useState } from 'react'
import { Dimensions, FlatList, StyleSheet, TVFocusGuideView, View, type ListRenderItem } from 'react-native'
import { Image } from 'expo-image'
import { LinearGradient } from 'expo-linear-gradient'
import { useTranslation } from 'react-i18next'
import { ArrowDownNarrowWideIcon, ArrowUpDownIcon, ArrowUpNarrowWideIcon, SearchIcon } from 'lucide-react-native'
import type { Library, MovieSummary, ShowSummary, TitleKind } from 'sceniter-shared'
import { useDebouncedPrefetch, useTitles } from '@/api/queries'
import { createFocusStore, useSettled } from '@/lib/focusStore'
import { forgetIndex, recallIndex, rememberIndex } from '@/lib/gridMemory'
import { openMovie, openSearch, openShow } from '@/lib/nav'
import { useLibrarySort } from '@/lib/prefs'
import { SORT_LABEL, type Sort } from '@/lib/sort'
import { backdropImage, posterImage } from '@/lib/titles'
import { bgAlpha, card, colors, px, radius, safe } from '@/theme'
import { Button } from './Button'
import { PosterCard } from './Cards'
import { NAV_COLLAPSED } from './SideNav'
import { SortPicker } from './SortPicker'
import { Glow } from './Glow'
import { Empty, ErrorView, Loading } from './States'
import { T } from './T'

export type GridItem = MovieSummary | ShowSummary

const usable = Dimensions.get('window').width - NAV_COLLAPSED - safe.x * 2
const COLUMNS = Math.max(3, Math.floor((usable + card.gap) / (card.posterW + card.gap)))
const ROW_H = card.posterH + px(120)
const GRID_TOP = px(16)

interface CellProps {
  item: GridItem
  index: number
  kind: TitleKind
  preferred: boolean
  onFocusItem: (index: number, item: GridItem) => void
}

/** Jedna karta mřížky – memo se stabilními callbacky, ať se při pohybu nepřekresluje celá mřížka. */
const GridCell = memo(function GridCell({ item, index, kind, preferred, onFocusItem }: CellProps) {
  const onFocus = useCallback(() => onFocusItem(index, item), [index, item, onFocusItem])
  const onPress = useCallback(() => (kind === 'movie' ? openMovie(item.id) : openShow(item.id)), [item.id, kind])
  return (
    <View style={styles.cell}>
      <PosterCard
        title={item.title}
        subtitle={item.year ? String(item.year) : undefined}
        image={posterImage(item)}
        hasTVPreferredFocus={preferred}
        onFocus={onFocus}
        onPress={onPress}
      />
    </View>
  )
})

/**
 * Rozmazaný backdrop zaměřeného titulu v horní části – nálada knihovny.
 * Mění se až po chvíli klidu a překresluje se jen tahle část, ne mřížka.
 */
function LibraryBackdrop({ store }: { store: ReturnType<typeof createFocusStore<GridItem>> }) {
  const item = store.use()
  const uri = useSettled(backdropImage(item), 220)
  return (
    <View style={styles.backdrop} pointerEvents="none">
      {uri ? (
        <Image
          source={uri}
          style={StyleSheet.absoluteFill}
          contentFit="cover"
          blurRadius={14}
          cachePolicy="memory-disk"
          transition={{ duration: 450, effect: 'cross-dissolve' }}
        />
      ) : null}
      <LinearGradient colors={[bgAlpha(0.5), bgAlpha(0.8), colors.bg]} locations={[0, 0.55, 1]} style={StyleSheet.absoluteFill} />
    </View>
  )
}

/**
 * Výpis jedné knihovny: nadpis s počtem, vpravo hledání, řazení a směr,
 * pod ním mřížka plakátů. Fokus se po návratu vrátí na poslední kartu.
 */
export function LibraryGrid({ library }: { library: Library }) {
  const { t } = useTranslation()
  const kind = library.kind
  const { sort, dir, setSort, setDir } = useLibrarySort(library.id)
  const titles = useTitles(kind, { library: library.id, sort, dir })
  const prefetch = useDebouncedPrefetch()
  const items = useMemo(() => titles.data?.pages.flatMap((p) => p.items) ?? [], [titles.data])
  const total = titles.data?.pages[0]?.total

  const listRef = useRef<FlatList<GridItem>>(null)
  const focusStore = useRef(createFocusStore<GridItem>()).current
  const [initialIndex] = useState(() => recallIndex(library.id))
  // Karta, která dostane fokus po namontování; -1 = žádná (fokus už je jinde).
  const [preferred, setPreferred] = useState(initialIndex)
  const [picker, setPicker] = useState(false)
  // Po zavření výběru se fokus vrátí na tlačítko řazení (klíč ho přemontuje).
  const [sortFocus, setSortFocus] = useState(0)

  const onFocusItem = useCallback(
    (index: number, item: GridItem) => {
      rememberIndex(library.id, index)
      setPreferred((p) => (p === -1 ? p : -1))
      focusStore.set(item)
      prefetch(kind, item.id)
    },
    [focusStore, kind, library.id, prefetch],
  )

  const closePicker = useCallback(() => {
    setPicker(false)
    setSortFocus((n) => n + 1)
  }, [])

  // Po přeskládání začíná výpis znovu shora; zapamatovaná karta už neplatí.
  const restart = useCallback(() => {
    forgetIndex(library.id)
    setPreferred(-1)
    listRef.current?.scrollToOffset({ offset: 0, animated: false })
  }, [library.id])

  const chooseSort = useCallback(
    (s: Sort) => {
      setSort(s)
      restart()
      closePicker()
    },
    [closePicker, restart, setSort],
  )

  const toggleDir = useCallback(() => {
    setDir(dir === 'asc' ? 'desc' : 'asc')
    restart()
  }, [dir, restart, setDir])

  // Zapamatovaná karta mohla mezitím z knihovny zmizet – pak poslední.
  const preferredIndex = preferred === -1 ? -1 : Math.min(preferred, items.length - 1)
  const renderItem = useCallback<ListRenderItem<GridItem>>(
    ({ item, index }) => <GridCell item={item} index={index} kind={kind} preferred={index === preferredIndex} onFocusItem={onFocusItem} />,
    [kind, onFocusItem, preferredIndex],
  )

  const startRow = Math.floor(Math.min(initialIndex, Math.max(0, items.length - 1)) / COLUMNS)

  return (
    <View style={styles.root}>
      <LibraryBackdrop store={focusStore} />
      <Glow />
      <View style={styles.header}>
        <View style={styles.heading}>
          <T variant="title" numberOfLines={1} style={styles.name}>
            {library.name}
          </T>
          {total !== undefined ? (
            <View style={styles.count}>
              <T variant="caption" dim>
                {total}
              </T>
            </View>
          ) : null}
        </View>
        <TVFocusGuideView autoFocus style={styles.tools}>
          <Button icon={SearchIcon} accessibilityLabel={t('video.nav.search')} onPress={openSearch} />
          <Button
            key={sortFocus}
            icon={ArrowUpDownIcon}
            label={t('video.tv.sort.button', { value: t(SORT_LABEL[sort]) })}
            hasTVPreferredFocus={sortFocus > 0 || (items.length === 0 && !titles.isPending)}
            onPress={() => setPicker(true)}
          />
          <Button
            icon={dir === 'asc' ? ArrowDownNarrowWideIcon : ArrowUpNarrowWideIcon}
            label={t(dir === 'asc' ? 'video.tv.sort.asc' : 'video.tv.sort.desc')}
            onPress={toggleDir}
          />
        </TVFocusGuideView>
      </View>

      {titles.isError && items.length === 0 ? (
        <ErrorView onRetry={() => void titles.refetch()} />
      ) : !titles.data ? (
        <Loading />
      ) : items.length === 0 ? (
        <Empty title={t(kind === 'movie' ? 'video.movies.empty' : 'video.shows.empty')} />
      ) : (
        <FlatList
          ref={listRef}
          data={items}
          numColumns={COLUMNS}
          keyExtractor={(x) => x.id}
          columnWrapperStyle={styles.columns}
          contentContainerStyle={styles.grid}
          style={titles.isPlaceholderData ? styles.stale : undefined}
          // FlatList s více sloupci volá getItemLayout s indexem řádku.
          getItemLayout={(_, row) => ({ length: ROW_H, offset: GRID_TOP + ROW_H * row, index: row })}
          initialScrollIndex={startRow > 0 ? startRow : undefined}
          onEndReached={() => {
            if (titles.hasNextPage && !titles.isFetchingNextPage && !titles.isPlaceholderData) void titles.fetchNextPage()
          }}
          onEndReachedThreshold={1.5}
          initialNumToRender={COLUMNS * 3}
          maxToRenderPerBatch={COLUMNS * 2}
          windowSize={5}
          removeClippedSubviews
          renderItem={renderItem}
          extraData={preferredIndex}
        />
      )}

      {picker ? <SortPicker value={sort} onSelect={chooseSort} onClose={closePicker} /> : null}
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: px(32),
    paddingHorizontal: safe.x,
    paddingTop: safe.y,
    paddingBottom: px(20),
  },
  heading: { flexDirection: 'row', alignItems: 'flex-end', flexShrink: 1 },
  tools: { flexDirection: 'row', alignItems: 'center', gap: px(16) },
  name: { flexShrink: 1 },
  count: {
    marginLeft: px(20),
    marginBottom: px(10),
    paddingHorizontal: px(18),
    paddingVertical: px(4),
    borderRadius: radius.pill,
    backgroundColor: colors.surfaceHi,
    borderWidth: px(2),
    borderColor: colors.edge,
  },
  backdrop: { position: 'absolute', left: 0, right: 0, top: 0, height: px(640) },
  grid: { paddingHorizontal: safe.x, paddingTop: GRID_TOP, paddingBottom: px(120) },
  columns: { gap: card.gap },
  cell: { height: ROW_H },
  stale: { opacity: 0.6 },
})
