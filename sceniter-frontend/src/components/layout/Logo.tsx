import { useId } from 'react'
import { cn } from 'core-web/src/lib/utils'

/**
 * Značka Sceniteru: zaoblený trojúhelník „přehrát“ v kovovém přechodu
 * tématu (měď, zlato). Barvy bere z --primary, takže je čitelná i ve
 * světlém režimu, kde je kov ztmavený.
 */
export function LogoMark({ className }: { className?: string }) {
  const id = useId()
  return (
    <svg role="img" aria-label="Sceniter" viewBox="0 0 32 32" fill="none" className={cn('size-10 shrink-0', className)}>
      <defs>
        <linearGradient id={id} x1="0.1" y1="0" x2="0.9" y2="1">
          <stop offset="0" style={{ stopColor: 'color-mix(in oklab, var(--primary) 62%, white)' }} />
          <stop offset="0.5" style={{ stopColor: 'var(--primary)' }} />
          <stop offset="1" style={{ stopColor: 'color-mix(in oklab, var(--primary) 68%, black)' }} />
        </linearGradient>
      </defs>
      <path
        d="M8.5 6.2c0-1.5 1.6-2.4 2.9-1.7l15.1 9.8c1.2.8 1.2 2.6 0 3.4L11.4 27.5c-1.3.8-2.9-.2-2.9-1.7Z"
        stroke={`url(#${id})`}
        strokeWidth="3.6"
        strokeLinejoin="round"
      />
      <path d="M13.2 12.4v7.2c0 .5.5.8.9.5l5.6-3.6c.4-.3.4-.8 0-1.1l-5.6-3.6c-.4-.3-.9 0-.9.6Z" fill={`url(#${id})`} />
    </svg>
  )
}

const sizes = {
  sm: { mark: 'size-8', text: 'text-xl' },
  md: { mark: 'size-10', text: 'text-2xl' },
  lg: { mark: 'size-14', text: 'text-4xl' },
}

/** Logo se slovní značkou – panel, hlavička na telefonu a přihlašovací stránka. */
export function Logo({ size = 'md', className }: { size?: 'sm' | 'md' | 'lg'; className?: string }) {
  const s = sizes[size]
  return (
    <span className={cn('flex items-center gap-2', className)}>
      <LogoMark className={s.mark} />
      <span className={cn('font-heading font-bold tracking-tight text-foreground', s.text)}>Sceniter</span>
    </span>
  )
}
