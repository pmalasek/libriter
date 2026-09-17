import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { SPEEDS, usePlayer } from '@/player/playerContext'

/** Rychlost přehrávání. Šířku určuje místo, kam je zrovna zasazená. */
export function SpeedSelect({
  className,
  size = 'sm',
}: {
  className?: string
  size?: 'sm' | 'default'
}) {
  const player = usePlayer()

  return (
    <Select value={String(player.speed)} onValueChange={(value) => player.setSpeed(Number(value))}>
      <SelectTrigger
        size={size}
        className={className}
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
  )
}
