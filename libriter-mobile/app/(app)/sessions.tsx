import { useMemo } from 'react'
import { Alert, View } from 'react-native'
import { Headphones } from 'lucide-react-native'
import { useTranslation } from 'react-i18next'
import { sessionBooks, sessionTitle, type PlaySession } from 'libriter-shared'

import { EmptyState, ErrorState } from '@/components/EmptyState'
import { PageHeader } from '@/components/PageHeader'
import { BackButton, Screen } from '@/components/Screen'
import { SessionCard } from '@/components/SessionCard'
import { SectionTitle } from '@/components/ui/Text'
import { useBooks, useSeriesById, useSessions } from '@/data/hooks'
import { usePullRefresh } from '@/data/usePullRefresh'
import { usePlayer } from '@/player/PlayerProvider'
import { spacing } from '@/theme'

/**
 * Úplný přehled poslechů – SessionsPage z webu. Domů nabídne jen ten
 * nejbližší, tady je vidět, co je kde rozposlouchané, a dá se to uklidit.
 */
export default function SessionsScreen() {
  const { t } = useTranslation()
  const sessions = useSessions()
  const books = useBooks()
  const { map: seriesById } = useSeriesById()
  const player = usePlayer()
  const pull = usePullRefresh(sessions.refetch)

  const bookById = useMemo(() => new Map((books.data ?? []).map((book) => [book.id, book])), [books.data])

  const { open, finished } = useMemo(() => {
    const all = sessions.data ?? []
    return { open: all.filter((s) => !s.finished_at), finished: all.filter((s) => s.finished_at) }
  }, [sessions.data])

  const title = (session: PlaySession) => sessionTitle(session, bookById, seriesById)

  const confirmRemove = (session: PlaySession) =>
    Alert.alert(t('mobile.sessions.removeTitle'), t('mobile.sessions.removeMessage', { title: title(session) }), [
      { text: t('common.cancel'), style: 'cancel' },
      { text: t('mobile.sessions.remove'), style: 'destructive', onPress: () => void player.removeSession(session.id) },
    ])

  return (
    <Screen refreshing={pull.refreshing} onRefresh={pull.onRefresh}>
      <BackButton label={t('common.back')} />
      <PageHeader
        title={t('mobile.sessions.title')}
        description={
          sessions.isPending
            ? undefined
            : open.length > 0
              ? t('mobile.sessions.openCount', { count: open.length })
              : t('mobile.sessions.noneOpen')
        }
      />

      {sessions.isError ? <ErrorState error={sessions.error} onRetry={() => void sessions.refetch()} /> : null}

      {!sessions.isPending && open.length === 0 && finished.length === 0 ? (
        <EmptyState
          icon={Headphones}
          title={t('mobile.sessions.emptyTitle')}
          description={t('mobile.sessions.emptyDescription')}
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
          <SectionTitle style={{ marginTop: spacing.xl, marginBottom: spacing.sm + 4 }}>{t('mobile.sessions.finished')}</SectionTitle>
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
