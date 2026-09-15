import { cn } from '@/lib/utils'

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  className,
}: {
  /** Malý štítek nad titulkem – u detailů říká, co se právě prohlíží. */
  eyebrow?: string
  title: string
  description?: string
  actions?: React.ReactNode
  /** Hlavně na úpravu spodní mezery, když hlavička stojí uvnitř bloku. */
  className?: string
}) {
  return (
    <div className={cn('mb-8 flex flex-wrap items-end justify-between gap-4', className)}>
      <div className="min-w-0">
        {eyebrow ? (
          <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
            {eyebrow}
          </p>
        ) : null}
        <h1 className="font-heading truncate text-3xl font-bold tracking-tight">{title}</h1>
        {description ? <p className="mt-1.5 text-muted-foreground">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  )
}
