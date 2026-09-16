import { MonitorIcon, MoonIcon, PaletteIcon, SunIcon } from 'lucide-react'
import { useTheme } from 'next-themes'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import { COLOR_SCHEMES, useColorScheme } from '@/theme/colorScheme'

export function ThemeToggle({ className }: { className?: string }) {
  const { theme } = useTheme()
  const { colorScheme, setColorScheme, setThemeMode, isSaving } = useColorScheme()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label="Nastavení vzhledu"
          title={isSaving ? 'Ukládám vzhled…' : 'Nastavení vzhledu'}
          aria-busy={isSaving}
          className={cn('shrink-0', className)}
        >
          <PaletteIcon className={isSaving ? 'animate-pulse' : undefined} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuLabel>Režim zobrazení</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={theme} onValueChange={setThemeMode} aria-label="Režim zobrazení">
          <DropdownMenuRadioItem value="light" disabled={isSaving}><SunIcon /> Světlý</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark" disabled={isSaving}><MoonIcon /> Tmavý</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system" disabled={isSaving}><MonitorIcon /> Podle systému</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>Barevné schéma</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={colorScheme} onValueChange={setColorScheme} aria-label="Barevné schéma">
          {COLOR_SCHEMES.map(({ value, label, color }) => (
            <DropdownMenuRadioItem key={value} value={value} disabled={isSaving}>
              <span aria-hidden="true" className="size-4 shrink-0 rounded-full border border-foreground/15" style={{ backgroundColor: color }} />
              {label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
