import { useMemo } from 'react'
import { View } from 'react-native'
import { useLocalSearchParams } from 'expo-router'
import { ListPlus, Pause, Play } from 'lucide-react-native'
import { authorsLabel, bookCount, seriesAuthors } from 'libriter-shared'

import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/EmptyState'
import { ExpandableText } from '@/components/ExpandableText'
import { ActionRow, BackButton, Screen } from '@/components/Screen'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Button } from '@/components/ui/Button'
import { GlassCard } from '@/components/ui/GlassCard'
import { Eyebrow, Heading, Muted } from '@/components/ui/Text'
import { useBooks, useSeriesOne, useSessions } from '@/data/hooks'
import { usePlayer } from '@/player/PlayerProvider'
import { spacing } from '@/theme'

/** Detail série – SeriesDetailPage z webu. */
export default function SeriesScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>()
  const series = useSeriesOne(id)
  const books = useBooks()
  const player = usePlayer()
  const sessions = useSessions()

  // Rozposlouchaný poslech této série – tlačítko pak nabízí pokračování.
  const openSeries = useMemo(
    () => (sessions.data ?? []).some((s) => s.kind === 'series' && s.source_id === id && !s.finished_at),
    [id, sessions.data],
  )

  // Přehrávač právě drží některý díl téhle série (i když vznikl jako seznam).
  const playingFromSeries = player.book?.series_id === id
  const isPlayingSeries = playingFromSeries && player.playing

  const seriesBooks = useMemo(
    () =>
      (books.data ?? [])
        .filter((book) => book.series_id === id)
        .sort((a, b) => (a.series_position ?? Number.MAX_SAFE_INTEGER) - (b.series_position ?? Number.MAX_SAFE_INTEGER)),
    [books.data, id],
  )

  const data = series.data

  return (
    <Screen>
      <BackButton label="Zpět na série" />

      {series.isError ? (
        <ErrorState error={series.error} onRetry={() => void series.refetch()} />
      ) : !data ? (
        <Muted>{series.isPending ? 'Načítám…' : 'Série nenalezena.'}</Muted>
      ) : (
        <>
          <GlassCard glow style={{ marginBottom: spacing.lg }}>
            <View style={{ alignItems: 'flex-start' }}>
              <SeriesCoverStack books={seriesBooks} variant="lg" />
            </View>
            <Eyebrow style={{ marginTop: spacing.md, marginBottom: 6 }}>Série</Eyebrow>
            <Heading size={26}>{data.title}</Heading>
            <Muted size={14} style={{ marginTop: 6 }}>
              {[authorsLabel(seriesAuthors(seriesBooks)), bookCount(seriesBooks.length)].filter(Boolean).join(' · ')}
            </Muted>
            {data.description ? (
              <View style={{ marginTop: spacing.md }}>
                <ExpandableText text={data.description} />
              </View>
            ) : null}

            {seriesBooks.length > 0 ? (
              <ActionRow style={{ marginTop: spacing.lg }}>
                <Button
                  size="lg"
                  icon={isPlayingSeries ? Pause : Play}
                  label={
                    isPlayingSeries
                      ? 'Pozastavit'
                      : playingFromSeries
                        ? 'Přehrát'
                        : openSeries
                          ? 'Pokračovat v sérii'
                          : 'Přehrát sérii'
                  }
                  onPress={() => (playingFromSeries ? void player.toggle() : void player.playSeries(id))}
                  disabled={player.loading}
                />
                {player.session && !openSeries ? (
                  <Button variant="outline" size="lg" icon={ListPlus} label="Přidat do poslechu" onPress={() => void player.addToSession({ seriesIds: [id] })} />
                ) : null}
              </ActionRow>
            ) : null}
          </GlassCard>

          <BookGrid books={seriesBooks} seriesContext={id} emptyTitle="V této sérii nejsou žádné knihy" />
        </>
      )}
    </Screen>
  )
}
