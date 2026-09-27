import {
  ChevronsLeftIcon,
  ChevronsRightIcon,
  PauseIcon,
  PlayIcon,
  RotateCcwIcon,
  RotateCwIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { SKIP_BACK, SKIP_FORWARD, usePlayer } from '@/player/playerContext'

/**
 * Přetáčení a přehrávání. `sm` je pro lištu na desktopu, `lg` pro rozbalený
 * přehrávač na dotykové obrazovce – tam mají tlačítka velikost prstu.
 * Velikost ikony patří na samotnou ikonu; tlačítko ji nastavuje jen tehdy,
 * když si ji ikona neurčí sama.
 */
export function TransportControls({ size = 'sm' }: { size?: 'sm' | 'lg' }) {
  const { t } = useTranslation()
  const player = usePlayer()
  const { chapter, playing } = player
  const large = size === 'lg'

  const button = large ? 'size-12' : undefined
  const icon = large ? 'size-6' : undefined

  return (
    <div className={cn('flex shrink-0 items-center', large ? 'gap-2' : 'gap-0.5')}>
      <Button
        variant="ghost"
        size={large ? 'icon-lg' : 'icon'}
        className={button}
        onClick={player.prevChapter}
        aria-label={t('player.previousChapter')}
        title={t('player.previousChapter')}
      >
        <ChevronsLeftIcon className={icon} />
      </Button>
      <Button
        variant="ghost"
        size={large ? 'icon-lg' : 'icon'}
        className={button}
        onClick={() => player.skip(-SKIP_BACK)}
        aria-label={t('player.skipBack', { count: SKIP_BACK })}
        title={t('player.skipBackShort', { seconds: SKIP_BACK })}
      >
        <RotateCcwIcon className={icon} />
      </Button>
      <Button
        size="icon-lg"
        className={cn('rounded-full', large && 'size-16')}
        onClick={player.toggle}
        disabled={!chapter}
        aria-label={playing ? t('player.pause') : t('player.play')}
        title={playing ? t('player.pause') : t('player.play')}
      >
        {playing ? (
          <PauseIcon className={large ? 'size-8' : undefined} />
        ) : (
          <PlayIcon className={large ? 'size-8' : undefined} />
        )}
      </Button>
      <Button
        variant="ghost"
        size={large ? 'icon-lg' : 'icon'}
        className={button}
        onClick={() => player.skip(SKIP_FORWARD)}
        aria-label={t('player.skipForward', { count: SKIP_FORWARD })}
        title={t('player.skipForwardShort', { seconds: SKIP_FORWARD })}
      >
        <RotateCwIcon className={icon} />
      </Button>
      <Button
        variant="ghost"
        size={large ? 'icon-lg' : 'icon'}
        className={button}
        onClick={player.nextChapter}
        aria-label={t('player.nextChapter')}
        title={t('player.nextChapter')}
      >
        <ChevronsRightIcon className={icon} />
      </Button>
    </div>
  )
}
