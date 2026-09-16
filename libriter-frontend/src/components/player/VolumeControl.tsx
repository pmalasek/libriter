import { Volume1Icon, Volume2Icon, VolumeXIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { usePlayer } from '@/player/playerContext'

/**
 * Hlasitost přehrávače. Na úzké obrazovce se neukazuje: telefon i tablet mají
 * vlastní tlačítka a iOS hlasitost přes `<audio>` stejně nastavit nedovolí.
 */
export function VolumeControl() {
  const { volume, muted, setVolume, toggleMute } = usePlayer()
  const level = muted ? 0 : volume
  const Icon = level === 0 ? VolumeXIcon : level < 0.5 ? Volume1Icon : Volume2Icon

  return (
    <div className="hidden items-center gap-1 lg:flex">
      <Button
        variant="ghost"
        size="icon"
        onClick={toggleMute}
        aria-label={muted ? 'Zrušit ztlumení' : 'Ztlumit'}
        title={muted ? 'Zrušit ztlumení' : 'Ztlumit'}
      >
        <Icon />
      </Button>
      <Slider
        className="w-20"
        value={[Math.round(level * 100)]}
        max={100}
        step={1}
        onValueChange={([value]) => setVolume(value / 100)}
        aria-label="Hlasitost"
      />
    </div>
  )
}
