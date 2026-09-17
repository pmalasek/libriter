import { useMemo } from 'react'
import { View } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { bookCount, lifeYears, sortBooks } from 'libriter-shared'

import { AuthorImage } from '@/components/AuthorImage'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/EmptyState'
import { ExpandableText } from '@/components/ExpandableText'
import { BackButton, Screen } from '@/components/Screen'
import { GlassCard } from '@/components/ui/GlassCard'
import { Eyebrow, Heading, Muted } from '@/components/ui/Text'
import { useAuthor, useBooks, useSeriesTitle } from '@/data/hooks'
import { spacing } from '@/theme'

/** Detail autora – AuthorDetailPage z webu bez úprav. */
export default function AuthorScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>()
  const author = useAuthor(id)
  const books = useBooks()
  const seriesTitle = useSeriesTitle()

  // Knihy autora řadíme jako hlavní seznam: série pohromadě v pořadí dílů.
  const authorBooks = useMemo(() => {
    const mine = (books.data ?? []).filter((book) => book.authors?.some((a) => a.id === id))
    return sortBooks(mine, 'title', 'asc', seriesTitle)
  }, [books.data, id, seriesTitle])

  const data = author.data

  return (
    <Screen>
      <BackButton label="Zpět na autory" />

      {author.isError ? (
        <ErrorState error={author.error} onRetry={() => void author.refetch()} />
      ) : !data ? (
        <Muted>{author.isPending ? 'Načítám…' : 'Autor nenalezen.'}</Muted>
      ) : (
        <>
          <GlassCard glow style={{ marginBottom: spacing.lg }}>
            <View style={{ alignItems: 'flex-start' }}>
              <AuthorImage author={data} size={112} />
            </View>
            <Eyebrow style={{ marginTop: spacing.md, marginBottom: 6 }}>Autor</Eyebrow>
            <Heading size={26}>{data.name}</Heading>
            <Muted size={14} style={{ marginTop: 6 }}>
              {[lifeYears(data), bookCount(authorBooks.length)].filter(Boolean).join(' · ')}
            </Muted>
            {data.bio ? (
              <View style={{ marginTop: spacing.md }}>
                <ExpandableText text={data.bio} />
              </View>
            ) : null}
          </GlassCard>

          <BookGrid books={authorBooks} emptyTitle="U tohoto autora nejsou žádné knihy" />
        </>
      )}
    </Screen>
  )
}
