import {
  ChevronsLeftIcon,
  ChevronsRightIcon,
  PauseIcon,
  PlayIcon,
  RotateCcwIcon,
  RotateCwIcon,
  XIcon,
} from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { BookCover } from '@/components/BookCover'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Slider } from '@/components/ui/slider'
import { formatClock } from '@/lib/format'
import { SKIP_BACK, SKIP_FORWARD, SPEEDS, usePlayer } from '@/player/playerContext'
import { SessionMenu } from './SessionMenu'

/**
 * Lišta přehrávače. Drží se u spodní hrany na všech stránkách, aby poslech
 * nepřerušila navigace v knihovně; vedle sidebaru začíná až za ním.
 */
export function PlayerBar() {
  const player = usePlayer()
  // Pozice ukazovaná během tažení posuvníku, než ji uživatel pustí.
  const [scrub, setScrub] = useState<number | null>(null)
  const { session, book, chapter, chapters, currentTime, duration, playing, loading, speed } = player

  if (!session) return null

  const chapterIndex = chapter ? chapters.findIndex((c) => c.id === chapter.id) : -1
  const itemIndex = session.items.findIndex((item) => item.book_id === book?.id)
  const length = duration || chapter?.duration_seconds || 0

  return (
    <div
      className="fixed inset-x-0 bottom-0 z-40 border-t bg-background/95 backdrop-blur md:left-64"
      role="region"
      aria-label="Přehrávač"
    >
      <div className="mx-auto flex max-w-7xl flex-col gap-1.5 px-4 py-2 sm:px-6 lg:px-8">
        <div className="flex items-center gap-3">
          {/* Co hraje */}
          <div className="flex min-w-0 flex-1 items-center gap-3">
            {book ? (
              <Link to={`/books/${book.id}`} className="shrink-0" aria-label={`Detail knihy ${book.title}`}>
                <BookCover book={book} key={book.id} lift={false} className="size-10 rounded-lg" />
              </Link>
            ) : (
              <div className="size-10 shrink-0 rounded-lg bg-muted" />
            )}
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">
                {book ? (
                  <Link to={`/books/${book.id}`} className="hover:underline">
                    {book.title}
                  </Link>
                ) : (
                  'Načítání…'
                )}
              </p>
              <p className="truncate text-xs text-muted-foreground">
                {chapter ? chapter.title : loading ? 'Načítání kapitoly…' : '—'}
                {chapterIndex >= 0 && chapters.length > 1
                  ? ` · ${chapterIndex + 1}/${chapters.length}`
                  : ''}
                {session.items.length > 1 && itemIndex >= 0
                  ? ` · kniha ${itemIndex + 1}/${session.items.length}`
                  : ''}
              </p>
            </div>
          </div>

          {/* Ovládání */}
          <div className="flex shrink-0 items-center gap-0.5">
            <Button
              variant="ghost"
              size="icon"
              className="hidden sm:inline-flex"
              onClick={player.prevChapter}
              aria-label="Předchozí kapitola"
              title="Předchozí kapitola"
            >
              <ChevronsLeftIcon />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => player.skip(-SKIP_BACK)}
              aria-label={`Zpět o ${SKIP_BACK} sekund`}
              title={`Zpět o ${SKIP_BACK} s`}
            >
              <RotateCcwIcon />
            </Button>
            <Button
              size="icon-lg"
              className="rounded-full"
              onClick={player.toggle}
              disabled={!chapter}
              aria-label={playing ? 'Pozastavit' : 'Přehrát'}
              title={playing ? 'Pozastavit' : 'Přehrát'}
            >
              {playing ? <PauseIcon /> : <PlayIcon />}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => player.skip(SKIP_FORWARD)}
              aria-label={`Vpřed o ${SKIP_FORWARD} sekund`}
              title={`Vpřed o ${SKIP_FORWARD} s`}
            >
              <RotateCwIcon />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="hidden sm:inline-flex"
              onClick={player.nextChapter}
              aria-label="Další kapitola"
              title="Další kapitola"
            >
              <ChevronsRightIcon />
            </Button>
          </div>

          {/* Rychlost, přepínání poslechů a zavření lišty */}
          <div className="flex flex-1 items-center justify-end gap-0.5">
            <Select value={String(speed)} onValueChange={(value) => player.setSpeed(Number(value))}>
              <SelectTrigger
                size="sm"
                className="hidden w-[4.5rem] sm:flex"
                aria-label="Rychlost přehrávání"
                title="Rychlost přehrávání"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SPEEDS.map((value) => (
                  <SelectItem key={value} value={String(value)}>
                    {value.toLocaleString('cs-CZ')}×
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <SessionMenu />
            <Button
              variant="ghost"
              size="icon"
              onClick={player.close}
              aria-label="Zavřít přehrávač"
              title="Zavřít přehrávač (poslech zůstane uložený)"
            >
              <XIcon />
            </Button>
          </div>
        </div>

        {/* Posun v kapitole */}
        <div className="flex items-center gap-3">
          <span className="w-12 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
            {formatClock(scrub ?? currentTime)}
          </span>
          {/* Během tažení se mění jen zobrazení; přetočí se až po puštění,
              aby se na server neposílala pozice z každého mezikroku. */}
          <Slider
            value={[Math.min(scrub ?? currentTime, length)]}
            max={Math.max(length, 1)}
            step={1}
            disabled={!chapter}
            onValueChange={([value]) => setScrub(value)}
            onValueCommit={([value]) => {
              player.seek(value)
              setScrub(null)
            }}
            aria-label="Pozice v kapitole"
          />
          <span className="w-12 shrink-0 text-xs text-muted-foreground tabular-nums">
            {formatClock(length)}
          </span>
        </div>
      </div>
    </div>
  )
}
