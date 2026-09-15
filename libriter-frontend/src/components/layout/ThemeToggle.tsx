import { MoonIcon, SunIcon } from 'lucide-react'
import { useTheme } from 'next-themes'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function ThemeToggle({ className }: { className?: string }) {
  const { resolvedTheme, setTheme } = useTheme()

  // Ikony přepínáme čistě v CSS podle třídy `dark` na <html>. Nepotřebujeme tak
  // stav "mounted" – před hydratací už je správná ikona vidět.
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label="Přepnout světlý/tmavý režim"
      className={cn('shrink-0', className)}
      onClick={() => setTheme(resolvedTheme === 'dark' ? 'light' : 'dark')}
    >
      <MoonIcon className="dark:hidden" />
      <SunIcon className="hidden dark:block" />
    </Button>
  )
}
