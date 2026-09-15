import { ArrowDownNarrowWideIcon, ArrowUpNarrowWideIcon, Grid3x3Icon, LayoutGridIcon, ListIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { VIEW_MODE_LABELS, VIEW_MODES, type SortDir, type ViewMode } from '@/lib/sorting'
import { cn } from '@/lib/utils'

const VIEW_ICONS: Record<ViewMode, React.ComponentType<{ className?: string }>> = {
  tiles: LayoutGridIcon,
  small: Grid3x3Icon,
  list: ListIcon,
}

/** Přepínač dlaždice / malé dlaždice / seznam. */
export function ViewModeToggle({
  value,
  onChange,
}: {
  value: ViewMode
  onChange: (view: ViewMode) => void
}) {
  return (
    <div role="group" aria-label="Zobrazení" className="inline-flex rounded-full bg-muted p-1">
      {VIEW_MODES.map((mode) => {
        const Icon = VIEW_ICONS[mode]
        const active = mode === value
        return (
          <Button
            key={mode}
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-pressed={active}
            aria-label={VIEW_MODE_LABELS[mode]}
            title={VIEW_MODE_LABELS[mode]}
            onClick={() => onChange(mode)}
            className={cn(
              'rounded-full hover:bg-transparent',
              active && 'bg-card text-primary shadow-sm hover:bg-card',
            )}
          >
            <Icon />
          </Button>
        )
      })}
    </div>
  )
}

/** Výběr klíče řazení a směru. */
export function SortControl<K extends string>({
  options,
  value,
  onChange,
  dir,
  onDirChange,
  className,
}: {
  options: { value: K; label: string }[]
  value: K
  onChange: (key: K) => void
  dir: SortDir
  onDirChange: (dir: SortDir) => void
  className?: string
}) {
  const DirIcon = dir === 'asc' ? ArrowDownNarrowWideIcon : ArrowUpNarrowWideIcon
  const dirLabel = dir === 'asc' ? 'Vzestupně – přepnout na sestupně' : 'Sestupně – přepnout na vzestupně'

  return (
    <div className={cn('flex items-center gap-1', className)}>
      <Select value={value} onValueChange={(next) => onChange(next as K)}>
        <SelectTrigger aria-label="Řadit podle" className="w-40">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button
        type="button"
        variant="outline"
        size="icon"
        aria-label={dirLabel}
        title={dirLabel}
        onClick={() => onDirChange(dir === 'asc' ? 'desc' : 'asc')}
      >
        <DirIcon />
      </Button>
    </div>
  )
}
