import { useTranslation } from 'react-i18next'
import { currentLanguage } from '@/api/types'
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
  const { t } = useTranslation()
  const player = usePlayer()

  return (
    <Select value={String(player.speed)} onValueChange={(value) => player.setSpeed(Number(value))}>
      <SelectTrigger
        size={size}
        className={className}
        aria-label={t('player.speed')}
        title={t('player.speed')}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {SPEEDS.map((value) => (
          <SelectItem key={value} value={String(value)}>
            {value.toLocaleString(currentLanguage())}×
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
