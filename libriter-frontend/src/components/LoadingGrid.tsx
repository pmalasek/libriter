import { LoadingList } from 'core-web/src/components/LoadingList'
import { Skeleton } from '@/components/ui/skeleton'
import type { ViewMode } from '@/lib/sorting'

// LoadingList je sdílený s ostatními produkty (core-web).
export { LoadingList }

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
          <Skeleton className="aspect-square w-full rounded-2xl" />
          <Skeleton className="h-4 w-4/5" />
          <Skeleton className="h-3 w-2/3" />
        </div>
      ))}
    </div>
  )
}
