import {
  ArrowLeftIcon,
  CalendarIcon,
  ClockIcon,
  InfoIcon,
  LanguagesIcon,
  MicIcon,
  PencilIcon,
  PlayIcon,
  StarIcon,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { useBook, useBooks, useSeriesOne, useSeriesTitle } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { canEdit } from '@/auth/permissions'
import { BookEditDialog } from '@/components/BookEditDialog'
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
import { chapterCount, formatDate, formatDuration } from '@/lib/format'
import { sortBooks, useBookListPrefs } from '@/lib/sorting'

export function BookDetailPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const book = useBook(id)
  const series = useSeriesOne(book.data?.series_id ?? '')
  const allBooks = useBooks()
  const { user } = useAuth()
  const [editing, setEditing] = useState(false)
  const { sortKey, sortDir } = useBookListPrefs()
  const seriesTitle = useSeriesTitle()

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
        <Link to="/">
          <ArrowLeftIcon />
          Zpět na knihy
        </Link>
      </Button>

      {/* Hlavička s rozmazanou obálkou v pozadí – dává detailu barvu knihy. */}
      <section className="relative overflow-hidden rounded-3xl bg-brand-glow p-6 md:p-8">
        {data.cover_path ? (
          <>
            <img
              src={coverUrl(data)}
              alt=""
              aria-hidden
              className="absolute inset-0 size-full scale-125 object-cover opacity-40 blur-3xl dark:opacity-30"
            />
            {/* Závoj drží text čitelný i nad pestrou obálkou. */}
            <div className="absolute inset-0 bg-linear-to-b from-background/70 via-background/80 to-background" />
          </>
        ) : null}

        <div className="relative grid gap-8 md:grid-cols-[240px_1fr]">
          <div className="max-w-[240px]">
            <BookCover key={data.id} book={data} lift={false} className="shadow-2xl" />
          </div>

          <div className="min-w-0">
            {series.data ? (
              <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
                <Link to={`/series/${series.data.id}`} className="underline-offset-4 hover:underline">
                  {series.data.title}
                </Link>
                {data.series_position != null ? ` · ${data.series_position}. díl` : null}
              </p>
            ) : (
              <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
                Audiokniha
              </p>
            )}

            <h1 className="font-heading text-3xl font-bold tracking-tight md:text-4xl">
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
                {data.language.toUpperCase()}
              </Badge>
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
                  <Button variant="outline" size="icon-sm" aria-label="Informace o záznamu">
                    <InfoIcon />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-80 p-3">
                  <dl className="space-y-2.5 text-sm">
                    <div>
                      <dt className="text-muted-foreground">Kapitoly</dt>
                      <dd className="tabular-nums">{chapterCount(data.chapter_count)}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">Soubory</dt>
                      {/* Dlouhou cestu je lepší zalomit než oříznout – jinak
                          není poznat, o kterou složku jde. */}
                      <dd className="font-mono text-xs break-all">{data.file_path}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">Přidáno</dt>
                      <dd className="tabular-nums">{formatDate(data.created_at)}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">Naposledy změněno</dt>
                      <dd className="tabular-nums">{formatDate(data.updated_at)}</dd>
                    </div>
                  </dl>
                </PopoverContent>
              </Popover>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-2">
              <Button
                size="lg"
                disabled
                title="Přehrávání zatím není k dispozici – backend pro něj ještě nemá endpoint."
              >
                <PlayIcon />
                Přehrát
              </Button>
              {canEdit(user) ? (
                <Button variant="outline" size="lg" onClick={() => setEditing(true)}>
                  <PencilIcon />
                  Upravit
                </Button>
              ) : null}
            </div>
          </div>
        </div>
      </section>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>O knize</CardTitle>
        </CardHeader>
        <CardContent>
          {data.description ? (
            <ExpandableText text={data.description} />
          ) : (
            <p className="text-sm text-muted-foreground">Popis není k dispozici.</p>
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
            Další knihy autora {mainAuthor.name}
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
    </>
  )
}
