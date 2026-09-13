import { ThemeProvider as NextThemesProvider } from 'next-themes'

/**
 * next-themes je framework-agnostický – přepíná třídu `dark` na <html>,
 * respektuje systémové nastavení a sám injektuje skript proti bliknutí.
 * Používá ho i shadcn Toaster (sonner) přes useTheme().
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      storageKey="libriter.theme"
      disableTransitionOnChange
    >
      {children}
    </NextThemesProvider>
  )
}
