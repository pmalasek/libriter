import { PauseIcon, PlayIcon, RotateCcwIcon, RotateCwIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { BookCover } from '@/components/BookCover'
import { Button } from '@/components/ui/button'
import { SKIP_BACK, SKIP_FORWARD, playerSubtitle, usePlayer } from '@/player/playerContext'
import { PlayerSheet } from './PlayerSheet'

/**
 * Plovoucí kapsle přehrávače. Drží se u spodní hrany na všech stránkách, aby
 * poslech nepřerušila navigace v knihovně, a vejde se do ní jen to nejnutnější:
 * co hraje, přetáčení a přehrávání. Zbytek je v panelu, který se vytáhne
 * klepnutím.
 *
 * Od xl kapsle mizí – tam je všechno ve sloupci u pravé hrany.
 */
export function PlayerCapsule() {
  const { t } = useTranslation()
  const player = usePlayer()
  const [expanded, setExpanded] = useState(false)
  const { session, book, chapter, currentTime, duration, playing } = player

  if (!session) return null

  const length = duration || chapter?.duration_seconds || 0
  const progress = length > 0 ? Math.min(currentTime / length, 1) * 100 : 0

  return (
    <>
      <div
        role="region"
        aria-label={t('player.region')}
        className="glass-strong inset-shadow-glass fixed bottom-[5.5rem] left-1/2 z-40 h-16 w-[min(36rem,calc(100%-1.5rem))] -translate-x-1/2 rounded-full px-2 shadow-glass-lg ring-1 ring-glass-edge md:bottom-3 md:left-[calc(50%+2.25rem)] md:w-[min(36rem,calc(100%-7rem))] xl:hidden"
      >
        <div className="flex h-full items-center gap-1">
          <button
            type="button"
            onClick={() => setExpanded(true)}
            className="flex h-12 min-w-0 flex-1 items-center gap-3 rounded-full pr-2 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            aria-label={t('player.expand')}
            aria-expanded={expanded}
          >
            {book ? (
              <BookCover
                book={book}
                key={book.id}
                lift={false}
                className="size-12 shrink-0 rounded-full"
              />
            ) : (
              <div className="size-12 shrink-0 rounded-full bg-foreground/8" />
            )}
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">
                {book ? book.title : t('player.loading')}
              </span>
              <span className="block truncate text-xs text-muted-foreground">
                {playerSubtitle(player)}
              </span>
            </span>
          </button>

          <div className="flex shrink-0 items-center gap-0.5">
            <Button
              variant="ghost"
              size="icon-lg"
              className="size-10 rounded-full max-[24rem]:hidden"
              onClick={() => player.skip(-SKIP_BACK)}
              aria-label={t('player.skipBack', { count: SKIP_BACK })}
              title={t('player.skipBackShort', { seconds: SKIP_BACK })}
            >
              <RotateCcwIcon className="size-5" />
            </Button>
            <Button
              size="icon-lg"
              className="size-12 rounded-full shadow-glow transition-transform active:scale-95 motion-reduce:active:scale-100"
              onClick={player.toggle}
              disabled={!chapter}
              aria-label={playing ? t('player.pause') : t('player.play')}
              title={playing ? t('player.pause') : t('player.play')}
            >
              {playing ? <PauseIcon className="size-6" /> : <PlayIcon className="size-6" />}
            </Button>
            <Button
              variant="ghost"
              size="icon-lg"
              className="size-10 rounded-full max-[24rem]:hidden"
              onClick={() => player.skip(SKIP_FORWARD)}
              aria-label={t('player.skipForward', { count: SKIP_FORWARD })}
              title={t('player.skipForwardShort', { seconds: SKIP_FORWARD })}
            >
              <RotateCwIcon className="size-5" />
            </Button>
          </div>
        </div>

        {/* Kde poslech je; táhnout se dá až jezdcem v panelu. */}
        <div
          className="absolute inset-x-8 bottom-1.5 h-0.5 overflow-hidden rounded-full bg-foreground/10"
          aria-hidden
        >
          <div
            className="bg-brand-gradient h-full transition-[width] duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>
      </div>

      <PlayerSheet open={expanded} onOpenChange={setExpanded} />
    </>
  )
}
