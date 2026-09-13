import { Link } from 'react-router'
import type { Book } from '@/api/types'
import { CoverPlaceholder } from '@/components/CoverPlaceholder'
import { authorNames, formatDuration } from '@/lib/format'

export function BookCard({ book }: { book: Book }) {
  return (
    <Link
      to={`/books/${book.id}`}
      className="group block rounded-xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <CoverPlaceholder className="transition-opacity group-hover:opacity-80" />
      <div className="mt-2 space-y-0.5">
        <p className="line-clamp-2 text-sm font-medium leading-snug">{book.title}</p>
        <p className="truncate text-xs text-muted-foreground">{authorNames(book.authors)}</p>
        <p className="text-xs text-muted-foreground">{formatDuration(book.duration_seconds)}</p>
      </div>
    </Link>
  )
}
