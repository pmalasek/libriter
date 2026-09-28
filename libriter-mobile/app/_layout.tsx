import { useEffect, useState } from 'react'
import { LogBox, useColorScheme, View } from 'react-native'
import { Stack, useRouter, useSegments } from 'expo-router'
import { StatusBar } from 'expo-status-bar'
import { useFonts } from 'expo-font'
import * as SplashScreen from 'expo-splash-screen'
import { BricolageGrotesque_600SemiBold, BricolageGrotesque_700Bold } from '@expo-google-fonts/bricolage-grotesque'
import { Geist_400Regular, Geist_500Medium, Geist_600SemiBold } from '@expo-google-fonts/geist'
import { SafeAreaProvider } from 'react-native-safe-area-context'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { AuthProvider, useAuth } from '@/auth/AuthProvider'
import { SyncBadge } from '@/components/SyncBadge'
import { Toaster } from '@/components/Toast'
import { useSyncInvalidation } from '@/data/hooks'
import { PrefsProvider } from '@/data/listPrefs'
import { LanguageProvider } from '@/i18n/LanguageProvider'
import { ModeProvider } from '@/data/ModeProvider'
import { openDb } from '@/db/schema'
import { PlayerProvider } from '@/player/PlayerProvider'
import { registerBackgroundSync, unregisterBackgroundSync } from '@/sync/backgroundSync'
import { syncEngine } from '@/sync/syncEngine'
import { palette, ThemeProvider, useTheme } from '@/theme'

// react-native-track-player hlásí při startu čtyři varování o metodách
// časovače spánku, které v nativním modulu na iOSu nejsou. Aplikace je
// nepoužívá (časovač je vlastní, v JS) a banner jen překrývá obsah.
LogBox.ignoreLogs([/method signature for the JS method/])

// Nativní splash zůstane, dokud nestojí databáze, fonty a přihlášení –
// uživatel tak nevidí mezikrok s prázdnou obrazovkou ani spinner.
void SplashScreen.preventAutoHideAsync()
SplashScreen.setOptions({ fade: true, duration: 250 })

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // V online režimu se čte ze serveru jako na webu; opakované pokusy by
      // při výpadku jen vybíjely baterii – fallback na lokální data řeší
      // withSource.
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
})

export default function RootLayout() {
  const [dbReady, setDbReady] = useState(false)
  const [fontsLoaded, fontsError] = useFonts({
    BricolageGrotesque_600SemiBold,
    BricolageGrotesque_700Bold,
    Geist_400Regular,
    Geist_500Medium,
    Geist_600SemiBold,
  })

  // Chybějící font není důvod aplikaci nepustit – vezme se systémový.
  const fontsReady = fontsLoaded || fontsError != null

  // Databáze musí stát dřív, než si o ni řekne první obrazovka.
  useEffect(() => {
    openDb()
      .then(() => setDbReady(true))
      .catch((error: unknown) => {
        // Bez databáze aplikace nepoběží, ale splash nesmí viset navždy.
        console.error('openDb failed', error)
        void SplashScreen.hideAsync()
      })
  }, [])

  if (!dbReady || !fontsReady) return <Splash />

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <LanguageProvider>
            <ThemeProvider>
              <ModeProvider>
                <PrefsProvider>
                  <PlayerProvider>
                    <AuthGate />
                    <SyncBadge />
                    <Toaster />
                  </PlayerProvider>
                </PrefsProvider>
              </ModeProvider>
            </ThemeProvider>
          </LanguageProvider>
        </AuthProvider>
      </QueryClientProvider>
    </SafeAreaProvider>
  )
}

/**
 * Drží uživatele na správné straně přihlášení a startuje synchronizaci.
 * Je to samostatná komponenta, protože se potřebuje dostat k useAuth –
 * uvnitř AuthProvider, ne nad ním.
 */
function AuthGate() {
  const { session, loading, signOut } = useAuth()
  const { colors, resolved } = useTheme()
  const segments = useSegments()
  const router = useRouter()
  useSyncInvalidation()

  useEffect(() => {
    if (loading) return
    const inAuthGroup = segments[0] === '(auth)'
    // Skupiny `(app)` a `(tabs)` se v adrese neprojeví, takže domů leží na `/`.
    if (!session && !inAuthGroup) router.replace('/login')
    else if (session && inAuthGroup) router.replace('/')
  }, [loading, router, segments, session])

  useEffect(() => {
    if (!session) return
    syncEngine.start({ onUnauthorized: () => void signOut() })
    return () => syncEngine.stop()
  }, [session, signOut])

  // Úkol na pozadí patří k přihlášení, ne k životu obrazovky: zrušit ho jen
  // po odhlášení, ne při každém odmountování.
  const signedIn = Boolean(session)
  useEffect(() => {
    if (loading) return
    const change = signedIn ? registerBackgroundSync() : unregisterBackgroundSync()
    change.catch(() => undefined)
  }, [loading, signedIn])

  useEffect(() => {
    if (!loading) void SplashScreen.hideAsync()
  }, [loading])

  if (loading) return <Splash />

  return (
    <>
      <StatusBar style={resolved === 'dark' ? 'light' : 'dark'} />
      <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.background } }}>
        <Stack.Screen name="(auth)/login" />
        <Stack.Screen name="(app)" />
      </Stack>
    </>
  )
}

/**
 * Podklad pod nativním splashem, než se aplikace rozběhne. Sám nic neukazuje –
 * jen drží stejnou barvu jako splash z app.json, aby při jeho zmizení nic
 * neblikalo. Téma ještě není načtené, proto se řídí režimem systému.
 */
function Splash() {
  const mode = useColorScheme() === 'dark' ? 'dark' : 'light'
  return <View style={{ flex: 1, backgroundColor: palette('teal', mode).background }} />
}
