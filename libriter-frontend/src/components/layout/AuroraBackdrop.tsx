import { cn } from '@/lib/utils'

/**
 * Klidná záře v odstínu zvoleného barevného schématu. Leží pod sklem všude
 * tam, kde není co rozmazávat – tedy když nic nehraje a na přihlašovacích
 * stránkách, kde přehrávač vůbec není.
 *
 * Skvrny jsou radiální gradienty, ne rozostřené tvary: `filter: blur` by se
 * na animovaném prvku přes celou obrazovku počítal v každém snímku znovu.
 */
export function AuroraBackdrop({ className }: { className?: string }) {
  return (
    <div
      aria-hidden
      className={cn('pointer-events-none absolute inset-0 overflow-hidden', className)}
    >
      <div className="animate-aurora absolute -top-1/4 -left-1/4 size-[70vmax] rounded-full bg-[radial-gradient(closest-side,var(--aurora-1),transparent)] will-change-transform" />
      <div className="animate-aurora absolute top-1/3 -right-1/4 size-[60vmax] rounded-full bg-[radial-gradient(closest-side,var(--aurora-2),transparent)] will-change-transform [animation-delay:-16s] [animation-duration:64s]" />
      <div className="animate-aurora absolute -bottom-1/3 left-1/3 size-[65vmax] rounded-full bg-[radial-gradient(closest-side,var(--aurora-3),transparent)] will-change-transform [animation-delay:-32s] [animation-duration:56s]" />
    </div>
  )
}
