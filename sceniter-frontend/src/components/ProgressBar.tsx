import { cn } from 'core-web/src/lib/utils'

/** Tenký pruh rozkoukanosti (0–1). */
export function ProgressBar({ value, className }: { value: number; className?: string }) {
  if (value <= 0) return null
  return (
    <div className={cn('h-1 overflow-hidden rounded-full bg-foreground/15', className)}>
      <div className="h-full rounded-full bg-primary" style={{ width: `${Math.round(value * 100)}%` }} />
    </div>
  )
}
