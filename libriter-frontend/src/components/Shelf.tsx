import { ChevronRightIcon } from 'lucide-react'
import { Link } from 'react-router'
import { cn } from '@/lib/utils'

/**
 * Vodorovná police na domovské stránce. Položky se posouvají do strany
 * a zaskakují na začátek, aby police nikdy nekončila rozpůlenou obálkou.
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
  return (
    <section className={cn('mt-10', className)}>
      <div className="mb-3 flex items-end justify-between gap-4">
        <h2 className="font-heading text-lg font-semibold tracking-tight">{title}</h2>
        {to ? (
          <Link
            to={to}
            className="flex shrink-0 items-center gap-0.5 text-sm text-primary underline-offset-4 hover:underline"
          >
            Zobrazit vše
            <ChevronRightIcon className="size-4" />
          </Link>
        ) : null}
      </div>

      {/* Záporný okraj nechá police začínat i končit u hrany obsahu, ale
          posunuté položky se nelepí na kraj obrazovky. */}
      <div className="-mx-4 overflow-x-auto px-4 pb-2 [scrollbar-width:none] sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
        <div className="flex snap-x snap-mandatory gap-4">{children}</div>
      </div>
    </section>
  )
}

/** Jedna položka police – pevná šířka, ať mřížka drží rytmus. */
export function ShelfItem({ children }: { children: React.ReactNode }) {
  return <div className="w-32 shrink-0 snap-start sm:w-36">{children}</div>
}
