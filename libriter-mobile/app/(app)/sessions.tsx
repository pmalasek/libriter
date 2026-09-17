import { useMemo } from 'react'
import { Alert, View } from 'react-native'
import { Headphones } from 'lucide-react-native'
import { sessionBooks, sessionTitle, type PlaySession } from 'libriter-shared'

import { EmptyState, ErrorState } from '@/components/EmptyState'
import { PageHeader } from '@/components/PageHeader'
import { BackButton, Screen } from '@/components/Screen'
import { SessionCard } from '@/components/SessionCard'
import { SectionTitle } from '@/components/ui/Text'
import { useBooks, useSeriesById, useSessions } from '@/data/hooks'
import { usePlayer } from '@/player/PlayerProvider'
import { spacing } from '@/theme'

/**
 * Úplný přehled poslechů – SessionsPage z webu. Domů nabídne jen ten
 * nejbližší, tady je vidět, co je kde rozposlouchané, a dá se to uklidit.
 */
export default function SessionsScreen() {
  const sessions = useSessions()
  const books = useBooks()
  const { map: seriesById } = useSeriesById()
  const player = usePlayer()

  const bookById = useMemo(() => new Map((books.data ?? []).map((book) => [book.id, book])), [books.data])

  const { open, finished } = useMemo(() => {
    const all = sessions.data ?? []
    return { open: all.filter((s) => !s.finished_at), finished: all.filter((s) => s.finished_at) }
  }, [sessions.data])

  const title = (session: PlaySession) => sessionTitle(session, bookById, seriesById)

  const confirmRemove = (session: PlaySession) =>
    Alert.alert('Odebrat poslech?', `„${title(session)}“ zmizí ze seznamu včetně uložené pozice. Knihy v knihovně zůstanou.`, [
      { text: 'Zrušit', style: 'cancel' },
      { text: 'Odebrat', style: 'destructive', onPress: () => void player.removeSession(session.id) },
    ])

  return (
    <Screen refreshing={sessions.isFetching && !sessions.isPending} onRefresh={() => void sessions.refetch()}>
      <BackButton label="Zpět" />
      <PageHeader
        title="Právě posloucháno"
        description={
          sessions.isPending
            ? undefined
            : open.length > 0
              ? `${open.length} rozposlouchaných · pokračujte tam, kde jste skončili`
              : 'Zatím nic rozposlouchaného'
        }
      />

      {sessions.isError ? <ErrorState error={sessions.error} onRetry={() => void sessions.refetch()} /> : null}

      {!sessions.isPending && open.length === 0 && finished.length === 0 ? (
        <EmptyState
          icon={Headphones}
          title="Zatím nic neposloucháte"
          description="Spusťte knihu tlačítkem Přehrát v jejím detailu, celou sérii u série, nebo si vyberte víc knih naráz v seznamu knih."
        />
      ) : null}

      <View style={{ gap: spacing.sm + 4 }}>
        {open.map((session) => (
          <SessionCard
            key={session.id}
            session={session}
            title={title(session)}
            books={sessionBooks(session, bookById)}
            bookById={bookById}
            onRemove={() => confirmRemove(session)}
          />
        ))}
      </View>

      {finished.length > 0 ? (
        <>
          <SectionTitle style={{ marginTop: spacing.xl, marginBottom: spacing.sm + 4 }}>Doposlechnuté</SectionTitle>
          <View style={{ gap: spacing.sm + 4 }}>
            {finished.map((session) => (
              <SessionCard
                key={session.id}
                session={session}
                title={title(session)}
                books={sessionBooks(session, bookById)}
                bookById={bookById}
                onRemove={() => confirmRemove(session)}
              />
            ))}
          </View>
        </>
      ) : null}
    </Screen>
  )
}
