import { XIcon } from 'lucide-react'
import { Link } from 'react-router'
import { BookCover, coverUrl } from '@/components/BookCover'
import { Button } from '@/components/ui/button'
import { authorsLabel } from '@/lib/format'
import { cn } from '@/lib/utils'
import { playerSubtitle, usePlayer } from '@/player/playerContext'
import { QueueList } from './QueueList'
import { SeekBar } from './SeekBar'
import { SessionMenu } from './SessionMenu'
import { SpeedSelect } from './SpeedSelect'
import { TransportControls } from './TransportControls'
import { VolumeControl } from './VolumeControl'

/**
 * Co právě hraje: obálka, název, jezdec, ovládání a obsah poslechu pod tím.
 * Na širokém displeji stojí natrvalo ve sloupci u pravé hrany
 * (NowPlayingColumn), na užším se vytáhne přes celou obrazovku (PlayerSheet).
 */
export function NowPlayingPanel({
  /** Zavře panel, který obsah drží – klepnutí na odkaz nesmí nechat panel otevřený. */
  onNavigate,
  className,
}: {
  onNavigate?: () => void
  className?: string
}) {
  const player = usePlayer()
  const book = player.book
  const subtitle = playerSubtitle(player)

  return (
    <div className={cn('relative isolate flex min-h-0 flex-1 flex-col', className)}>
      {/* Záře z obálky nad panelem; maska ji nechá vyznít do ztracena. */}
      {book?.cover_path ? (
        <img
          src={coverUrl(book)}
          alt=""
          aria-hidden
          decoding="async"
          className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-72 w-full scale-125 object-cover opacity-50 blur-3xl [mask-image:radial-gradient(60%_60%_at_50%_35%,black,transparent)] dark:opacity-40"
        />
      ) : null}

      {/* Obálka a ovládání drží místo; rolují se jen kapitoly pod nimi. */}
      <div className="shrink-0 px-5 pt-5">
        {book ? (
          <Link to={`/books/${book.id}`} onClick={onNavigate} className="mx-auto block w-fit">
            <BookCover
              book={book}
              key={book.id}
              lift={false}
              className="w-44 shadow-glass-lg sm:w-52"
            />
          </Link>
        ) : (
          <div className="mx-auto aspect-square w-44 rounded-2xl bg-foreground/8 sm:w-52" />
        )}

        <div className="mt-5 shrink-0 text-center">
          <p className="font-heading text-xl font-semibold tracking-tight text-balance">
            {book ? (
              <Link
                to={`/books/${book.id}`}
                onClick={onNavigate}
                className="line-clamp-2 hover:underline"
              >
                {book.title}
              </Link>
            ) : (
              'Načítání…'
            )}
          </p>
          {book ? (
            <p className="mt-1 line-clamp-1 text-sm text-muted-foreground">
              {authorsLabel(book.authors)}
            </p>
          ) : null}
          <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{subtitle}</p>
        </div>

        <div className="mt-6 shrink-0">
          <SeekBar size="lg" />
        </div>

        <div className="mt-4 flex shrink-0 items-center justify-center">
          <TransportControls size="lg" />
        </div>

        <div className="mt-5 flex shrink-0 items-center justify-between gap-2">
          <SpeedSelect className="w-24" size="default" />
          <div className="flex items-center gap-1">
            <VolumeControl />
            <SessionMenu triggerClassName="size-10" />
            <Button
              variant="ghost"
              size="icon"
              className="size-10"
              onClick={() => {
                onNavigate?.()
                player.close()
              }}
              aria-label="Zavřít přehrávač"
              title="Zavřít přehrávač (poslech zůstane uložený)"
            >
              <XIcon />
            </Button>
          </div>
        </div>
      </div>

      {/* Obsah poslechu rovnou pod ovládáním – nemusí se kvůli němu nic
          otevírat. Roluje se jen tenhle blok, obálka a tlačítka zůstanou
          na očích i u knihy s třiceti kapitolami. */}
      <div className="hairline-t mt-4 flex min-h-0 flex-1 flex-col px-5 pt-3">
        <p className="mb-2 shrink-0 px-2 text-xs font-medium tracking-wide text-muted-foreground uppercase">
          Obsah poslechu
        </p>
        <div className="min-h-0 flex-1 overflow-y-auto pb-5">
          <QueueList />
        </div>
      </div>
    </div>
  )
}
