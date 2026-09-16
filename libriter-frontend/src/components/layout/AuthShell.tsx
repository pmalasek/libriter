import { LayersIcon, LibraryIcon, UsersIcon } from 'lucide-react'
import { Logo } from './Logo'
import { ThemeToggle } from './ThemeToggle'

const FEATURES = [
  { icon: LibraryIcon, label: 'Celá sbírka audioknih na jednom místě' },
  { icon: UsersIcon, label: 'Autoři s fotkou a životopisem' },
  { icon: LayersIcon, label: 'Série srovnané podle dílů' },
]

/**
 * Rámec přihlašovacích stránek: vlevo firemní panel (jen na velkém displeji),
 * vpravo formulář. Obsah formuláře si každá stránka řeší sama.
 */
export function AuthShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <aside className="bg-brand-gradient relative hidden flex-col justify-between p-12 text-primary-foreground lg:flex">
        <div className="self-start rounded-2xl bg-background p-3">
          <Logo size="lg" />
        </div>

        <div>
          <p className="font-heading max-w-md text-4xl font-bold tracking-tight text-balance">
            Vaše audioknihy. Na jednom místě.
          </p>
          <ul className="mt-8 space-y-4">
            {FEATURES.map(({ icon: Icon, label }) => (
              <li key={label} className="flex items-center gap-3">
                <span className="flex size-9 items-center justify-center rounded-xl bg-white/15">
                  <Icon className="size-4.5" />
                </span>
                <span className="text-sm">{label}</span>
              </li>
            ))}
          </ul>
        </div>

        <p className="text-sm opacity-70">Osobní správce a přehrávač audioknih.</p>
      </aside>

      <main className="relative flex items-center justify-center p-6 pt-16">
        <ThemeToggle className="absolute right-4 top-4" />
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
