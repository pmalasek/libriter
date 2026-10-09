import { cn } from 'core-web/src/lib/utils'

/** Značka Sceniteru: klapka v barvě schématu. */
export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      role="img"
      aria-label="Sceniter"
      className={cn(
        'bg-brand-gradient inset-shadow-glass flex size-12 shrink-0 items-center justify-center rounded-2xl text-primary-foreground shadow-glass ring-1 ring-white/20',
        className,
      )}
    >
      <svg viewBox="0 0 32 32" className="size-7" fill="none" aria-hidden>
        <rect x="5" y="11" width="22" height="15" rx="3" fill="currentColor" />
        <path d="M5.6 10.4 25.4 5.8l.9 3.7L6.5 14.1Z" fill="currentColor" />
        <path
          d="M9.6 9.5l2.6 3.4M15.2 8.2l2.6 3.4M20.8 6.9l2.6 3.4"
          stroke="var(--primary)"
          strokeWidth="1.5"
          strokeLinecap="round"
        />
        <path d="M14 15.4 20 18.5 14 21.6Z" fill="var(--primary)" />
      </svg>
    </span>
  )
}

const sizes = {
  sm: { mark: 'size-8 rounded-xl [&_svg]:size-5', text: 'text-xl' },
  md: { mark: 'size-10 [&_svg]:size-6', text: 'text-2xl' },
  lg: { mark: 'size-14 [&_svg]:size-8', text: 'text-4xl' },
}

/** Logo se slovní značkou – hlavička na telefonu a přihlašovací stránka. */
export function Logo({ size = 'md', className }: { size?: 'sm' | 'md' | 'lg'; className?: string }) {
  const s = sizes[size]
  return (
    <span className={cn('flex items-center gap-2.5', className)}>
      <LogoMark className={s.mark} />
      <span className={cn('font-heading font-bold tracking-tight text-foreground', s.text)}>Sceniter</span>
    </span>
  )
}
