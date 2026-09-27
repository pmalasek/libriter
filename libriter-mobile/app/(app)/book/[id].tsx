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
import { useTranslation } from 'react-i18next'
import {
  chapterCount,
  formatBytes,
  formatClock,
  formatDate,
  formatDuration,
  languageLabel,
  seriesLabel,
  sortBooks,
  t,
} from 'libriter-shared'

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
  useLanguages,
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
  const { t } = useTranslation()
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
  const languages = useLanguages()

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
        onSuccess: () => toast.success(finished ? t('mobile.book.markedFinished') : t('mobile.book.unmarked')),
        onError: (error) => toast.error(error.message),
      },
    )

  const data = book.data

  return (
    <Screen>
      <BackButton label={t('common.back')} />

      {book.isError ? (
        <ErrorState error={book.error} onRetry={() => void book.refetch()} />
      ) : !data ? (
        <Muted>{book.isPending ? t('common.loading') : t('mobile.book.notFound')}</Muted>
      ) : (
        <>
          <GlassCard glow backdrop={data.cover_path ? coverUrl(data) : undefined}>
            <View style={{ alignItems: 'flex-start' }}>
              <BookCover book={data} size={160} downloaded={download?.state === 'complete'} />
            </View>

            {series.data ? (
              <Pressable onPress={() => router.push(`/series/${series.data?.id}`)} style={{ marginTop: spacing.md }}>
                <Eyebrow>{seriesLabel(series.data.title, data.series_position)}</Eyebrow>
              </Pressable>
            ) : (
              <Eyebrow style={{ marginTop: spacing.md }}>{t('mobile.book.eyebrow')}</Eyebrow>
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
              <Badge variant="outline" icon={Languages} label={languageLabel(data.language, languages.data)} />
              {status === 'finished' ? (
                <Badge variant="highlight" icon={CheckCircle2} label={finishedAt ? t('mobile.book.finishedOn', { date: formatDate(finishedAt) }) : t('mobile.book.finished')} />
              ) : status === 'started' ? (
                <Badge icon={Headphones} label={t('mobile.book.inProgress')} />
              ) : null}
              {data.internal_rating ? <Badge variant="outline" icon={Star} label={`${data.internal_rating}/5`} /> : null}
            </View>

            <ActionRow style={{ marginTop: spacing.lg }}>
              <Button
                size="lg"
                icon={isPlayingBook ? Pause : Play}
                label={
                  isPlayingBook
                    ? t('mobile.actions.pause')
                    : isOpenBook
                      ? t('mobile.actions.play')
                      : resumeAt != null
                        ? t('mobile.book.resumeAt', { time: formatClock(resumeAt) })
                        : t('mobile.actions.play')
                }
                onPress={() => (isOpenBook ? void player.toggle() : void player.playBook(data.id))}
                disabled={player.loading}
              />
              {player.session && !inSession ? (
                <Button variant="outline" size="lg" icon={ListPlus} label={t('mobile.actions.addToSession')} onPress={() => void player.addToSession({ bookIds: [data.id] })} />
              ) : null}
              {status === 'finished' ? (
                <Button variant="outline" size="lg" icon={RotateCcw} label={t('mobile.book.unmark')} onPress={() => changeStatus(false)} disabled={setFinished.isPending} />
              ) : (
                <Button
                  variant="outline"
                  size="lg"
                  icon={CheckCircle2}
                  label={t('mobile.book.markFinished')}
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
                <SectionTitle>{t('mobile.book.onPhone')}</SectionTitle>
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
            <SectionTitle style={{ marginBottom: spacing.sm }}>{t('mobile.book.about')}</SectionTitle>
            {data.description ? <ExpandableText text={data.description} /> : <Muted size={14}>{t('mobile.book.noDescription')}</Muted>}
            <Muted size={12} style={{ marginTop: spacing.md }}>
              {chapterCount(data.chapter_count)} · {t('mobile.book.added', { date: formatDate(data.created_at) })}
            </Muted>
          </View>

          <ChapterList key={data.id} book={data} />

          {moreByAuthor.length > 0 && mainAuthor ? (
            <View style={{ marginTop: spacing.xl }}>
              <SectionTitle style={{ marginBottom: spacing.md }}>{t('mobile.book.moreByAuthor', { name: mainAuthor.name })}</SectionTitle>
              <BookGrid books={moreByAuthor} />
            </View>
          ) : null}
        </>
      )}
    </Screen>
  )
}

function DownloadButton({ bookId, state }: { bookId: string; state?: string }) {
  const { t } = useTranslation()
  if (state === 'complete') {
    return <Button variant="outline" icon={Trash2} label={t('common.delete')} onPress={() => void downloadManager.remove(bookId)} />
  }
  if (state === 'downloading' || state === 'queued') {
    return <Button variant="outline" label={t('mobile.actions.pause')} onPress={() => void downloadManager.pause(bookId)} />
  }
  return (
    <Button
      icon={Download}
      label={state === 'paused' ? t('mobile.actions.resume') : t('mobile.book.download')}
      onPress={() => void downloadManager.enqueue(bookId).catch((error: Error) => toast.error(error.message))}
    />
  )
}

function describeDownload(state: string | undefined, done: number, total: number, size: number, error: string): string {
  switch (state) {
    case 'complete':
      return t('mobile.book.downloadState.complete', { size: formatBytes(done) })
    case 'downloading':
      return t('mobile.downloads.state.downloading', { done: formatBytes(done), total: formatBytes(total) })
    case 'queued':
      return t('mobile.downloads.state.queued')
    case 'paused':
      return error ? t('mobile.downloads.state.pausedWithReason', { reason: error }) : t('mobile.downloads.state.paused')
    case 'error':
      return t('mobile.book.downloadState.error', { error })
    default:
      return size > 0 ? t('mobile.book.downloadState.available', { size: formatBytes(size) }) : t('mobile.book.downloadState.streaming')
  }
}

const styles = StyleSheet.create({
  authors: { flexDirection: 'row', flexWrap: 'wrap', marginTop: spacing.sm },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm, marginTop: spacing.md },
  card: { borderWidth: 1, borderRadius: radius['2xl'], padding: spacing.md, marginTop: spacing.lg, gap: spacing.sm },
  track: { height: 4, borderRadius: 2, overflow: 'hidden' },
  fill: { height: 4 },
})
