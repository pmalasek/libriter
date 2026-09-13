import { useMemo } from 'react'
import { Link } from 'react-router'
import { useAuthors, useBooks } from '@/api/hooks'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Card, CardContent } from '@/components/ui/card'
import { bookCount } from '@/lib/format'

export function AuthorsPage() {
  const authors = useAuthors()
  const books = useBooks()

  const countByAuthor = useMemo(() => {
    const counts = new Map<string, number>()
    for (const book of books.data ?? []) {
      for (const author of book.authors ?? []) {
        counts.set(author.id, (counts.get(author.id) ?? 0) + 1)
      }
    }
    return counts
  }, [books.data])

  if (authors.isPending) {
    return (
      <>
        <PageHeader title="Autoři" />
        <LoadingList />
      </>
    )
  }

  if (authors.isError) {
    return (
      <>
        <PageHeader title="Autoři" />
        <ErrorState error={authors.error} onRetry={() => void authors.refetch()} />
      </>
    )
  }

  return (
    <>
      <PageHeader title="Autoři" description={`${authors.data.length} celkem`} />

      {authors.data.length === 0 ? (
        <EmptyState
          title="Zatím žádní autoři"
          description="Autoři vznikají automaticky při načtení audio souborů scannerem."
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {authors.data.map((author) => (
            <Link key={author.id} to={`/authors/${author.id}`} className="group">
              <Card className="transition-colors group-hover:border-ring/50">
                <CardContent>
                  <p className="truncate font-medium">{author.name}</p>
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    {bookCount(countByAuthor.get(author.id) ?? 0)}
                  </p>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </>
  )
}
