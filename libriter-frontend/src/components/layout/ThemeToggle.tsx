import { MonitorIcon, MoonIcon, PaletteIcon, SunIcon } from 'lucide-react'
import { useTheme } from 'next-themes'
import { useTranslation } from 'react-i18next'
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
import { COLOR_SCHEMES, colorSchemeLabel, useColorScheme } from '@/theme/colorScheme'

export function ThemeToggle({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const { colorScheme, setColorScheme, setThemeMode, isSaving } = useColorScheme()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('layout.appearance.settings')}
          title={isSaving ? t('layout.appearance.saving') : t('layout.appearance.settings')}
          aria-busy={isSaving}
          className={cn('shrink-0', className)}
        >
          <PaletteIcon className={isSaving ? 'animate-pulse' : undefined} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuLabel>{t('layout.appearance.mode')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={theme} onValueChange={setThemeMode} aria-label={t('layout.appearance.mode')}>
          <DropdownMenuRadioItem value="light" disabled={isSaving}><SunIcon /> {t('layout.appearance.light')}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark" disabled={isSaving}><MoonIcon /> {t('layout.appearance.dark')}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system" disabled={isSaving}><MonitorIcon /> {t('layout.appearance.system')}</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t('layout.appearance.colorScheme')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={colorScheme} onValueChange={setColorScheme} aria-label={t('layout.appearance.colorScheme')}>
          {COLOR_SCHEMES.map(({ value, color }) => (
            <DropdownMenuRadioItem key={value} value={value} disabled={isSaving}>
              <span aria-hidden="true" className="size-4 shrink-0 rounded-full border border-foreground/15" style={{ backgroundColor: color }} />
              {colorSchemeLabel(value)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
