import { memo, useCallback, useMemo, useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Layers } from 'lucide-react-native'
import { authorsLabel, bookCount, foldName, seriesAuthors, type Author, type Book, type Series } from 'libriter-shared'

import { EmptyState, ErrorState } from '@/components/EmptyState'
import { ListScreen } from '@/components/ListScreen'
import { PageHeader } from '@/components/PageHeader'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Badge } from '@/components/ui/Badge'
import { SearchInput } from '@/components/ui/SearchInput'
import { Body, Muted } from '@/components/ui/Text'
import { useBooks, useSeriesList } from '@/data/hooks'
import { usePullRefresh } from '@/data/usePullRefresh'
import { radius, spacing, useTheme } from '@/theme'

/** Co o sérii víme z knih – počet dílů, autoři a obálky prvních dílů. */
interface SeriesInfo {
  count: number
  authors: Author[]
  covers: Book[]
}

const EMPTY: SeriesInfo = { count: 0, authors: [], covers: [] }

/** Seznam sérií – SeriesPage z webu. */
export default function SeriesScreen() {
  const series = useSeriesList()
  const books = useBooks()
  const [query, setQuery] = useState('')
  const pull = usePullRefresh(series.refetch)

  // Série nesou jen název; počet dílů i autory dopočítáváme z knih.
  const infoBySeries = useMemo(() => {
    const bySeries = new Map<string, Book[]>()
    for (const book of books.data ?? []) {
      if (!book.series_id) continue
      const group = bySeries.get(book.series_id)
      if (group) group.push(book)
      else bySeries.set(book.series_id, [book])
    }
    const info = new Map<string, SeriesInfo>()
    for (const [id, group] of bySeries) {
      const covers = [...group]
        .sort((a, b) => (a.series_position ?? Number.MAX_SAFE_INTEGER) - (b.series_position ?? Number.MAX_SAFE_INTEGER))
        .slice(0, 3)
      info.set(id, { count: group.length, authors: seriesAuthors(group), covers })
    }
    return info
  }, [books.data])

  // Prázdné série v seznamu jen překážejí.
  const withBooks = useMemo(
    () => (series.data ?? []).filter((item) => (infoBySeries.get(item.id)?.count ?? 0) > 0),
    [series.data, infoBySeries],
  )

  const filtered = useMemo(() => {
    const needle = foldName(query)
    if (!needle) return withBooks
    // Hledá se podle názvu série i podle jmen autorů.
    return withBooks.filter((item) => {
      const info = infoBySeries.get(item.id) ?? EMPTY
      return foldName(item.title).includes(needle) || info.authors.some((author) => foldName(author.name).includes(needle))
    })
  }, [withBooks, infoBySeries, query])

  const renderItem = useCallback(
    ({ item }: { item: Series }) => <SeriesItem series={item} info={infoBySeries.get(item.id) ?? EMPTY} />,
    [infoBySeries],
  )

  const header = (
    <PageHeader
      panel
      title="Série"
      description={series.data ? `${withBooks.length} celkem` : undefined}
      actions={<SearchInput value={query} onChangeText={setQuery} placeholder="Hledat podle názvu nebo autora…" />}
    />
  )

  return (
    <ListScreen
      data={filtered}
      keyExtractor={(item) => item.id}
      renderItem={renderItem}
      header={header}
      refreshing={pull.refreshing}
      onRefresh={pull.onRefresh}
      empty={
        series.isError ? (
          <ErrorState error={series.error} onRetry={() => void series.refetch()} />
        ) : (
          <EmptyState
            icon={Layers}
            title={series.isPending ? 'Načítám…' : query ? 'Nic nenalezeno' : 'Zatím žádné série'}
            description={series.isPending ? undefined : query ? 'Zkuste jiný hledaný výraz.' : 'Série se zakládají ve webovém rozhraní.'}
          />
        )
      }
    />
  )
}

const SeriesItem = memo(function SeriesItem({ series, info }: { series: Series; info: SeriesInfo }) {
  const { colors } = useTheme()
  const router = useRouter()
  return (
    <Pressable
      onPress={() => router.push(`/series/${series.id}`)}
      style={({ pressed }) => [styles.card, { backgroundColor: colors.card, borderColor: colors.border, opacity: pressed ? 0.7 : 1 }]}
    >
      <SeriesCoverStack books={info.covers} />
      <View style={{ flex: 1, minWidth: 0 }}>
        <Body medium numberOfLines={1}>
          {series.title}
        </Body>
        <Muted size={13} numberOfLines={1}>
          {authorsLabel(info.authors)}
        </Muted>
        <View style={{ marginTop: 4, alignSelf: 'flex-start' }}>
          <Badge variant="highlight" label={bookCount(info.count)} />
        </View>
      </View>
    </Pressable>
  )
})

const styles = StyleSheet.create({
  card: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md - 4,
    borderWidth: 1,
    borderRadius: radius['2xl'],
    padding: spacing.md,
    marginBottom: spacing.sm + 4,
  },
})
