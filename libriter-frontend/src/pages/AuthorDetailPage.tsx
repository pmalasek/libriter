import { ArrowLeftIcon } from 'lucide-react'
import { useMemo } from 'react'
import { Link, useParams } from 'react-router'
import { useAuthor, useBooks } from '@/api/hooks'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { bookCount } from '@/lib/format'

export function AuthorDetailPage() {
  const { id = '' } = useParams()
  const author = useAuthor(id)
  const books = useBooks()

  const authorBooks = useMemo(
    () => (books.data ?? []).filter((book) => book.author_id === id),
    [books.data, id],
  )

  if (author.isPending) return <LoadingGrid count={4} />
  if (author.isError) {
    return <ErrorState error={author.error} onRetry={() => void author.refetch()} />
  }

  return (
    <>
      <Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
        <Link to="/authors">
          <ArrowLeftIcon />
          Zpět na autory
        </Link>
      </Button>

      <PageHeader title={author.data.name} description={bookCount(authorBooks.length)} />

      {author.data.bio ? (
        <p className="mb-8 max-w-3xl whitespace-pre-line text-sm leading-relaxed">
          {author.data.bio}
        </p>
      ) : null}

      <BookGrid books={authorBooks} emptyTitle="U tohoto autora nejsou žádné knihy" />
    </>
  )
}
