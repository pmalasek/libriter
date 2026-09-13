import { useMemo, useState } from 'react'
import { useAuthorsById, useBooks } from '@/api/hooks'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Input } from '@/components/ui/input'
import { bookCount } from '@/lib/format'

export function BooksPage() {
  const [query, setQuery] = useState('')
  const books = useBooks()
  const authors = useAuthorsById()

  const filtered = useMemo(() => {
    const all = books.data ?? []
    const needle = query.trim().toLocaleLowerCase('cs')
    if (!needle) return all

    return all.filter((book) => {
      const author = authors.map.get(book.author_id)?.name ?? ''
      return (
        book.title.toLocaleLowerCase('cs').includes(needle) ||
        author.toLocaleLowerCase('cs').includes(needle)
      )
    })
  }, [books.data, authors.map, query])

  if (books.isPending) {
    return (
      <>
        <PageHeader title="Knihy" />
        <LoadingGrid />
      </>
    )
  }

  if (books.isError) {
    return (
      <>
        <PageHeader title="Knihy" />
        <ErrorState error={books.error} onRetry={() => void books.refetch()} />
      </>
    )
  }

  return (
    <>
      <PageHeader
        title="Knihy"
        description={bookCount(books.data.length)}
        actions={
          <Input
            type="search"
            placeholder="Hledat podle názvu nebo autora…"
            className="w-56"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        }
      />
      <BookGrid
        books={filtered}
        authorsById={authors.map}
        emptyTitle={query ? 'Nic nenalezeno' : 'Zatím žádné knihy'}
        emptyDescription={
          query
            ? 'Zkuste jiný hledaný výraz.'
            : 'Přidejte audio soubory do adresáře AUDIO_ROOT – scanner je načte automaticky.'
        }
      />
    </>
  )
}
