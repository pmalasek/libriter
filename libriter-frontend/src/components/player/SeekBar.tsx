import { useState } from 'react'
import { Slider } from '@/components/ui/slider'
import { formatClock } from '@/lib/format'
import { usePlayer } from '@/player/playerContext'

/**
 * Posun v kapitole i s časy. V liště na desktopu stojí časy po stranách
 * jezdce, v rozbaleném přehrávači pod ním, aby byl jezdec přes celou šířku.
 */
export function SeekBar({ size = 'sm' }: { size?: 'sm' | 'lg' }) {
  const player = usePlayer()
  // Pozice ukazovaná během tažení posuvníku, než ji uživatel pustí.
  const [scrub, setScrub] = useState<number | null>(null)
  const { chapter, currentTime, duration } = player
  const length = duration || chapter?.duration_seconds || 0
  const position = Math.min(scrub ?? currentTime, length)

  // Během tažení se mění jen zobrazení; přetočí se až po puštění,
  // aby se na server neposílala pozice z každého mezikroku.
  const slider = (
    <Slider
      size={size === 'lg' ? 'lg' : 'default'}
      value={[position]}
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
  )

  if (size === 'lg') {
    return (
      <div>
        {slider}
        <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground tabular-nums">
          <span>{formatClock(position)}</span>
          <span>{formatClock(length)}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="flex items-center gap-3">
      <span className="w-12 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
        {formatClock(position)}
      </span>
      {slider}
      <span className="w-12 shrink-0 text-xs text-muted-foreground tabular-nums">
        {formatClock(length)}
      </span>
    </div>
  )
}
