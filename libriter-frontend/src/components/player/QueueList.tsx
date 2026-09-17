import { useQueries } from '@tanstack/react-query'
import { AudioLinesIcon, ChevronDownIcon, PauseIcon, PlayIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { chaptersQuery, useBooks } from '@/api/hooks'
import type { Book, Chapter } from '@/api/types'
import { BookCover } from '@/components/BookCover'
import { Badge } from '@/components/ui/badge'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Skeleton } from '@/components/ui/skeleton'
import { chapterCount, formatClock, formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'
import { usePlayer } from '@/player/playerContext'

/**
 * Obsah otevřeného poslechu: všechny jeho knihy i s jednotlivými soubory.
 * Stojí pod ovládáním v panelu Now Playing – na sloupci i v celoobrazovkovém
 * přehrávači.
 *
 * Kapitoly se stahují až tehdy, když je seznam vidět – u dlouhých audioknih
 * jde o stovky řádků na knihu.
 */
export function QueueList({
  enabled = true,
  autoScroll = true,
}: {
  enabled?: boolean
  /** Po zobrazení seznamu na něj odrolovat tak, aby byla vidět hrající kapitola. */
  autoScroll?: boolean
}) {
  const player = usePlayer()
  const books = useBooks()

  const items = player.session?.items ?? []

  const chapterQueries = useQueries({
    queries: items.map((item) => ({ ...chaptersQuery(item.book_id), enabled })),
  })

  const bookById = new Map((books.data ?? []).map((book) => [book.id, book]))

  return (
    <>
      {items.map((item, index) => (
        <QueueBook
          key={item.book_id}
          book={bookById.get(item.book_id)}
          order={index + 1}
          showOrder={items.length > 1}
          chapters={chapterQueries[index]?.data ?? []}
          loading={chapterQueries[index]?.isPending ?? false}
          autoScroll={autoScroll}
          onPlay={(chapterId) => {
            if (item.book_id) player.playBook(item.book_id, chapterId)
          }}
        />
      ))}
    </>
  )
}

function QueueBook({
  book,
  order,
  showOrder,
  chapters,
  loading,
  autoScroll,
  onPlay,
}: {
  book: Book | undefined
  order: number
  showOrder: boolean
  chapters: Chapter[]
  loading: boolean
  autoScroll: boolean
  onPlay: (chapterId: string) => void
}) {
  const player = usePlayer()
  const isCurrent = player.book?.id === book?.id
  // Odrolovat jen jednou za zobrazení seznamu. Panel se při každém otevření
  // připojí znovu, takže se to trefí pokaždé; při přepnutí kapitoly už ne –
  // seznam by uživateli ujel pod rukou.
  const scrolled = useRef(false)

  const revealPlaying = (el: HTMLButtonElement | null) => {
    if (!el || !autoScroll || scrolled.current) return
    scrolled.current = true
    el.scrollIntoView({ block: 'center' })
  }
  // Rozbalená je kniha, která hraje; ostatní by jen přidaly stovky řádků.
  const [open, setOpen] = useState(isCurrent)
  const [wasCurrent, setWasCurrent] = useState(isCurrent)

  // Když poslech přejde na další knihu, rozbalí se sama, ať je vidět, kde je.
  // Srovnání při renderu, ne v efektu – jinak by se panel překreslil dvakrát.
  if (isCurrent !== wasCurrent) {
    setWasCurrent(isCurrent)
    if (isCurrent) setOpen(true)
  }

  if (!book) {
    return <p className="px-3 py-2.5 text-sm text-muted-foreground">Kniha už není v knihovně.</p>
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="mb-1">
      <CollapsibleTrigger asChild>
        <button
          type="button"
          className={cn(
            'flex w-full items-center gap-3 rounded-xl p-2 text-left transition-colors hover:bg-foreground/6',
            isCurrent && 'bg-primary/10',
          )}
        >
          <BookCover book={book} key={book.id} lift={false} className="size-11 rounded-lg" />
          <span className="min-w-0 flex-1">
            <span className="flex items-center gap-2">
              <span className="truncate font-medium">
                {showOrder ? `${order}. ` : null}
                {book.title}
              </span>
              {isCurrent ? (
                <Badge variant="brand">
                  <AudioLinesIcon />
                  Hraje
                </Badge>
              ) : null}
            </span>
            <span className="block truncate text-xs text-muted-foreground">
              {chapterCount(book.chapter_count)} · {formatDuration(book.duration_seconds)}
            </span>
          </span>
          <ChevronDownIcon
            className={cn(
              'size-4 shrink-0 text-muted-foreground transition-transform',
              open && 'rotate-180',
            )}
          />
        </button>
      </CollapsibleTrigger>

      <CollapsibleContent>
        {loading ? (
          <div className="space-y-1.5 p-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : chapters.length === 0 ? (
          <p className="px-3 py-2 text-sm text-muted-foreground">Kniha nemá načtené kapitoly.</p>
        ) : (
          <ol className="pb-1 pl-3">
            {chapters.map((chapter, index) => {
              const playingThis = isCurrent && player.chapter?.id === chapter.id
              return (
                <li key={chapter.id}>
                  <button
                    type="button"
                    ref={playingThis ? revealPlaying : undefined}
                    onClick={() => (playingThis ? player.toggle() : onPlay(chapter.id))}
                    className={cn(
                      'flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-foreground/6',
                      playingThis && 'bg-primary/10',
                    )}
                  >
                    <span className="w-6 shrink-0 text-xs text-muted-foreground tabular-nums">
                      {index + 1}.
                    </span>
                    <span className="min-w-0 flex-1">
                      <span
                        className={cn(
                          'block truncate text-sm',
                          playingThis && 'font-medium text-primary',
                        )}
                      >
                        {chapter.title}
                      </span>
                      {/* Název souboru pomáhá poznat špatně seřazené kapitoly. */}
                      <span className="block truncate font-mono text-[11px] text-muted-foreground">
                        {chapter.file_name}
                      </span>
                    </span>
                    <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                      {formatClock(chapter.duration_seconds)}
                    </span>
                    <span className="flex size-6 shrink-0 items-center justify-center text-muted-foreground">
                      {playingThis && player.playing ? (
                        <PauseIcon className="size-4" />
                      ) : (
                        <PlayIcon className="size-4" />
                      )}
                    </span>
                  </button>
                </li>
              )
            })}
          </ol>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}
