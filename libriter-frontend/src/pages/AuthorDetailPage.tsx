import { ArrowLeftIcon, PencilIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router'
import { useAuthor, useBooks } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { canEdit } from '@/auth/permissions'
import { AuthorEditDialog } from '@/components/AuthorEditDialog'
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
  const { user } = useAuth()
  const [editing, setEditing] = useState(false)

  const authorBooks = useMemo(
    () => (books.data ?? []).filter((book) => book.authors?.some((a) => a.id === id)),
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

      <PageHeader
        title={author.data.name}
        description={bookCount(authorBooks.length)}
        actions={
          canEdit(user) ? (
            <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
              <PencilIcon />
              Upravit
            </Button>
          ) : null
        }
      />

      {author.data.bio ? (
        <p className="mb-8 max-w-3xl whitespace-pre-line text-sm leading-relaxed">
          {author.data.bio}
        </p>
      ) : null}

      <BookGrid books={authorBooks} emptyTitle="U tohoto autora nejsou žádné knihy" />

      <AuthorEditDialog author={author.data} open={editing} onOpenChange={setEditing} />
    </>
  )
}
