import { cn } from '@/lib/utils'

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
  sticky = false,
  children,
  className,
}: {
  /** Malý štítek nad titulkem – u detailů říká, co se právě prohlíží. */
  eyebrow?: string
  title: string
  description?: string
  actions?: React.ReactNode
  /**
   * Hlavička zůstane u horní hrany a roluje se jen obsah pod ní. Pro výpisy,
   * kde je v hlavičce hledání a řazení – ty musí být po ruce i po odrolování.
   */
  sticky?: boolean
  /** Další řádek uvnitř přilepené hlavičky (například lišta hromadného výběru). */
  children?: React.ReactNode
  /** Hlavně na úpravu spodní mezery, když hlavička stojí uvnitř bloku. */
  className?: string
}) {
  return (
    <div
      className={cn(
        sticky &&
          // Plovoucí panel, ne pruh přilepený k hraně okna – stejně jako rail
          // nebo kapsle přehrávače.
          //
          // Přilepí se až od md. Na telefonu má hlavička pod sebou ještě
          // hledání, řazení a přepínač zobrazení, takže by ukrojila skoro půl
          // obrazovky; tam se radši odroluje pryč.
          'glass-strong inset-shadow-glass z-30 mb-6 rounded-3xl px-4 py-3 shadow-glass ring-1 ring-glass-edge md:sticky md:top-3',
        !sticky && 'mb-8',
        className,
      )}
    >
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="min-w-0">
          {eyebrow ? (
            <p className="mb-1.5 text-xs font-semibold tracking-wider text-primary uppercase">
              {eyebrow}
            </p>
          ) : null}
          {/* Titulek se zalomí, neořízne: u názvu série nebo jména autora je
              useknutý text horší než o řádek vyšší hlavička. */}
          <h1 className="font-heading text-3xl font-semibold tracking-tight text-balance break-words">
            {title}
          </h1>
          {description ? <p className="mt-1.5 text-muted-foreground">{description}</p> : null}
        </div>
        {/* Na mobilu akce zabírají celou šířku (hledání se roztáhne pod titulek),
            od sm se zase vejdou vedle titulku. */}
        {actions ? (
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">{actions}</div>
        ) : null}
      </div>
      {children}
    </div>
  )
}
