import {
  ChevronUpIcon,
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
import { formatClock } from '@/lib/format'
import { SKIP_BACK, SKIP_FORWARD, usePlayer } from '@/player/playerContext'
import { PlayerQueue } from './PlayerQueue'
import { PlayerSheet } from './PlayerSheet'
import { SeekBar } from './SeekBar'
import { SessionMenu } from './SessionMenu'
import { SpeedSelect } from './SpeedSelect'
import { TransportControls } from './TransportControls'
import { VolumeControl } from './VolumeControl'

/**
 * Lišta přehrávače. Drží se u spodní hrany na všech stránkách, aby poslech
 * nepřerušila navigace v knihovně; vedle sidebaru začíná až za ním.
 *
 * Do lg je lišta jen kompaktní ovladač – jezdec a zbytek ovládání se vejdou
 * až do panelu, který se vytáhne klepnutím (PlayerSheet). Od lg se vejde
 * všechno do dvou řádků vedle sebe.
 */
export function PlayerBar() {
  const player = usePlayer()
  const [expanded, setExpanded] = useState(false)
  const { session, book, chapter, chapters, currentTime, duration, playing, loading } = player

  if (!session) return null

  const chapterIndex = chapter ? chapters.findIndex((c) => c.id === chapter.id) : -1
  const itemIndex = session.items.findIndex((item) => item.book_id === book?.id)
  const length = duration || chapter?.duration_seconds || 0

  const subtitle =
    (chapter ? chapter.title : loading ? 'Načítání kapitoly…' : '—') +
    (chapterIndex >= 0 && chapters.length > 1 ? ` · ${chapterIndex + 1}/${chapters.length}` : '') +
    (session.items.length > 1 && itemIndex >= 0
      ? ` · kniha ${itemIndex + 1}/${session.items.length}`
      : '')

  return (
    <div
      className="fixed inset-x-0 bottom-0 z-40 border-t bg-background/95 backdrop-blur md:left-64"
      role="region"
      aria-label="Přehrávač"
    >
      {/* Mobil a tablet: jeden řádek s velkými tlačítky, zbytek v panelu. */}
      <div className="lg:hidden">
        {/* Kde poslech je; v liště se netáhne, na to je jezdec v panelu. */}
        <div className="h-0.5 w-full bg-muted" aria-hidden>
          <div
            className="h-full bg-primary transition-[width] duration-300"
            style={{ width: `${length > 0 ? Math.min(currentTime / length, 1) * 100 : 0}%` }}
          />
        </div>

        {/* Šipka přes celou šířku říká, že se lišta dá rozbalit; po stranách
            je pozice v kapitole a její délka. Vodorovné místo vedle názvu
            si tenhle řádek nebere, to patří tlačítkům. */}
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className="flex w-full items-center px-3 py-0.5 text-muted-foreground hover:text-foreground sm:px-6"
          aria-label="Rozbalit přehrávač"
          aria-expanded={expanded}
        >
          <span className="w-12 text-left text-[0.7rem] tabular-nums">
            {formatClock(currentTime)}
          </span>
          <ChevronUpIcon className="mx-auto size-4" />
          <span className="w-12 text-right text-[0.7rem] tabular-nums">{formatClock(length)}</span>
        </button>

        <div className="flex items-center gap-1 px-3 pb-[max(0.5rem,env(safe-area-inset-bottom))] sm:px-6">
          <button
            type="button"
            onClick={() => setExpanded(true)}
            className="flex min-w-0 flex-1 items-center gap-3 rounded-xl py-1 text-left"
            aria-label="Rozbalit přehrávač"
            aria-expanded={expanded}
          >
            {book ? (
              <BookCover
                book={book}
                key={book.id}
                lift={false}
                className="size-12 shrink-0 rounded-lg"
              />
            ) : (
              <div className="size-12 shrink-0 rounded-lg bg-muted" />
            )}
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">
                {book ? book.title : 'Načítání…'}
              </span>
              <span className="block truncate text-xs text-muted-foreground">{subtitle}</span>
            </span>
          </button>

          <div className="flex shrink-0 items-center gap-0.5">
            <Button
              variant="ghost"
              size="icon-lg"
              className="size-11"
              onClick={() => player.skip(-SKIP_BACK)}
              aria-label={`Zpět o ${SKIP_BACK} sekund`}
              title={`Zpět o ${SKIP_BACK} s`}
            >
              <RotateCcwIcon className="size-6" />
            </Button>
            <Button
              size="icon-lg"
              className="size-14 rounded-full"
              onClick={player.toggle}
              disabled={!chapter}
              aria-label={playing ? 'Pozastavit' : 'Přehrát'}
              title={playing ? 'Pozastavit' : 'Přehrát'}
            >
              {playing ? <PauseIcon className="size-7" /> : <PlayIcon className="size-7" />}
            </Button>
            <Button
              variant="ghost"
              size="icon-lg"
              className="size-11"
              onClick={() => player.skip(SKIP_FORWARD)}
              aria-label={`Vpřed o ${SKIP_FORWARD} sekund`}
              title={`Vpřed o ${SKIP_FORWARD} s`}
            >
              <RotateCwIcon className="size-6" />
            </Button>
          </div>
        </div>
      </div>

      {/* Desktop: informace, ovládání a jezdec ve dvou řádcích. */}
      <div className="mx-auto hidden max-w-7xl flex-col gap-1.5 px-4 py-2 sm:px-6 lg:flex lg:px-8">
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
              <p className="truncate text-xs text-muted-foreground">{subtitle}</p>
            </div>
          </div>

          {/* Ovládání */}
          <TransportControls />

          {/* Hlasitost, rychlost, obsah poslechu a zavření lišty */}
          <div className="flex flex-1 items-center justify-end gap-0.5">
            <VolumeControl />
            <SpeedSelect className="w-[4.5rem]" />
            <PlayerQueue />
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
        <SeekBar />
      </div>

      <PlayerSheet open={expanded} onOpenChange={setExpanded} subtitle={subtitle} />
    </div>
  )
}
