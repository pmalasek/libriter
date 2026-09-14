import {
  ArrowLeftIcon,
  CalendarIcon,
  ClockIcon,
  LanguagesIcon,
  MicIcon,
  PencilIcon,
  StarIcon,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { useBook, useBooks, useSeriesOne } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { canEdit } from '@/auth/permissions'
import { BookEditDialog } from '@/components/BookEditDialog'
import { BookGrid } from '@/components/BookGrid'
import { BookCover } from '@/components/BookCover'
import { ErrorState } from '@/components/ErrorState'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate, formatDuration } from '@/lib/format'
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

  // Další kniha pro „Uložit a další“ – ve stejném pořadí, jaké má seznam knih.
  const nextBook = useMemo(() => {
    const ordered = sortBooks(allBooks.data ?? [], sortKey, sortDir)
    const index = ordered.findIndex((b) => b.id === id)
    return index >= 0 ? ordered[index + 1] : undefined
  }, [allBooks.data, id, sortKey, sortDir])

  // "Další knihy autora" bereme podle hlavního (prvního) autora knihy.
  const mainAuthor = book.data?.authors?.[0]

  const moreByAuthor = useMemo(() => {
    if (!book.data || !mainAuthor) return []
    return (allBooks.data ?? []).filter(
      (b) => b.id !== book.data.id && b.authors?.some((a) => a.id === mainAuthor.id),
    )
  }, [allBooks.data, book.data, mainAuthor])

  if (book.isPending) {
    return (
      <div className="grid gap-8 md:grid-cols-[220px_1fr]">
        <Skeleton className="aspect-square w-full rounded-lg" />
        <div className="space-y-3">
          <Skeleton className="h-8 w-2/3" />
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

      <div className="grid gap-8 md:grid-cols-[220px_1fr]">
        <div className="max-w-[220px]">
          <BookCover key={data.id} book={data} />
        </div>

        <div className="min-w-0">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <h1 className="font-heading text-2xl font-semibold tracking-tight">{data.title}</h1>
            {canEdit(user) ? (
              <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
                <PencilIcon />
                Upravit
              </Button>
            ) : null}
          </div>

          {data.authors?.length ? (
            <p className="mt-1 text-muted-foreground">
              {data.authors.map((author, index) => (
                <span key={author.id}>
                  {index > 0 ? ', ' : null}
                  <Link to={`/authors/${author.id}`} className="underline-offset-4 hover:underline">
                    {author.name}
                  </Link>
                </span>
              ))}
            </p>
          ) : null}

          {series.data ? (
            <p className="mt-1 text-sm text-muted-foreground">
              <Link to={`/series/${series.data.id}`} className="underline-offset-4 hover:underline">
                {series.data.title}
              </Link>
              {data.series_position != null ? ` · ${data.series_position}. díl` : null}
            </p>
          ) : null}

          <div className="mt-4 flex flex-wrap gap-2">
            <Badge variant="secondary">
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
          </div>

          {data.description ? (
            <p className="mt-6 whitespace-pre-line text-sm leading-relaxed">{data.description}</p>
          ) : (
            <p className="mt-6 text-sm text-muted-foreground">Popis není k dispozici.</p>
          )}

          <Separator className="my-6" />

          <dl className="grid gap-2 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-muted-foreground">Přidáno</dt>
              <dd>{formatDate(data.created_at)}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Naposledy změněno</dt>
              <dd>{formatDate(data.updated_at)}</dd>
            </div>
          </dl>

          <p className="mt-6 text-xs text-muted-foreground">
            Přehrávání zatím není k dispozici – backend pro něj ještě nemá endpoint.
          </p>
        </div>
      </div>

      {moreByAuthor.length > 0 && mainAuthor ? (
        <section className="mt-10">
          <h2 className="font-heading mb-4 text-lg font-semibold">
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
