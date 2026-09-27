import { LayersIcon, LibraryIcon, UsersIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AuroraBackdrop } from './AuroraBackdrop'
import { Logo } from './Logo'
import { LanguageToggle } from './LanguageToggle'
import { ThemeToggle } from './ThemeToggle'

const FEATURES = [
  { icon: LibraryIcon, labelKey: 'auth.shell.features.library' },
  { icon: UsersIcon, labelKey: 'auth.shell.features.authors' },
  { icon: LayersIcon, labelKey: 'auth.shell.features.series' },
] as const

/**
 * Rámec přihlašovacích stránek: vlevo firemní panel (jen na velkém displeji),
 * vpravo formulář. Obsah formuláře si každá stránka řeší sama.
 */
export function AuthShell({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()

  return (
    <div className="relative grid min-h-svh lg:grid-cols-2">
      {/* Přehrávač tu není, takže pozadí nemá co rozmazávat – jen aurora. */}
      <AuroraBackdrop className="fixed -z-10" />

      <aside className="bg-brand-gradient relative m-4 hidden flex-col justify-between rounded-4xl p-12 text-primary-foreground shadow-glass-lg ring-1 ring-white/20 inset-shadow-[0_1px_0_0_oklch(1_0_0/0.35)] lg:flex">
        <div className="self-start rounded-2xl bg-white/90 p-3 shadow-glass dark:bg-background/90">
          <Logo size="lg" />
        </div>

        <div>
          <p className="font-heading max-w-md text-4xl font-bold tracking-tight text-balance">
            {t('auth.shell.headline')}
          </p>
          <ul className="mt-8 space-y-4">
            {FEATURES.map(({ icon: Icon, labelKey }) => (
              <li key={labelKey} className="flex items-center gap-3">
                <span className="flex size-9 items-center justify-center rounded-xl bg-white/15 ring-1 ring-white/25 inset-shadow-[0_1px_0_0_oklch(1_0_0/0.3)]">
                  <Icon className="size-4.5" />
                </span>
                <span className="text-sm">{t(labelKey)}</span>
              </li>
            ))}
          </ul>
        </div>

        <p className="text-sm opacity-70">{t('auth.shell.tagline')}</p>
      </aside>

      <main className="relative flex items-center justify-center p-6 pt-16">
        <div className="absolute right-4 top-4 flex gap-1">
          <LanguageToggle />
          <ThemeToggle />
        </div>
        <div className="w-full max-w-sm">
          <div className="mb-8 flex justify-center lg:hidden">
            <Logo size="lg" />
          </div>
          {children}
        </div>
      </main>
    </div>
  )
}
