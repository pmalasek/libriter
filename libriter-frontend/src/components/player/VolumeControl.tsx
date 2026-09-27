import { Volume1Icon, Volume2Icon, VolumeXIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { usePlayer } from '@/player/playerContext'

/**
 * Hlasitost přehrávače. Na úzké obrazovce se neukazuje: telefon i tablet mají
 * vlastní tlačítka a iOS hlasitost přes `<audio>` stejně nastavit nedovolí.
 */
export function VolumeControl() {
  const { t } = useTranslation()
  const { volume, muted, setVolume, toggleMute } = usePlayer()
  const level = muted ? 0 : volume
  const Icon = level === 0 ? VolumeXIcon : level < 0.5 ? Volume1Icon : Volume2Icon

  return (
    <div className="hidden items-center gap-1 lg:flex">
      <Button
        variant="ghost"
        size="icon"
        onClick={toggleMute}
        aria-label={muted ? t('player.unmute') : t('player.mute')}
        title={muted ? t('player.unmute') : t('player.mute')}
      >
        <Icon />
      </Button>
      <Slider
        className="w-20"
        value={[Math.round(level * 100)]}
        max={100}
        step={1}
        onValueChange={([value]) => setVolume(value / 100)}
        aria-label={t('player.volume')}
      />
    </div>
  )
}
