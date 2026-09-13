import { BookHeadphonesIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

/**
 * Backend zatím nemá endpoint pro obálky (COVER_ROOT se neservíruje),
 * takže zobrazujeme zástupný obrázek s ikonou.
 */
export function CoverPlaceholder({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        'flex aspect-2/3 w-full items-center justify-center rounded-lg bg-muted text-muted-foreground',
        className,
      )}
      aria-hidden
    >
      <BookHeadphonesIcon className="size-8 opacity-60" />
    </div>
  )
}
