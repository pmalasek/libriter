import { useEffect, useState } from 'react'
import { ActivityIndicator, View } from 'react-native'
import { Stack, useRouter, useSegments } from 'expo-router'
import { StatusBar } from 'expo-status-bar'
import { SafeAreaProvider } from 'react-native-safe-area-context'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { AuthProvider, useAuth } from '@/auth/AuthProvider'
import { PlayerProvider } from '@/player/PlayerProvider'
import { openDb } from '@/db/schema'
import { syncEngine } from '@/sync/syncEngine'
import { colors } from '@/theme'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Rozhraní čte z lokální databáze, dotazy na server obstarává
      // synchronizace – opakované pokusy by jen vybíjely baterii.
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
})

export default function RootLayout() {
  const [ready, setReady] = useState(false)

  // Databáze musí stát dřív, než si o ni řekne první obrazovka.
  useEffect(() => {
    void openDb().then(() => setReady(true))
  }, [])

  if (!ready) return <Splash />

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <PlayerProvider>
            <StatusBar style="light" />
            <AuthGate />
          </PlayerProvider>
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
  const segments = useSegments()
  const router = useRouter()

  useEffect(() => {
    if (loading) return
    const inAuthGroup = segments[0] === '(auth)'
    // Skupiny `(app)` a `(tabs)` se v adrese neprojeví, takže knihovna leží
    // rovnou na `/`.
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
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.background } }}>
      <Stack.Screen name="(auth)/login" />
      <Stack.Screen name="(app)" />
    </Stack>
  )
}

function Splash() {
  return (
    <View style={{ flex: 1, backgroundColor: colors.background, justifyContent: 'center' }}>
      <ActivityIndicator color={colors.accent} />
    </View>
  )
}
