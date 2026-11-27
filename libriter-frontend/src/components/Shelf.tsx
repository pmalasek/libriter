import { ChevronRightIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { ShelfScroller } from 'core-web/src/components/ShelfScroller'
import { cn } from '@/lib/utils'

/**
 * Vodorovná police na domovské stránce. Položky se posouvají do strany
 * a zaskakují na začátek, aby police nikdy nekončila rozpůlenou obálkou;
 * myší se posouvá šipkami (viz ShelfScroller).
 */
export function Shelf({
  title,
  to,
  children,
  className,
}: {
  title: string
  /** Kam vede „Zobrazit vše“; bez něj se odkaz nevykreslí. */
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
          <Link
            to={to}
            className="flex shrink-0 items-center gap-0.5 text-sm text-primary underline-offset-4 hover:underline"
          >
            {t('books.shelf.showAll')}
            <ChevronRightIcon className="size-4" />
          </Link>
        ) : null}
      </div>

      <ShelfScroller>{children}</ShelfScroller>
    </section>
  )
}

/** Jedna položka police – pevná šířka, ať mřížka drží rytmus. */
export function ShelfItem({ children }: { children: React.ReactNode }) {
  return <div className="w-32 shrink-0 snap-start sm:w-36">{children}</div>
}
