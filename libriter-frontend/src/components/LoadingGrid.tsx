import { Skeleton } from '@/components/ui/skeleton'
import type { ViewMode } from '@/lib/sorting'

const GRID: Record<Exclude<ViewMode, 'list'>, string> = {
  tiles: 'grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5',
  small: 'grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7 xl:grid-cols-8',
}

export function LoadingGrid({ count = 8, view = 'tiles' }: { count?: number; view?: ViewMode }) {
  if (view === 'list') return <LoadingList count={count} />

  return (
    <div className={GRID[view]}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="space-y-2">
          <Skeleton className="aspect-square w-full rounded-lg" />
          <Skeleton className="h-4 w-4/5" />
          <Skeleton className="h-3 w-2/3" />
        </div>
      ))}
    </div>
  )
}

export function LoadingList({ count = 6 }: { count?: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }, (_, i) => (
        <Skeleton key={i} className="h-16 w-full rounded-xl" />
      ))}
    </div>
  )
}
