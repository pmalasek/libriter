import { Skeleton } from './ui/skeleton'

/** Zástupné řádky seznamu, dokud se nenačtou data. */
export function LoadingList({ count = 6 }: { count?: number }) {
  return (
    <div className="space-y-3">
      {Array.from({ length: count }, (_, i) => (
        <Skeleton key={i} className="h-16 w-full rounded-2xl" />
      ))}
    </div>
  )
}
