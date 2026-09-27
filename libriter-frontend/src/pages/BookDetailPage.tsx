import {
  ArrowLeftIcon,
  CalendarIcon,
  CheckCircle2Icon,
  ClockIcon,
  HeadphonesIcon,
  InfoIcon,
  LanguagesIcon,
  ListPlusIcon,
  MicIcon,
  PauseIcon,
  PencilIcon,
  PlayIcon,
  RotateCcwIcon,
  StarIcon,
  Trash2Icon,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'
import {
  useBook,
  useBookProgress,
  useBooks,
  useLanguages,
  useSeriesOne,
  useSeriesTitle,
  useDeleteBook,
  useSessions,
  useSetBookFinished,
} from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { canEdit, isAdmin } from '@/auth/permissions'
import { BookEditDialog } from '@/components/BookEditDialog'
import { DeleteBookDialog } from '@/components/DeleteBookDialog'
import { BookGrid } from '@/components/BookGrid'
import { BookCover, coverUrl } from '@/components/BookCover'
import { ChapterList } from '@/components/ChapterList'
import { ErrorState } from '@/components/ErrorState'
import { ExpandableText } from '@/components/ExpandableText'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { chapterCount, formatClock, formatDate, formatDuration, languageLabel } from '@/lib/format'
import { sortBooks, useBookListPrefs } from '@/lib/sorting'
import { usePlayer } from '@/player/playerContext'

export function BookDetailPage() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const book = useBook(id)
  const series = useSeriesOne(book.data?.series_id ?? '')
  const allBooks = useBooks()
  const { user } = useAuth()
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const { sortKey, sortDir } = useBookListPrefs()
  const seriesTitle = useSeriesTitle()
  const languages = useLanguages()
  const player = usePlayer()
  const sessions = useSessions()
  const progress = useBookProgress()
  const setFinished = useSetBookFinished()
  const deleteBook = useDeleteBook(id)

  // Stav knihy: doposlechnutá, rozposlouchaná, nebo zatím nic.
  const bookStatus = progress.status(id)
  const finishedAt = progress.map.get(id)?.finished_at

  function changeStatus(finished: boolean) {
    setFinished.mutate(
      { bookId: id, finished },
      {
        onSuccess: () =>
          toast.success(finished ? t('books.detail.markedFinished') : t('books.detail.unmarked')),
        onError: (error) => toast.error(error.message),
      },
    )
  }

  function handleDelete(deleteFiles: boolean) {
    deleteBook.mutate(deleteFiles, {
      onSuccess: (result) => {
        setDeleting(false)
        toast.success(
          deleteFiles
            ? t('books.detail.deletedWithFiles', {
                title: result.title,
                count: result.deleted_files,
              })
            : t('books.detail.deleted', { title: result.title }),
        )
        navigate('/books')
      },
      onError: (error) => {
        setDeleting(false)
        toast.error(error.message)
      },
    })
  }

  // Rozposlouchaná pozice knihy – z libovolného poslechu, který ji obsahuje.
  const started = useMemo(
    () => (sessions.data ?? []).flatMap((s) => s.items).find((item) => item.book_id === id),
    [id, sessions.data],
  )
  const inSession = started !== undefined
  const resumeAt = started && started.position_seconds > 0 ? started.position_seconds : null

  // Kniha, kterou drží přehrávač – pak tlačítko ovládá přehrávání, ne otevření.
  const isOpenBook = player.book?.id === id
  const isPlayingBook = isOpenBook && player.playing

  // Další kniha pro „Uložit a další“ – ve stejném pořadí, jaké má seznam knih.
  const nextBook = useMemo(() => {
    const ordered = sortBooks(allBooks.data ?? [], sortKey, sortDir, seriesTitle)
    const index = ordered.findIndex((b) => b.id === id)
    return index >= 0 ? ordered[index + 1] : undefined
  }, [allBooks.data, id, sortKey, sortDir, seriesTitle])

  // "Další knihy autora" bereme podle hlavního (prvního) autora knihy.
  const mainAuthor = book.data?.authors?.[0]

  const moreByAuthor = useMemo(() => {
    if (!book.data || !mainAuthor) return []
    const byAuthor = (allBooks.data ?? []).filter(
      (b) => b.id !== book.data.id && b.authors?.some((a) => a.id === mainAuthor.id),
    )
    return sortBooks(byAuthor, 'title', 'asc', seriesTitle)
  }, [allBooks.data, book.data, mainAuthor, seriesTitle])

  if (book.isPending) {
    return (
      <div className="grid gap-8 md:grid-cols-[240px_1fr]">
        <Skeleton className="aspect-square w-full rounded-2xl" />
        <div className="space-y-3">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-4 w-1/3" />
          <Skeleton className="h-24 w-full" />
        </div>
      </div>
    )
  }

  if (book.isError) {
    return <ErrorState error={book.error} onRetry={() => void book.refetch()} />
  }

  const { data } = book

  return (
    <>
      <Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
        <Link to="/books">
          <ArrowLeftIcon />
          {t('books.detail.back')}
        </Link>
      </Button>

      {/* Hlavička s rozmazanou obálkou v pozadí – dává detailu barvu knihy.
          Pozadí aplikace barví hraná kniha, tahle vrstva ta prohlížená. */}
      <section className="glass inset-shadow-glass relative isolate overflow-hidden rounded-3xl bg-brand-glow p-6 shadow-glass ring-1 ring-glass-edge md:p-8">
        {data.cover_path ? (
          <img
            src={coverUrl(data)}
            alt=""
            aria-hidden
            decoding="async"
            className="absolute inset-0 -z-10 size-full scale-125 object-cover opacity-25 blur-3xl dark:opacity-20"
          />
        ) : null}

        <div className="relative grid gap-8 md:grid-cols-[240px_1fr]">
          <div className="max-w-[240px]">
            <BookCover key={data.id} book={data} lift={false} className="shadow-glass-lg" />
          </div>

          <div className="min-w-0">
            {series.data ? (
              <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
                <Link to={`/series/${series.data.id}`} className="underline-offset-4 hover:underline">
                  {series.data.title}
                </Link>
                {data.series_position != null
                  ? ` · ${t('format.seriesPart', { position: data.series_position })}`
                  : null}
              </p>
            ) : (
              <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
                {t('books.detail.audiobook')}
              </p>
            )}

            <h1 className="font-heading text-3xl font-semibold tracking-tight md:text-4xl">
              {data.title}
            </h1>

            {data.authors?.length ? (
              <p className="mt-2 text-lg">
                {data.authors.map((author, index) => (
                  <span key={author.id}>
                    {index > 0 ? ', ' : null}
                    <Link
                      to={`/authors/${author.id}`}
                      className="font-medium text-primary underline-offset-4 hover:underline"
                    >
                      {author.name}
                    </Link>
                  </span>
                ))}
              </p>
            ) : null}

            <div className="mt-5 flex flex-wrap items-center gap-2">
              <Badge variant="highlight">
                <ClockIcon />
                {formatDuration(data.duration_seconds)}
              </Badge>
              {data.narrator ? (
                <Badge variant="secondary">
                  <MicIcon />
                  {data.narrator}
                </Badge>
              ) : null}
              {data.published_year ? (
                <Badge variant="secondary">
                  <CalendarIcon />
                  {data.published_year}
                </Badge>
              ) : null}
              <Badge variant="outline">
                <LanguagesIcon />
                {languageLabel(data.language, languages.data)}
              </Badge>
              {bookStatus === 'finished' ? (
                <Badge variant="highlight">
                  <CheckCircle2Icon />
                  {finishedAt
                    ? t('books.detail.finishedOn', { date: formatDate(finishedAt) })
                    : t('books.detail.finished')}
                </Badge>
              ) : bookStatus === 'started' ? (
                <Badge variant="secondary">
                  <HeadphonesIcon />
                  {t('books.detail.started')}
                </Badge>
              ) : null}
              {data.internal_rating ? (
                <Badge variant="outline">
                  <StarIcon />
                  {data.internal_rating}/5
                </Badge>
              ) : null}

              {/* Údaje, které se týkají spíš záznamu než knihy samotné –
                  v hlavičce by jen odváděly pozornost. */}
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" size="icon-sm" aria-label={t('books.detail.recordInfo')}>
                    <InfoIcon />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-80 p-3">
                  <dl className="space-y-2.5 text-sm">
                    <div>
                      <dt className="text-muted-foreground">{t('books.detail.chapters')}</dt>
                      <dd className="tabular-nums">{chapterCount(data.chapter_count)}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">{t('books.detail.files')}</dt>
                      {/* Dlouhou cestu je lepší zalomit než oříznout – jinak
                          není poznat, o kterou složku jde. */}
                      <dd className="font-mono text-xs break-all">{data.file_path}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">{t('books.detail.added')}</dt>
                      <dd className="tabular-nums">{formatDate(data.created_at)}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">{t('books.detail.updated')}</dt>
                      <dd className="tabular-nums">{formatDate(data.updated_at)}</dd>
                    </div>
                  </dl>
                </PopoverContent>
              </Popover>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-2">
              {/* Knihu, která je právě v přehrávači, tohle tlačítko jen
                  pozastaví a rozjede – načítat ji znovu by zahodilo pozici. */}
              <Button
                size="lg"
                onClick={() => (isOpenBook ? player.toggle() : player.playBook(data.id))}
                disabled={player.loading}
              >
                {isPlayingBook ? <PauseIcon /> : <PlayIcon />}
                {isPlayingBook
                  ? t('books.detail.pause')
                  : isOpenBook
                    ? t('books.detail.play')
                    : resumeAt != null
                      ? t('books.detail.resume', { time: formatClock(resumeAt) })
                      : t('books.detail.play')}
              </Button>
              {player.session && !inSession ? (
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => player.addToSession({ bookIds: [data.id] })}
                  title={t('books.detail.addToSessionTitle')}
                >
                  <ListPlusIcon />
                  {t('books.detail.addToSession')}
                </Button>
              ) : null}
              {/* Ruční oprava stavu: kniha slyšená jinde, nebo omylem
                  dohraná do konce. Poslech ani pozici to nemění. */}
              {bookStatus === 'finished' ? (
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => changeStatus(false)}
                  disabled={setFinished.isPending}
                >
                  <RotateCcwIcon />
                  {t('books.detail.unmarkFinished')}
                </Button>
              ) : (
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => changeStatus(true)}
                  disabled={setFinished.isPending}
                >
                  <CheckCircle2Icon />
                  {t('books.detail.markFinished')}
                </Button>
              )}
              {canEdit(user) ? (
                <Button variant="outline" size="lg" onClick={() => setEditing(true)}>
                  <PencilIcon />
                  {t('common.edit')}
                </Button>
              ) : null}
              {isAdmin(user) ? (
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => setDeleting(true)}
                  disabled={deleteBook.isPending}
                >
                  <Trash2Icon />
                  {t('common.delete')}
                </Button>
              ) : null}
            </div>
          </div>
        </div>
      </section>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>{t('books.detail.about')}</CardTitle>
        </CardHeader>
        <CardContent>
          {data.description ? (
            <ExpandableText text={data.description} />
          ) : (
            <p className="text-sm text-muted-foreground">{t('books.detail.noDescription')}</p>
          )}
        </CardContent>
      </Card>

      {/* key resetuje rozbalení i rozepsané pořadí při přechodu na další knihu –
          stránka se nepřemountuje, jen se změní parametr v URL. */}
      <ChapterList key={data.id} book={data} />

      {moreByAuthor.length > 0 && mainAuthor ? (
        <section className="mt-12">
          <h2 className="font-heading mb-5 flex items-center gap-2.5 text-xl font-bold">
            <span className="size-2 rounded-full bg-primary" />
            {t('books.detail.moreByAuthor', { name: mainAuthor.name })}
          </h2>
          <BookGrid books={moreByAuthor} />
        </section>
      ) : null}

      {/* Dialog zůstává otevřený i po přechodu na další knihu – stránka se
          nepřemountuje, jen se změní parametr v URL. */}
      <BookEditDialog
        book={data}
        open={editing}
        onOpenChange={setEditing}
        onSaveAndNext={nextBook ? () => navigate(`/books/${nextBook.id}`) : undefined}
      />

      <DeleteBookDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={data.title}
        pending={deleteBook.isPending}
        onConfirm={handleDelete}
      />
    </>
  )
}
