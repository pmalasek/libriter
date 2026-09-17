import { ArrowLeftIcon, PencilIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useParams } from 'react-router'
import { useAuthor, useBooks, useSeriesTitle } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { canEdit } from '@/auth/permissions'
import { AuthorEditDialog } from '@/components/AuthorEditDialog'
import { AuthorImage, lifeYears } from '@/components/AuthorImage'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { ExpandableText } from '@/components/ExpandableText'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { bookCount } from '@/lib/format'
import { sortBooks } from '@/lib/sorting'

export function AuthorDetailPage() {
  const { id = '' } = useParams()
  const author = useAuthor(id)
  const books = useBooks()
  const { user } = useAuth()
  const [editing, setEditing] = useState(false)
  const seriesTitle = useSeriesTitle()

  // Knihy autora řadíme jako hlavní seznam: série pohromadě v pořadí dílů,
  // ostatní podle názvu.
  const authorBooks = useMemo(() => {
    const mine = (books.data ?? []).filter((book) => book.authors?.some((a) => a.id === id))
    return sortBooks(mine, 'title', 'asc', seriesTitle)
  }, [books.data, id, seriesTitle])

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

      <section className="glass inset-shadow-glass mb-8 rounded-3xl bg-brand-glow p-6 shadow-glass ring-1 ring-glass-edge md:p-8">
        {/* Stejně jako u série: na úzkém obsahu fotka nad textem, ne vedle. */}
        <div className="flex flex-col items-start gap-6 @md:flex-row">
          <AuthorImage
            key={author.data.id}
            author={author.data}
            className="size-28 shrink-0 shadow-glass-lg ring-4 ring-white/40 md:size-36 dark:ring-white/10"
          />
          <div className="min-w-0 flex-1">
            <PageHeader
              className="mb-0"
              eyebrow="Autor"
              title={author.data.name}
              description={[lifeYears(author.data), bookCount(authorBooks.length)]
                .filter(Boolean)
                .join(' · ')}
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
              <ExpandableText text={author.data.bio} className="mt-4 max-w-3xl" />
            ) : null}
          </div>
        </div>
      </section>

      <BookGrid books={authorBooks} emptyTitle="U tohoto autora nejsou žádné knihy" />

      <AuthorEditDialog author={author.data} open={editing} onOpenChange={setEditing} />
    </>
  )
}
