// sceniter-shared jako první: při importu zaregistruje texty Sceniteru do
// sdílené instance i18next, takže první vykreslení už je má.
import { ApiError } from 'sceniter-shared'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import { AuthProvider } from 'core-web/src/auth/AuthContext'
import { configureSessionStorage } from 'core-web/src/auth/session'
import { BrandProvider } from 'core-web/src/brand'
import { Toaster } from 'core-web/src/components/ui/sonner'
import { LanguageProvider } from 'core-web/src/i18n/LanguageProvider'
import { ThemeProvider } from 'core-web/src/theme/ThemeProvider'
import { App } from './App'
import { sceniterBrand } from './brand'
import './index.css'

// Každý produkt má vlastní session (i ve vývoji, kdy oba běží na localhostu).
configureSessionStorage('sceniter.session')

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false
        return failureCount < 2
      },
    },
  },
})

const rootElement = document.getElementById('root')
if (!rootElement) throw new Error('Element #root nebyl nalezen')

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <BrandProvider brand={sceniterBrand}>
          <AuthProvider>
            <LanguageProvider>
              <ThemeProvider>
                <App />
                <Toaster />
              </ThemeProvider>
            </LanguageProvider>
          </AuthProvider>
        </BrandProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
