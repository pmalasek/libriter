import { useCallback, useMemo, useState } from 'react'
import { StyleSheet, View } from 'react-native'
import { Headphones, Library, ListPlus, SquareCheckBig, X } from 'lucide-react-native'
import { authorNames, bookCount, BOOK_SORT_OPTIONS, seriesLabel, sortBooks, type Book } from 'libriter-shared'

import { BookCard, BookRow, BookRowHeader } from '@/components/BookCard'
import { EmptyState, ErrorState } from '@/components/EmptyState'
import { ListScreen } from '@/components/ListScreen'
import { SortControl, ViewModeToggle } from '@/components/ListControls'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/Button'
import { SearchInput } from '@/components/ui/SearchInput'
import { Body } from '@/components/ui/Text'
import { useBookProgress, useBooks, useDownloads, useSeriesById, useSeriesTitle } from '@/data/hooks'
import { useBookListPrefs } from '@/data/listPrefs'
import { usePullRefresh } from '@/data/usePullRefresh'
import { usePlayer } from '@/player/PlayerProvider'
import { radius, spacing, useTheme } from '@/theme'

const COLUMNS = { tiles: 2, small: 3, list: 1 } as const

/** Seznam knih – BooksPage z webu bez editorského „Přidat do série“. */
export default function BooksScreen() {
  const { colors } = useTheme()
  const [query, setQuery] = useState('')
  const books = useBooks()
  const pull = usePullRefresh(books.refetch)
  const prefs = useBookListPrefs()
  const seriesTitle = useSeriesTitle()
  const seriesById = useSeriesById()
  const { status } = useBookProgress()
  const downloads = useDownloads()
  const player = usePlayer()

  // Hromadný výběr: null = vypnuto, jinak množina ID vybraných knih.
  const [selected, setSelected] = useState<Set<string> | null>(null)

  const sorted = useMemo(
    () => sortBooks(books.data ?? [], prefs.sortKey, prefs.sortDir, seriesTitle),
    [books.data, prefs.sortKey, prefs.sortDir, seriesTitle],
  )

  const filtered = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase('cs')
    if (!needle) return sorted
    return sorted.filter(
      (book) =>
        book.title.toLocaleLowerCase('cs').includes(needle) ||
        authorNames(book.authors).toLocaleLowerCase('cs').includes(needle),
    )
  }, [sorted, query])

  const downloaded = useMemo(
    () => new Set((downloads.data ?? []).filter((row) => row.state === 'complete').map((row) => row.bookId)),
    [downloads.data],
  )

  const selectedIds = useMemo(
    () => (selected ? sorted.filter((book) => selected.has(book.id)).map((book) => book.id) : []),
    [selected, sorted],
  )

  const toggle = useCallback(
    (id: string) =>
      setSelected((prev) => {
        const next = new Set(prev)
        if (next.has(id)) next.delete(id)
        else next.add(id)
        return next
      }),
    [],
  )

  const seriesFor = useCallback(
    (book: Book) => (book.series_id ? seriesLabel(seriesById.map.get(book.series_id)?.title, book.series_position) : ''),
    [seriesById.map],
  )

  const renderItem = useCallback(
    ({ item }: { item: Book }) => {
      const selection = selected ? { selected: selected.has(item.id), onToggle: () => toggle(item.id) } : undefined
      const common = { book: item, series: seriesFor(item), selection, status: status(item.id), downloaded: downloaded.has(item.id) }
      if (prefs.view === 'list') return <BookRow {...common} />
      return (
        <View style={{ flex: 1 / COLUMNS[prefs.view] }}>
          <BookCard {...common} size={prefs.view} />
        </View>
      )
    },
    [downloaded, prefs.view, selected, seriesFor, status, toggle],
  )

  const selecting = selected !== null

  const header = (
    <PageHeader
      panel
      title="Knihy"
      description={books.data ? bookCount(books.data.length) : undefined}
      actions={
        <>
          <SearchInput value={query} onChangeText={setQuery} placeholder="Hledat podle názvu nebo autora…" />
          <View style={styles.controls}>
            <SortControl
              options={BOOK_SORT_OPTIONS}
              value={prefs.sortKey}
              onChange={prefs.setSortKey}
              dir={prefs.sortDir}
              onDirChange={prefs.setSortDir}
            />
            <ViewModeToggle value={prefs.view} onChange={prefs.setView} />
          </View>
          {/* Výběr slouží k poskládání poslechu, takže ho má i čtenář. */}
          {!selecting ? (
            <Button variant="outline" icon={SquareCheckBig} label="Vybrat" onPress={() => setSelected(new Set())} style={{ alignSelf: 'flex-start' }} />
          ) : null}
        </>
      }
    >
      {selecting ? (
        <View style={[styles.selectionBar, { backgroundColor: colors.muted }]}>
          <Body size={14} medium>
            {selected.size === 0 ? 'Klepněte na knihy, které chcete vybrat.' : `Vybráno: ${bookCount(selected.size)}`}
          </Body>
          <View style={styles.selectionActions}>
            <Button
              variant="ghost"
              size="sm"
              label={query ? 'Vybrat vše nalezené' : 'Vybrat vše'}
              onPress={() => setSelected(new Set(filtered.map((book) => book.id)))}
              disabled={filtered.length === 0}
            />
            <Button variant="ghost" size="sm" label="Zrušit výběr" onPress={() => setSelected(new Set())} disabled={selected.size === 0} />
          </View>
          <View style={styles.selectionActions}>
            {player.session ? (
              <Button
                variant="outline"
                size="sm"
                icon={ListPlus}
                label="Přidat do poslechu"
                onPress={() => {
                  void player.addToSession({ bookIds: selectedIds })
                  setSelected(null)
                }}
                disabled={selected.size === 0}
              />
            ) : null}
            <Button
              size="sm"
              icon={Headphones}
              label="Poslouchat výběr"
              onPress={() => {
                void player.playList({ bookIds: selectedIds })
                setSelected(null)
              }}
              disabled={selected.size === 0}
            />
            <Button variant="outline" size="sm" icon={X} label="Hotovo" onPress={() => setSelected(null)} />
          </View>
        </View>
      ) : null}
      {prefs.view === 'list' && filtered.length > 0 ? (
        <View style={[styles.rowHeader, { borderColor: colors.glassEdge }]}>
          <BookRowHeader selecting={selecting} />
        </View>
      ) : null}
    </PageHeader>
  )

  return (
    <ListScreen
      listKey={prefs.view}
      columns={COLUMNS[prefs.view]}
      data={filtered}
      keyExtractor={(book) => book.id}
      renderItem={renderItem}
      header={header}
      refreshing={pull.refreshing}
      onRefresh={pull.onRefresh}
      empty={
        books.isError ? (
          <ErrorState error={books.error} onRetry={() => void books.refetch()} />
        ) : (
          <EmptyState
            icon={Library}
            title={books.isPending ? 'Načítám…' : query ? 'Nic nenalezeno' : 'Zatím žádné knihy'}
            description={
              books.isPending
                ? undefined
                : query
                  ? 'Zkuste jiný hledaný výraz.'
                  : 'Přidejte audio soubory do adresáře AUDIO_ROOT – scanner je načte automaticky.'
            }
          />
        )
      }
    />
  )
}

const styles = StyleSheet.create({
  controls: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  selectionBar: { marginTop: spacing.md, borderRadius: radius['2xl'], padding: spacing.sm + 4, gap: spacing.sm },
  selectionActions: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  rowHeader: { marginTop: spacing.md, borderRadius: radius.lg, borderWidth: 1, overflow: 'hidden' },
})
