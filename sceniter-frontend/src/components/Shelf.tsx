import { ChevronRightIcon, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { cn } from 'core-web/src/lib/utils'

/**
 * Vodorovná police na domovské stránce (jako v Libriteru). Police knihoven
 * dostávají ikonu z navigace a nadpisek nad názvem, aby se od sebe na první
 * pohled lišily; ostatní police mají jen nadpis.
 */
export function Shelf({
  title,
  eyebrow,
  icon: Icon,
  to,
  divider = true,
  children,
  className,
}: {
  title: string
  /** Malý text nad nadpisem („Nově v knihovně“) – nadpis je pak větší. */
  eyebrow?: string
  /** Ikona před nadpisem, stejná jako u položky v navigaci. */
  icon?: LucideIcon
  to?: string
  /** Vlasová linka nad policí; první police na stránce ji nepotřebuje. */
  divider?: boolean
  children: React.ReactNode
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <section className={cn('mt-8', divider && 'hairline-t pt-8', className)}>
      <div className="mb-3 flex items-end justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          {Icon ? (
            <span
              aria-hidden
              className="inset-shadow-glass flex size-10 shrink-0 items-center justify-center rounded-2xl bg-primary/12 text-primary ring-1 ring-primary/20"
            >
              <Icon className="size-5" />
            </span>
          ) : null}
          <div className="min-w-0">
            {eyebrow ? (
              <p className="text-xs font-semibold tracking-wider text-primary uppercase">{eyebrow}</p>
            ) : null}
            <h2
              className={cn(
                'font-heading truncate font-semibold tracking-tight',
                eyebrow ? 'text-xl md:text-2xl' : 'text-lg',
              )}
            >
              {title}
            </h2>
          </div>
        </div>
        {to ? (
          <Link to={to} className="flex shrink-0 items-center gap-0.5 text-sm text-primary underline-offset-4 hover:underline">
            {t('video.list.showAll')}
            <ChevronRightIcon className="size-4" />
          </Link>
        ) : null}
      </div>
      <div className="-mx-4 overflow-x-auto px-4 pb-2 [scrollbar-width:none] sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
        <div className="flex snap-x snap-mandatory gap-4">{children}</div>
      </div>
    </section>
  )
}

export function ShelfItem({ children, wide = false }: { children: React.ReactNode; wide?: boolean }) {
  return <div className={cn('shrink-0 snap-start', wide ? 'w-64 sm:w-72' : 'w-32 sm:w-40')}>{children}</div>
}
