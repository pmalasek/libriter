import { useMemo } from 'react'
import { Pressable, StyleSheet, View } from 'react-native'
import { useLocalSearchParams, useRouter } from 'expo-router'
import {
  Calendar,
  CheckCircle2,
  Clock,
  Download,
  Headphones,
  Languages,
  ListPlus,
  Mic,
  Pause,
  Play,
  RotateCcw,
  Star,
  Trash2,
} from 'lucide-react-native'
import { chapterCount, formatBytes, formatClock, formatDate, formatDuration, sortBooks } from 'libriter-shared'

import { BookCover, coverUrl } from '@/components/BookCover'
import { BookGrid } from '@/components/BookGrid'
import { ChapterList } from '@/components/ChapterList'
import { ErrorState } from '@/components/EmptyState'
import { ExpandableText } from '@/components/ExpandableText'
import { ActionRow, BackButton, Screen } from '@/components/Screen'
import { toast } from '@/components/Toast'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { GlassCard } from '@/components/ui/GlassCard'
import { Body, Eyebrow, Heading, Muted, SectionTitle } from '@/components/ui/Text'
import {
  useBook,
  useBookProgress,
  useBooks,
  useChapters,
  useDownloads,
  useSeriesOne,
  useSeriesTitle,
  useSessions,
  useSetBookFinished,
} from '@/data/hooks'
import { downloadManager } from '@/downloads/downloadManager'
import { usePlayer } from '@/player/PlayerProvider'
import { fonts, radius, spacing, useTheme } from '@/theme'

/** Detail knihy – BookDetailPage z webu bez úprav; navíc stahování do telefonu. */
export default function BookScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>()
  const { colors } = useTheme()
  const router = useRouter()
  const book = useBook(id)
  const series = useSeriesOne(book.data?.series_id ?? '')
  const allBooks = useBooks()
  const seriesTitle = useSeriesTitle()
  const player = usePlayer()
  const sessions = useSessions()
  const progress = useBookProgress()
  const setFinished = useSetBookFinished()
  const downloads = useDownloads()
  const chapters = useChapters(id)

  const status = progress.status(id)
  const finishedAt = progress.map.get(id)?.finished_at
  const download = useMemo(() => (downloads.data ?? []).find((row) => row.bookId === id), [downloads.data, id])
  const size = useMemo(() => (chapters.data ?? []).reduce((sum, chapter) => sum + chapter.size_bytes, 0), [chapters.data])

  // Rozposlouchaná pozice knihy – z libovolného poslechu, který ji obsahuje.
  const started = useMemo(
    () => (sessions.data ?? []).flatMap((s) => s.items).find((item) => item.book_id === id),
    [id, sessions.data],
  )
  const inSession = started !== undefined
  const resumeAt = started && started.position_seconds > 0 ? started.position_seconds : null

  const isOpenBook = player.book?.id === id
  const isPlayingBook = isOpenBook && player.playing

  const mainAuthor = book.data?.authors?.[0]
  const moreByAuthor = useMemo(() => {
    if (!book.data || !mainAuthor) return []
    const byAuthor = (allBooks.data ?? []).filter(
      (b) => b.id !== book.data?.id && b.authors?.some((a) => a.id === mainAuthor.id),
    )
    return sortBooks(byAuthor, 'title', 'asc', seriesTitle)
  }, [allBooks.data, book.data, mainAuthor, seriesTitle])

  const changeStatus = (finished: boolean) =>
    setFinished.mutate(
      { bookId: id, finished },
      {
        onSuccess: () => toast.success(finished ? 'Kniha je označená jako doposlechnutá.' : 'Označení zrušeno.'),
        onError: (error) => toast.error(error.message),
      },
    )

  const data = book.data

  return (
    <Screen>
      <BackButton label="Zpět" />

      {book.isError ? (
        <ErrorState error={book.error} onRetry={() => void book.refetch()} />
      ) : !data ? (
        <Muted>{book.isPending ? 'Načítám…' : 'Kniha nenalezena.'}</Muted>
      ) : (
        <>
          <GlassCard glow backdrop={data.cover_path ? coverUrl(data) : undefined}>
            <View style={{ alignItems: 'flex-start' }}>
              <BookCover book={data} size={160} downloaded={download?.state === 'complete'} />
            </View>

            {series.data ? (
              <Pressable onPress={() => router.push(`/series/${series.data?.id}`)} style={{ marginTop: spacing.md }}>
                <Eyebrow>
                  {series.data.title}
                  {data.series_position != null ? ` · ${data.series_position}. díl` : ''}
                </Eyebrow>
              </Pressable>
            ) : (
              <Eyebrow style={{ marginTop: spacing.md }}>Audiokniha</Eyebrow>
            )}

            <Heading size={26} style={{ marginTop: 6 }}>
              {data.title}
            </Heading>

            {data.authors?.length ? (
              <View style={styles.authors}>
                {data.authors.map((author, index) => (
                  <Pressable key={author.id} onPress={() => router.push(`/author/${author.id}`)}>
                    <Body size={17} style={{ color: colors.primary, fontFamily: fonts.sansMedium }}>
                      {index > 0 ? ', ' : ''}
                      {author.name}
                    </Body>
                  </Pressable>
                ))}
              </View>
            ) : null}

            <View style={styles.badges}>
              <Badge variant="highlight" icon={Clock} label={formatDuration(data.duration_seconds)} />
              {data.narrator ? <Badge icon={Mic} label={data.narrator} /> : null}
              {data.published_year ? <Badge icon={Calendar} label={String(data.published_year)} /> : null}
              <Badge variant="outline" icon={Languages} label={data.language.toUpperCase()} />
              {status === 'finished' ? (
                <Badge variant="highlight" icon={CheckCircle2} label={finishedAt ? `Doposlechnuto ${formatDate(finishedAt)}` : 'Doposlechnuto'} />
              ) : status === 'started' ? (
                <Badge icon={Headphones} label="Rozposlouchané" />
              ) : null}
              {data.internal_rating ? <Badge variant="outline" icon={Star} label={`${data.internal_rating}/5`} /> : null}
            </View>

            <ActionRow style={{ marginTop: spacing.lg }}>
              <Button
                size="lg"
                icon={isPlayingBook ? Pause : Play}
                label={
                  isPlayingBook ? 'Pozastavit' : isOpenBook ? 'Přehrát' : resumeAt != null ? `Pokračovat (${formatClock(resumeAt)})` : 'Přehrát'
                }
                onPress={() => (isOpenBook ? void player.toggle() : void player.playBook(data.id))}
                disabled={player.loading}
              />
              {player.session && !inSession ? (
                <Button variant="outline" size="lg" icon={ListPlus} label="Přidat do poslechu" onPress={() => void player.addToSession({ bookIds: [data.id] })} />
              ) : null}
              {status === 'finished' ? (
                <Button variant="outline" size="lg" icon={RotateCcw} label="Zrušit označení" onPress={() => changeStatus(false)} disabled={setFinished.isPending} />
              ) : (
                <Button
                  variant="outline"
                  size="lg"
                  icon={CheckCircle2}
                  label="Označit jako doposlechnuté"
                  onPress={() => changeStatus(true)}
                  disabled={setFinished.isPending}
                />
              )}
            </ActionRow>
          </GlassCard>

          {/* Stahování je jediné, co web nemá – obsah zůstává na serveru,
              telefon si ho bere s sebou. */}
          <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.border }]}>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: spacing.md }}>
              <View style={{ flex: 1 }}>
                <SectionTitle>V telefonu</SectionTitle>
                <Muted size={13}>{describeDownload(download?.state, download?.bytesDone ?? 0, download?.bytesTotal ?? 0, size, download?.error ?? '')}</Muted>
              </View>
              <DownloadButton bookId={data.id} state={download?.state} />
            </View>
            {download && download.state !== 'complete' && download.state !== 'error' ? (
              <View style={[styles.track, { backgroundColor: colors.border }]}>
                <View
                  style={[
                    styles.fill,
                    { backgroundColor: colors.primary, width: `${download.bytesTotal > 0 ? Math.min(100, (download.bytesDone / download.bytesTotal) * 100) : 0}%` },
                  ]}
                />
              </View>
            ) : null}
          </View>

          <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.border }]}>
            <SectionTitle style={{ marginBottom: spacing.sm }}>O knize</SectionTitle>
            {data.description ? <ExpandableText text={data.description} /> : <Muted size={14}>Popis není k dispozici.</Muted>}
            <Muted size={12} style={{ marginTop: spacing.md }}>
              {chapterCount(data.chapter_count)} · přidáno {formatDate(data.created_at)}
            </Muted>
          </View>

          <ChapterList key={data.id} book={data} />

          {moreByAuthor.length > 0 && mainAuthor ? (
            <View style={{ marginTop: spacing.xl }}>
              <SectionTitle style={{ marginBottom: spacing.md }}>Další knihy autora {mainAuthor.name}</SectionTitle>
              <BookGrid books={moreByAuthor} />
            </View>
          ) : null}
        </>
      )}
    </Screen>
  )
}

