import { memo, useCallback, useMemo, useState } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useRouter } from 'expo-router'
import { Users } from 'lucide-react-native'
import {
  AUTHOR_SORT_OPTIONS,
  bookCount,
  catalogName,
  foldName,
  lifeYears,
  sortAuthors,
  type Author,
  type ViewMode,
} from 'libriter-shared'

import { AuthorImage } from '@/components/AuthorImage'
import { EmptyState, ErrorState } from '@/components/EmptyState'
import { ListScreen } from '@/components/ListScreen'
import { SortControl, ViewModeToggle } from '@/components/ListControls'
import { PageHeader } from '@/components/PageHeader'
import { SearchInput } from '@/components/ui/SearchInput'
import { Badge } from '@/components/ui/Badge'
import { Body, Muted, Title } from '@/components/ui/Text'
import { useAuthors, useBooks } from '@/data/hooks'
import { useAuthorListPrefs } from '@/data/listPrefs'
import { radius, spacing, useTheme } from '@/theme'

const COLUMNS: Record<ViewMode, number> = { tiles: 1, small: 2, list: 1 }

/** Seznam autorů – AuthorsPage z webu. */
export default function AuthorsScreen() {
  const authors = useAuthors()
  const books = useBooks()
  const prefs = useAuthorListPrefs()
  const [query, setQuery] = useState('')

  const countByAuthor = useMemo(() => {
    const counts = new Map<string, number>()
    for (const book of books.data ?? []) {
      for (const author of book.authors ?? []) counts.set(author.id, (counts.get(author.id) ?? 0) + 1)
    }
    return counts
  }, [books.data])

  // Autoři bez knih v seznamu jen překážejí.
  const withBooks = useMemo(
    () => (authors.data ?? []).filter((author) => (countByAuthor.get(author.id) ?? 0) > 0),
    [authors.data, countByAuthor],
  )

  const sorted = useMemo(
    () => sortAuthors(withBooks, prefs.sortKey, prefs.sortDir, (id) => countByAuthor.get(id) ?? 0),
    [withBooks, prefs.sortKey, prefs.sortDir, countByAuthor],
  )

  // Hledá se přes celé jméno i jeho části, bez ohledu na diakritiku.
  const filtered = useMemo(() => {
    const needle = foldName(query)
    if (!needle) return sorted
    return sorted.filter((author) => {
      const name = foldName(author.name)
      return needle.split(' ').every((part) => name.includes(part))
    })
  }, [sorted, query])

  const renderItem = useCallback(
    ({ item }: { item: Author }) => (
      <AuthorItem
        author={item}
        view={prefs.view}
        // Při řazení podle příjmení se jméno ukazuje katalogově („Čapek, Karel“).
        name={prefs.sortKey === 'last_name' ? catalogName(item) : item.name}
        count={countByAuthor.get(item.id) ?? 0}
      />
    ),
    [countByAuthor, prefs.sortKey, prefs.view],
  )

  const header = (
    <PageHeader
      panel
      title="Autoři"
      description={authors.data ? `${withBooks.length} celkem` : undefined}
      actions={
        <>
          <SearchInput value={query} onChangeText={setQuery} placeholder="Hledat podle jména…" />
          <View style={styles.controls}>
            <SortControl
              options={AUTHOR_SORT_OPTIONS}
              value={prefs.sortKey}
              onChange={prefs.setSortKey}
              dir={prefs.sortDir}
              onDirChange={prefs.setSortDir}
            />
            <ViewModeToggle value={prefs.view} onChange={prefs.setView} />
          </View>
        </>
      }
    />
  )

  return (
    <ListScreen
      listKey={prefs.view}
      columns={COLUMNS[prefs.view]}
      data={filtered}
      keyExtractor={(author) => author.id}
      renderItem={renderItem}
      header={header}
      refreshing={authors.isFetching && !authors.isPending}
      onRefresh={() => void authors.refetch()}
      empty={
        authors.isError ? (
          <ErrorState error={authors.error} onRetry={() => void authors.refetch()} />
        ) : (
          <EmptyState
            icon={Users}
            title={authors.isPending ? 'Načítám…' : query ? 'Nic nenalezeno' : 'Zatím žádní autoři'}
            description={
              authors.isPending
                ? undefined
                : query
                  ? 'Zkuste jiný hledaný výraz.'
                  : 'Autoři vznikají automaticky při načtení audio souborů scannerem.'
            }
          />
        )
      }
    />
  )
}

/** Jeden autor ve třech variantách zobrazení; memo kvůli psaní do hledání. */
const AuthorItem = memo(function AuthorItem({
  author,
  view,
  name,
  count,
}: {
  author: Author
  view: ViewMode
  name: string
  count: number
}) {
  const { colors } = useTheme()
  const router = useRouter()
  const details = [lifeYears(author), bookCount(count)].filter(Boolean).join(' · ')
  const open = () => router.push(`/author/${author.id}`)

  if (view === 'list') {
    return (
      <Pressable
        onPress={open}
        style={({ pressed }) => [
          styles.listRow,
          { backgroundColor: pressed ? colors.muted : colors.glass, borderColor: colors.glassEdge },
        ]}
      >
        <AuthorImage author={author} size={36} />
        <Title numberOfLines={1} style={{ flex: 1 }}>
          {name}
        </Title>
        <Muted size={12}>{details}</Muted>
      </Pressable>
    )
  }

  if (view === 'small') {
    return (
      <Pressable
        onPress={open}
        style={({ pressed }) => [styles.smallCard, { backgroundColor: colors.card, borderColor: colors.border, opacity: pressed ? 0.7 : 1 }]}
      >
        <AuthorImage author={author} size={36} />
        <View style={{ flex: 1, minWidth: 0 }}>
          <Body size={13} medium numberOfLines={1}>
            {name}
          </Body>
          <Muted size={11} numberOfLines={1}>
            {details}
          </Muted>
        </View>
      </Pressable>
    )
  }

  return (
    <Pressable
      onPress={open}
      style={({ pressed }) => [styles.tileCard, { backgroundColor: colors.card, borderColor: colors.border, opacity: pressed ? 0.7 : 1 }]}
    >
      <AuthorImage author={author} size={56} />
      <View style={{ flex: 1, minWidth: 0 }}>
        <Body medium numberOfLines={1}>
          {name}
        </Body>
        {lifeYears(author) ? <Muted size={13}>{lifeYears(author)}</Muted> : null}
      </View>
      <Badge variant="brand" label={bookCount(count)} />
    </Pressable>
  )
})

const styles = StyleSheet.create({
  controls: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  listRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm + 4,
    paddingHorizontal: spacing.sm + 4,
    paddingVertical: spacing.sm,
    borderWidth: 1,
    borderRadius: radius.lg,
    marginBottom: spacing.sm,
  },
  smallCard: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderWidth: 1,
    borderRadius: radius['2xl'],
    paddingHorizontal: spacing.sm + 4,
    paddingVertical: spacing.sm,
    marginBottom: spacing.sm + 4,
  },
  tileCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md - 4,
    borderWidth: 1,
    borderRadius: radius['2xl'],
    padding: spacing.md,
    marginBottom: spacing.sm + 4,
  },
})
