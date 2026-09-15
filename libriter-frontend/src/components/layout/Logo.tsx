import { cn } from '@/lib/utils'

/**
 * Značka Libriteru jako inline SVG – přehrávací disk se zvukovými vlnami.
 * Barvy jdou z tokenů, takže značka funguje ve světlém i tmavém režimu
 * bez druhého obrázku. Na firemním gradientu (přihlášení) se použije
 * tone="invert".
 */
const TONES = {
  brand: '[--logo-square:var(--primary)] [--logo-disc:var(--primary-foreground)] [--logo-glyph:var(--primary)]',
  invert: '[--logo-square:oklch(1_0_0_/_0.18)] [--logo-disc:white] [--logo-glyph:oklch(0.52_0.1_195)]',
} as const

const SIZES = {
  sm: { mark: 'size-7', text: 'text-base' },
  md: { mark: 'size-8', text: 'text-lg' },
  lg: { mark: 'size-11', text: 'text-2xl' },
} as const

export function LogoMark({
  className,
  tone = 'brand',
}: {
  className?: string
  tone?: keyof typeof TONES
}) {
  return (
    <svg
      viewBox="0 0 32 32"
      aria-hidden
      className={cn('shrink-0', TONES[tone], className)}
      fill="none"
    >
      <rect x="1" y="1" width="30" height="30" rx="9" className="fill-(--logo-square)" />
      {/* Oblouk kolem disku – zůstal z původního loga, dává značce pohyb. */}
      <path
        d="M6.2 11.6A11 11 0 0 1 17 5.2"
        className="stroke-(--logo-disc)"
        strokeWidth="1.6"
        strokeLinecap="round"
        opacity="0.55"
      />
      <circle cx="16" cy="16.5" r="9.5" className="fill-(--logo-disc)" />
      <path d="M14 12.6 21 16.5 14 20.4Z" className="fill-(--logo-glyph)" />
      <g
        className="stroke-(--logo-glyph)"
        strokeWidth="1.4"
        strokeLinecap="round"
        fill="none"
      >
        <path d="M22.6 13.4a5 5 0 0 1 0 6.2" />
        <path d="M11.8 14.2v4.6" />
        <path d="M9.4 15.4v2.2" />
        <path d="M8.2 21.8c2.1-1.5 4.2-1.5 6.3 0s4.2 1.5 6.3 0" strokeWidth="1.5" />
      </g>
    </svg>
  )
}

export function Logo({
  className,
  tone = 'brand',
  size = 'md',
  wordmark = true,
}: {
  className?: string
  tone?: keyof typeof TONES
  size?: keyof typeof SIZES
  wordmark?: boolean
}) {
  const scale = SIZES[size]

  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <LogoMark tone={tone} className={scale.mark} />
      {wordmark ? (
        <span className={cn('font-heading font-bold tracking-tight', scale.text)}>Libriter</span>
      ) : (
        <span className="sr-only">Libriter</span>
      )}
    </span>
  )
}
