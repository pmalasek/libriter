import { BookHeadphonesIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

/**
 * Značka do navigace: symbol v kovu na ploše v barvě tématu.
 *
 * rounded-xl, ne 2xl: položky navigace jsou kolečka, značka má být zaoblený
 * čtverec jako ikona aplikace, ať se s nimi neplete.
 */
export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      role="img"
      aria-label="Libriter"
      className={cn(
        'bg-brand-surface inset-shadow-glass inline-flex size-12 shrink-0 items-center justify-center rounded-xl text-metal shadow-glass ring-1 ring-metal/35',
        className,
      )}
    >
      <BookHeadphonesIcon className="size-7" />
    </span>
  )
}

const SIZES = {
  sm: { mark: 'size-8 rounded-lg [&_svg]:size-5', text: 'text-xl' },
  md: { mark: 'size-10 [&_svg]:size-6', text: 'text-2xl' },
  lg: { mark: 'size-14 [&_svg]:size-8', text: 'text-4xl' },
} as const

/**
 * Logo se slovní značkou – hlavička na telefonu, panel a přihlášení.
 *
 * Nápis je sázený písmem, ne obrázek: obarví se podle tématu i režimu sám.
 * Původní PNG loga (_image/libriter-logo-*.png) patří ke starým schématům.
 */
export function Logo({ className, size = 'md' }: { className?: string; size?: keyof typeof SIZES }) {
  const s = SIZES[size]
  return (
    <span className={cn('inline-flex shrink-0 items-center gap-2.5', className)}>
      <LogoMark className={s.mark} />
      <span className={cn('font-heading font-bold tracking-tight text-foreground', s.text)}>Libriter</span>
    </span>
  )
}
