import { useEffect, useState } from 'react'
import { ActivityIndicator, LogBox, View } from 'react-native'
import { Stack, useRouter, useSegments } from 'expo-router'
import { StatusBar } from 'expo-status-bar'
import { useFonts } from 'expo-font'
import { BricolageGrotesque_600SemiBold, BricolageGrotesque_700Bold } from '@expo-google-fonts/bricolage-grotesque'
import { Geist_400Regular, Geist_500Medium, Geist_600SemiBold } from '@expo-google-fonts/geist'
import { SafeAreaProvider } from 'react-native-safe-area-context'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { AuthProvider, useAuth } from '@/auth/AuthProvider'
import { Toaster } from '@/components/Toast'
import { useSyncInvalidation } from '@/data/hooks'
import { PrefsProvider } from '@/data/listPrefs'
import { ModeProvider } from '@/data/ModeProvider'
import { openDb } from '@/db/schema'
import { PlayerProvider } from '@/player/PlayerProvider'
import { syncEngine } from '@/sync/syncEngine'
import { ThemeProvider, useTheme } from '@/theme'

// react-native-track-player hlásí při startu čtyři varování o metodách
// časovače spánku, které v nativním modulu na iOSu nejsou. Aplikace je
// nepoužívá (časovač je vlastní, v JS) a banner jen překrývá obsah.
LogBox.ignoreLogs([/method signature for the JS method/])

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
  const [fontsReady] = useFonts({
    BricolageGrotesque_600SemiBold,
    BricolageGrotesque_700Bold,
    Geist_400Regular,
    Geist_500Medium,
    Geist_600SemiBold,
  })

  // Databáze musí stát dřív, než si o ni řekne první obrazovka.
  useEffect(() => {
    void openDb().then(() => setDbReady(true))
  }, [])

  if (!dbReady || !fontsReady) return <Splash />

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <ThemeProvider>
            <ModeProvider>
              <PrefsProvider>
                <PlayerProvider>
                  <AuthGate />
                  <Toaster />
                </PlayerProvider>
              </PrefsProvider>
            </ModeProvider>
          </ThemeProvider>
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

function Splash() {
  return (
    <View style={{ flex: 1, backgroundColor: '#0F1417', justifyContent: 'center' }}>
      <ActivityIndicator color="#3FB8AF" />
    </View>
  )
}