function DownloadButton({ bookId, state }: { bookId: string; state?: string }) {
  if (state === 'complete') {
    return <Button variant="outline" icon={Trash2} label="Smazat" onPress={() => void downloadManager.remove(bookId)} />
  }
  if (state === 'downloading' || state === 'queued') {
    return <Button variant="outline" label="Pozastavit" onPress={() => void downloadManager.pause(bookId)} />
  }
  return (
    <Button
      icon={Download}
      label={state === 'paused' ? 'Pokračovat' : 'Stáhnout'}
      onPress={() => void downloadManager.enqueue(bookId).catch((error: Error) => toast.error(error.message))}
    />
  )
}

function describeDownload(state: string | undefined, done: number, total: number, size: number, error: string): string {
  switch (state) {
    case 'complete':
      return `Staženo · ${formatBytes(done)}`
    case 'downloading':
      return `Stahuji · ${formatBytes(done)} z ${formatBytes(total)}`
    case 'queued':
      return 'Ve frontě'
    case 'paused':
      return error ? `Pozastaveno · ${error}` : 'Pozastaveno'
    case 'error':
      return `Stahování selhalo: ${error}`
    default:
      return size > 0 ? `Ke stažení ${formatBytes(size)}` : 'Kniha se přehrává ze serveru'
  }
}

const styles = StyleSheet.create({
  authors: { flexDirection: 'row', flexWrap: 'wrap', marginTop: spacing.sm },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm, marginTop: spacing.md },
  card: { borderWidth: 1, borderRadius: radius['2xl'], padding: spacing.md, marginTop: spacing.lg, gap: spacing.sm },
  track: { height: 4, borderRadius: 2, overflow: 'hidden' },
  fill: { height: 4 },
})
