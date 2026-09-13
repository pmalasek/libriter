import type { Book } from '@/api/types'
import { BookCard } from '@/components/BookCard'
import { EmptyState } from '@/components/EmptyState'

export function BookGrid({
  books,
  emptyTitle = 'Žádné knihy',
  emptyDescription,
}: {
  books: Book[]
  emptyTitle?: string
  emptyDescription?: string
}) {
  if (books.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />
  }

  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {books.map((book) => (
        <BookCard key={book.id} book={book} />
      ))}
    </div>
  )
}
