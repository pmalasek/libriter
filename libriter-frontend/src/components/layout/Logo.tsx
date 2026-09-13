import { cn } from '@/lib/utils'

/** Logo má světlou i tmavou variantu; přepínáme je podle třídy `dark` na <html>. */
export function Logo({ className }: { className?: string }) {
  return (
    <>
      <img
        src="/libriter-logo-light.png"
        alt="Libriter"
        className={cn('h-7 w-auto dark:hidden', className)}
      />
      <img
        src="/libriter-logo-dark.png"
        alt="Libriter"
        className={cn('hidden h-7 w-auto dark:block', className)}
      />
    </>
  )
}
