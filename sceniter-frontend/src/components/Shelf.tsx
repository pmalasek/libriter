import { ChevronRightIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { cn } from 'core-web/src/lib/utils'

/** Vodorovná police na domovské stránce (jako v Libriteru). */
export function Shelf({
  title,
  to,
  children,
  className,
}: {
  title: string
  to?: string
  children: React.ReactNode
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <section className={cn('mt-10', className)}>
      <div className="mb-3 flex items-end justify-between gap-4">
        <h2 className="font-heading text-lg font-semibold tracking-tight">{title}</h2>
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
