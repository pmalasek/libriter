import type { Author, Book } from '@/api/types'
import { BookCard } from '@/components/BookCard'
import { EmptyState } from '@/components/EmptyState'

export function BookGrid({
  books,
  authorsById,
  emptyTitle = 'Žádné knihy',
  emptyDescription,
}: {
  books: Book[]
  authorsById?: Map<string, Author>
  emptyTitle?: string
  emptyDescription?: string
}) {
  if (books.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />
  }

  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {books.map((book) => (
        <BookCard key={book.id} book={book} authorName={authorsById?.get(book.author_id)?.name} />
      ))}
    </div>
  )
}
