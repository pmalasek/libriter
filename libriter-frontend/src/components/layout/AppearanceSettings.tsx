import { MonitorIcon, MoonIcon, SunIcon } from 'lucide-react'
import { useTheme } from 'next-themes'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { COLOR_SCHEMES, colorSchemeLabel, useColorScheme } from '@/theme/colorScheme'

const MODES = [
  { value: 'light', icon: SunIcon, labelKey: 'layout.appearance.light' },
  { value: 'dark', icon: MoonIcon, labelKey: 'layout.appearance.dark' },
  { value: 'system', icon: MonitorIcon, labelKey: 'layout.appearance.system' },
] as const

const PILL = 'inline-flex flex-wrap gap-1 rounded-full bg-foreground/6 p-1 ring-1 ring-inset ring-foreground/5'
const PILL_ITEM = 'rounded-full hover:bg-transparent'
const PILL_ACTIVE = 'glass-strong inset-shadow-glass text-primary shadow-sm'

/**
 * Vzhled rozložený přímo na stránce (profil) – na rozdíl od ThemeToggle je
 * aktuální volba vidět bez otevírání menu.
 */
export function AppearanceSettings() {
  const { t } = useTranslation()
  const { theme } = useTheme()
  const { colorScheme, setColorScheme, setThemeMode, isSaving } = useColorScheme()

  return (
    <div className="space-y-4" aria-busy={isSaving}>
      <div className="space-y-2">
        <p id="appearance-mode" className="text-sm font-medium">{t('layout.appearance.mode')}</p>
        <div role="radiogroup" aria-labelledby="appearance-mode" className={PILL}>
          {MODES.map(({ value, icon: Icon, labelKey }) => {
            const active = value === theme
            return (
              <Button
                key={value}
                type="button"
                variant="ghost"
                size="sm"
                role="radio"
                aria-checked={active}
                disabled={isSaving}
                onClick={() => setThemeMode(value)}
                className={cn(PILL_ITEM, 'px-3', active && PILL_ACTIVE)}
              >
                <Icon />
                {t(labelKey)}
              </Button>
            )
          })}
        </div>
      </div>

      <div className="space-y-2">
        <p id="appearance-scheme" className="text-sm font-medium">{t('layout.appearance.colorScheme')}</p>
        <div role="radiogroup" aria-labelledby="appearance-scheme" className={PILL}>
          {COLOR_SCHEMES.map(({ value, color }) => {
            const active = value === colorScheme
            return (
              <Button
                key={value}
                type="button"
                variant="ghost"
                size="sm"
                role="radio"
                aria-checked={active}
                disabled={isSaving}
                onClick={() => setColorScheme(value)}
                className={cn(PILL_ITEM, 'px-3', active && PILL_ACTIVE)}
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    'size-3.5 shrink-0 rounded-full border border-foreground/15',
                    active && 'ring-2 ring-primary/40 ring-offset-1 ring-offset-background',
                  )}
                  style={{ backgroundColor: color }}
                />
                {colorSchemeLabel(value)}
              </Button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
