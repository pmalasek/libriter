import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import { ApiError } from '@/api/client'
import { AuthProvider } from '@/auth/AuthContext'
import { Toaster } from '@/components/ui/sonner'
import { PlayerProvider } from '@/player/PlayerProvider'
import { ThemeProvider } from '@/theme/ThemeProvider'
import { App } from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      // 4xx jsou chyby klienta – opakování nemá smysl.
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
        <AuthProvider>
          <ThemeProvider>
            {/* Přehrávač stojí nad routerem, aby poslech přežil změnu stránky. */}
            <PlayerProvider>
              <App />
              <Toaster />
            </PlayerProvider>
          </ThemeProvider>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
