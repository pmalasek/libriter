import { BookHeadphonesIcon } from 'lucide-react'
import logoLight from '../../../../_image/libriter-logo-light.png'
import logoDark from '../../../../_image/libriter-logo-dark.png'
import logoBlueLight from '../../../../_image/libriter-logo-blue-light.png'
import logoBlueDark from '../../../../_image/libriter-logo-blue-dark.png'
import logoVioletLight from '../../../../_image/libriter-logo-violet-light.png'
import logoVioletDark from '../../../../_image/libriter-logo-violet-dark.png'
import logoGreenLight from '../../../../_image/libriter-logo-green-light.png'
import logoGreenDark from '../../../../_image/libriter-logo-green-dark.png'
import { cn } from '@/lib/utils'
import { useColorScheme } from '@/theme/colorScheme'

const LOGOS = {
  teal: { light: logoLight, dark: logoDark },
  blue: { light: logoBlueLight, dark: logoBlueDark },
  violet: { light: logoVioletLight, dark: logoVioletDark },
  green: { light: logoGreenLight, dark: logoGreenDark },
} as const

const SIZES = {
  sm: 'h-8 w-32',
  md: 'h-11 w-44',
  lg: 'h-16 w-64',
} as const

export function Logo({
  className,
  size = 'md',
}: {
  className?: string
  size?: keyof typeof SIZES
}) {
  const { colorScheme } = useColorScheme()
  const logo = LOGOS[colorScheme]

  return (
    <span role="img" aria-label="Libriter" className={cn('inline-flex shrink-0', SIZES[size], className)}>
      {/* ViewBox skryje průhledné okraje originálů, bez úprav zdrojových PNG. */}
      <svg viewBox="95 260 1500 410" aria-hidden="true" className="size-full dark:hidden">
        <image href={logo.light} width="1672" height="941" />
      </svg>
      <svg viewBox="95 260 1500 410" aria-hidden="true" className="hidden size-full dark:block">
        <image href={logo.dark} width="1672" height="941" />
      </svg>
    </span>
  )
}

/**
 * Značka do ikonového railu, kde na nápis není místo.
 *
 * Záměrně nekreslí čtvercovou ikonu ze zdrojového PNG: ta má vlastní
 * neprůhledné pozadí a na skle vypadá jako nalepená. Kreslí symbol na firemním
 * gradientu se stejným zaoblením, jaké má zbytek rozhraní – je to jediná sytá
 * plocha ve sloupku, takže čte jako značka, ne jako další položka navigace.
 */
export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      role="img"
      aria-label="Libriter"
      className={cn(
        // rounded-xl, ne 2xl: položky navigace jsou kolečka, značka má být
        // zaoblený čtverec jako ikona aplikace, ať se s nimi neplete.
        'bg-brand-gradient inline-flex size-12 shrink-0 items-center justify-center rounded-xl text-primary-foreground shadow-glow inset-shadow-[0_1px_0_0_oklch(1_0_0/0.3)]',
        className,
      )}
    >
      <BookHeadphonesIcon className="size-7" />
    </span>
  )
}
