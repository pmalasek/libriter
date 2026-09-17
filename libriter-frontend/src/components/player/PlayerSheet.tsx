import { ChevronDownIcon, XIcon } from 'lucide-react'
import { Link } from 'react-router'
import { BookCover } from '@/components/BookCover'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { authorsLabel } from '@/lib/format'
import { usePlayer } from '@/player/playerContext'
import { PlayerQueue } from './PlayerQueue'
import { SeekBar } from './SeekBar'
import { SessionMenu } from './SessionMenu'
import { SpeedSelect } from './SpeedSelect'
import { TransportControls } from './TransportControls'

/**
 * Rozbalený přehrávač pro mobil a tablet. Na úzké obrazovce se do lišty
 * vejde jen název a pár tlačítek, zbytek ovládání i celá informace o knize
 * je tady – panel se vytáhne klepnutím na lištu.
 */
export function PlayerSheet({
  open,
  onOpenChange,
  subtitle,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Kapitola a pořadí v poslechu; počítá je lišta, ať je popisek všude stejný. */
  subtitle: string
}) {
  const player = usePlayer()
  const book = player.book

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="bottom"
        showCloseButton={false}
        className="max-h-[92svh] gap-0 overflow-y-auto rounded-t-3xl px-5 pt-4 pb-[max(1.25rem,env(safe-area-inset-bottom))]"
      >
        <SheetHeader className="sr-only p-0">
          <SheetTitle>Přehrávač</SheetTitle>
        </SheetHeader>

        {/* Šipka dolů panel zase sbalí zpátky do lišty. */}
        <button
          type="button"
          onClick={() => onOpenChange(false)}
          className="mb-3 flex w-full shrink-0 items-center justify-center py-1 text-muted-foreground hover:text-foreground"
          aria-label="Sbalit přehrávač"
        >
          <ChevronDownIcon className="size-5" />
        </button>

        {book ? (
          <Link
            to={`/books/${book.id}`}
            onClick={() => onOpenChange(false)}
            className="mx-auto shrink-0"
          >
            <BookCover book={book} key={book.id} lift={false} className="w-48 sm:w-56" />
          </Link>
        ) : (
          <div className="mx-auto aspect-square w-48 shrink-0 rounded-2xl bg-muted sm:w-56" />
        )}

        <div className="mt-5 text-center">
          <p className="font-heading text-xl font-semibold text-balance">
            {book ? (
              <Link
                to={`/books/${book.id}`}
                onClick={() => onOpenChange(false)}
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

        <div className="mt-6">
          <SeekBar size="lg" />
        </div>

        <div className="mt-4 flex items-center justify-center">
          <TransportControls size="lg" />
        </div>

        <div className="mt-6 flex items-center justify-between border-t pt-3">
          <SpeedSelect className="w-24" size="default" />
          {/* Pro prst musí být i tahle tlačítka větší než v desktopové liště. */}
          <div className="flex items-center gap-1">
            <PlayerQueue triggerClassName="size-11" />
            <SessionMenu triggerClassName="size-11" />
            <Button
              variant="ghost"
              size="icon"
              className="size-11"
              onClick={() => {
                onOpenChange(false)
                player.close()
              }}
              aria-label="Zavřít přehrávač"
              title="Zavřít přehrávač (poslech zůstane uložený)"
            >
              <XIcon />
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}
