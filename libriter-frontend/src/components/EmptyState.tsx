import type { LucideIcon } from 'lucide-react'

export function EmptyState({
  title,
  description,
  icon: Icon,
}: {
  title: string
  description?: string
  icon?: LucideIcon
}) {
  return (
    <div className="rounded-3xl border border-dashed border-glass-edge bg-glass/60 p-12 text-center">
      {Icon ? (
        <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-2xl bg-primary/10 text-primary">
          <Icon className="size-6" />
        </span>
      ) : null}
      <p className="font-heading text-lg font-semibold">{title}</p>
      {description ? (
        <p className="mx-auto mt-1.5 max-w-md text-sm text-muted-foreground">{description}</p>
      ) : null}
    </div>
  )
}
